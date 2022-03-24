// Copyright © 2022 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
	"github.com/hyperledger/firefly-perf-cli/internal/server"
	"github.com/hyperledger/firefly-perf-cli/internal/types"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path"
)

var configFilePath string
var instanceName string
var instanceIndex int
var daemonOverride bool
var deliquentAction string

var httpServer *server.HttpServer
var perfRunner perf.PerfRunner

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network",
	Long: GetFireflyAsciiArt() + `
Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		config, err := loadConfig(configFilePath)
		if err != nil {
			return err
		}

		if !config.Daemon {
			config.Daemon = daemonOverride
		}

		if instanceName != "" && instanceIndex != -1 {
			log.Warn("both the \"instance-name\" and \"instance-index\" flags were provided, using \"instance-name\"")
		}

		instance, err := selectInstance(config)
		if err != nil {
			return err
		}

		runnerConfig, err := generateRunnerConfigFromInstance(instance, config)
		if err != nil {
			return err
		}

		perfRunner = perf.New(runnerConfig)
		httpServer = server.NewHttpServer()

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := perfRunner.Init()
		if err != nil {
			return err
		}

		go httpServer.Run()
		return perfRunner.Start()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&configFilePath, "config", "c", "", "Path to performance config that describes the network and test instances")
	runCmd.Flags().StringVarP(&instanceName, "instance-name", "n", "", "Instance within performance config to run against the network")
	runCmd.Flags().IntVarP(&instanceIndex, "instance-idx", "i", -1, "Index of the instance within performance config to run against the network")
	runCmd.Flags().BoolVarP(&daemonOverride, "daemon", "d", false, "Run in long-lived, daemon mode. Any provided test length is ignored.")
	runCmd.Flags().StringVarP(&deliquentAction, "delinquent", "", "exit", "Action to take when delinquent messages are detected. Valid options: [exit log]")

	runCmd.MarkFlagRequired("config")
}

func loadConfig(filename string) (*conf.PerformanceTestConfig, error) {
	if d, err := ioutil.ReadFile(filename); err != nil {
		return nil, err
	} else {
		var config *conf.PerformanceTestConfig
		var err error
		if path.Ext(filename) == ".yaml" {
			err = yaml.Unmarshal(d, &config)
		} else {
			err = json.Unmarshal(d, &config)
		}

		if err != nil {
			return nil, err
		}
		return config, nil
	}
}

func selectInstance(config *conf.PerformanceTestConfig) (*conf.InstanceConfig, error) {
	if instanceName != "" {
		for _, i := range config.Instances {
			if i.Name == instanceName {
				return &i, nil
			}
		}
		return nil, errors.Errorf("did not find instance named \"%s\" within the provided config", instanceName)
	} else if instanceIndex != -1 {
		if instanceIndex >= len(config.Instances) || instanceIndex < 0 {
			return nil, errors.Errorf("provided instance index \"%d\" is outside of the range of instances within the provided config", instanceIndex)
		}
		return &config.Instances[instanceIndex], nil
	}

	return nil, errors.Errorf("please set either the \"instance-name\" or \"instance-index\" ")
}

func generateRunnerConfigFromInstance(instance *conf.InstanceConfig, perfConfig *conf.PerformanceTestConfig) (*conf.PerfRunnerConfig, error) {
	runnerConfig := &conf.PerfRunnerConfig{
		Tests: []fftypes.FFEnum{instance.Test},
	}

	switch instance.Test {
	case conf.PerfTestBroadcast:
	case conf.PerfTestPrivateMsg:
		runnerConfig.MessageOptions = instance.MessageOptions
	case conf.PerfTestTokenMint:
		runnerConfig.TokenOptions = instance.TokenOptions
	case conf.PerfTestCustomFabricContract:
	case conf.PerfTestCustomEthereumContract:
		runnerConfig.ContractOptions = instance.ContractOptions
	}

	runnerConfig.Workers = instance.Workers
	runnerConfig.Length = instance.Length
	runnerConfig.Recipient = instance.Recipient
	runnerConfig.RecipientAddress = instance.RecipientAddress
	runnerConfig.StackJSONPath = perfConfig.StackJSONPath
	runnerConfig.WebSocket = perfConfig.WSConfig
	runnerConfig.Daemon = perfConfig.Daemon
	runnerConfig.DelinquentAction = deliquentAction

	err := validateConfig(*runnerConfig)
	if err != nil {
		return nil, err
	}

	if runnerConfig.StackJSONPath == "" {
		runnerConfig.NodeURLs = []string{"http://localhost:5000"}
	} else {
		stack, err := readStackJSON(runnerConfig.StackJSONPath)
		if err != nil {
			return nil, err
		}
		runnerConfig.NodeURLs = make([]string, len(stack.Members))
		for i, member := range stack.Members {
			if member.FireflyHostname == "" {
				member.FireflyHostname = "localhost"
			}
			scheme := "http"
			if member.UseHTTPS {
				scheme = "https"
			}

			// TODO support username / passwords ? ideally isn't embedded into the URL itself but set as a header
			runnerConfig.NodeURLs[i] = fmt.Sprintf("%s://%s:%v", scheme, member.FireflyHostname, member.ExposedFireflyPort)
		}
	}

	return runnerConfig, nil
}

func validateConfig(cfg conf.PerfRunnerConfig) error {
	if cfg.TokenOptions.TokenType != "" && cfg.TokenOptions.TokenType != fftypes.TokenTypeFungible.String() && cfg.TokenOptions.TokenType != fftypes.TokenTypeNonFungible.String() {
		return fmt.Errorf("invalid token type. Choose from [%s %s]", fftypes.TokenTypeFungible.String(), fftypes.TokenTypeNonFungible.String())
	}
	return nil
}

func readStackJSON(filename string) (*types.Stack, error) {
	if d, err := ioutil.ReadFile(filename); err != nil {
		return nil, err
	} else {
		var stack *types.Stack
		if err := json.Unmarshal(d, &stack); err != nil {
			return nil, err
		}
		return stack, nil
	}
}
