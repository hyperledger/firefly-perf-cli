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

var perfRunner perf.PerfRunner

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network",
	Long: GetFireflyAsciiArt() + `
Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network"
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		config, err := loadPerfConfig(configFilePath)
		if err != nil {
			return err
		}

		if instanceName != "" && instanceIndex != -1 {
			log.Warn("both the \"instance-name\" and \"instance-index\" flags were provided, using \"instance-name\"")
		}

		var instance *conf.InstanceConfig
		if instanceName != "" {
			for _, i := range config.Instances {
				if i.Name == instanceName {
					instance = &i
				}
			}
			if instance == nil {
				return errors.Errorf("did not find instance named \"%s\" within the provided config", instanceName)
			}
		} else if instanceIndex != -1 {
			if instanceIndex >= len(config.Instances) {
				return errors.Errorf("provided instance index \"%d\" is outside of the range of instances within the provided config", instanceIndex)
			}
			instance = &config.Instances[instanceIndex]
		} else {
			return errors.Errorf("please set either the \"instance-name\" or \"instance-index\" ")
		}

		if perfRunner == nil {
			runnerConfig, err := loadRunnerConfigFromInstance(instance, config)
			if err != nil {
				return err
			}

			if runnerConfig.StackJSONPath == "" {
				runnerConfig.NodeURLs = []string{"http://localhost:5000"}
			} else {
				stack, err := readStackJSON(runTestsConfig.StackJSONPath)
				if err != nil {
					return err
				}
				runnerConfig.NodeURLs = make([]string, len(stack.Members))
				for i, member := range stack.Members {
					// TODO dont hardcode to localhost
					runnerConfig.NodeURLs[i] = fmt.Sprintf("http://localhost:%v", member.ExposedFireflyPort)
				}
			}

			perfRunner = perf.New(runnerConfig)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := perfRunner.Init()
		if err != nil {
			return err
		}
		return perfRunner.Start()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	runCmd.Flags().StringVarP(&configFilePath, "config", "c", "", "Path to performance config that describes the network and test instances")
	runCmd.Flags().StringVarP(&instanceName, "instance-name", "i", "", "Instance within performance config to run against the network")
	runCmd.Flags().IntVarP(&instanceIndex, "instance-idx", "n", -1, "Index of the instance within performance config to run against the network")

	runCmd.MarkFlagRequired("config")
}

func loadPerfConfig(filename string) (*conf.PerformanceTestConfig, error) {
	if d, err := ioutil.ReadFile(filename); err != nil {
		return nil, err
	} else {
		var config *conf.PerformanceTestConfig
		var err error
		if path.Ext(filename) == "yaml" {
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

func loadRunnerConfigFromInstance(instance *conf.InstanceConfig, perfConfig *conf.PerformanceTestConfig) (*conf.PerfRunnerConfig, error) {
	var runnerConfig *conf.PerfRunnerConfig

	runnerConfig.Tests = []fftypes.FFEnum{instance.Test}

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

	err := validateConfig(*runnerConfig)
	if err != nil {
		return nil, err
	}

	return runnerConfig, nil
}
