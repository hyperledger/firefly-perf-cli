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
	"io/ioutil"
	"net/url"
	"path"
	"time"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
	"github.com/hyperledger/firefly-perf-cli/internal/server"
	"github.com/hyperledger/firefly-perf-cli/internal/types"
	"github.com/hyperledger/firefly-perf-cli/internal/util"
	"github.com/hyperledger/firefly/pkg/core"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

		instanceConfig, err := selectInstance(config)
		if err != nil {
			return err
		}

		runnerConfig, err := generateRunnerConfigFromInstance(instanceConfig, config)
		if err != nil {
			return err
		}

		configYaml, err := yaml.Marshal(instanceConfig)
		if err != nil {
			return err
		}

		perfRunner = perf.New(runnerConfig, util.NewReportForTestInstance(string(configYaml), instanceName))
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
		if path.Ext(filename) == ".yaml" || path.Ext(filename) == ".yml" {
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

func generateRunnerConfigFromInstance(instance *conf.InstanceConfig, perfConfig *conf.PerformanceTestConfig) (*conf.RunnerConfig, error) {
	runnerConfig := &conf.RunnerConfig{
		Tests: instance.Tests,
	}

	runnerConfig.WebSocket = perfConfig.WSConfig
	runnerConfig.InvokeOptions = instance.InvokeOptions

	if len(perfConfig.Nodes) > 0 {
		// Use node configuration defined in the ffperf config file

		// Use manual endpoint configuration instead of getting it from a FireFly stack
		log.Infof("Running test against manual endpoint \"%s\"\n", perfConfig.Nodes[instance.ManualNodeIndex].APIEndpoint)

		runnerConfig.NodeURLs = make([]string, 0)
		runnerConfig.NodeURLs = append(runnerConfig.NodeURLs, perfConfig.Nodes[instance.ManualNodeIndex].APIEndpoint)
		runnerConfig.SenderURL = runnerConfig.NodeURLs[0]
		runnerConfig.RecipientAddress = instance.TokenOptions.RecipientAddress
		runnerConfig.SigningKey = instance.SigningKey
		runnerConfig.PerWorkerSigningKeyPrefix = instance.PerWorkerSigningKeyPrefix

		if perfConfig.Nodes[instance.ManualNodeIndex].AuthUsername != "" {
			runnerConfig.WebSocket.AuthUsername = perfConfig.Nodes[instance.ManualNodeIndex].AuthUsername
		}

		if perfConfig.Nodes[instance.ManualNodeIndex].AuthPassword != "" {
			runnerConfig.WebSocket.AuthPassword = perfConfig.Nodes[instance.ManualNodeIndex].AuthPassword
		}
	} else {
		// Read endpoint information from the stack JSON
		log.Infof("Running test against stack \"%s\"\n", perfConfig.StackJSONPath)

		runnerConfig.StackJSONPath = perfConfig.StackJSONPath
		stack, stackErr := readStackJSON(runnerConfig.StackJSONPath)
		if stackErr != nil {
			fmt.Println("Err")
			fmt.Println(stackErr)
			return nil, stackErr
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

			runnerConfig.NodeURLs[i] = fmt.Sprintf("%s://%s:%v", scheme, member.FireflyHostname, member.ExposedFireflyPort)
		}

		runnerConfig.SenderURL = runnerConfig.NodeURLs[instance.Sender]
		if instance.Recipient != nil {
			runnerConfig.RecipientOrg = fmt.Sprintf("did:firefly:org/%s", stack.Members[*instance.Recipient].OrgName)
			runnerConfig.RecipientAddress = stack.Members[*instance.Recipient].Address
		}
	}

	runnerConfig.TokenOptions = instance.TokenOptions
	runnerConfig.MessageOptions = instance.MessageOptions
	runnerConfig.TokenOptions = instance.TokenOptions
	runnerConfig.ContractOptions = instance.ContractOptions

	// Common configuration regardless of running with manually defined nodes or a local stack
	runnerConfig.LogLevel = perfConfig.LogLevel
	runnerConfig.SkipMintConfirmations = instance.SkipMintConfirmations
	runnerConfig.NoWaitSubmission = instance.NoWaitSubmission
	runnerConfig.Length = instance.Length
	runnerConfig.Daemon = perfConfig.Daemon
	runnerConfig.LogEvents = perfConfig.LogEvents
	runnerConfig.DelinquentAction = deliquentAction
	runnerConfig.FFNamespace = instance.FFNamespace
	runnerConfig.APIPrefix = instance.APIPrefix
	if instance.FFNamespaceBasePath != "" {
		basePath, err := url.JoinPath(instance.APIPrefix, instance.FFNamespaceBasePath)
		if err != nil {
			return nil, err
		}
		runnerConfig.FFNamespacePath = basePath
	}
	runnerConfig.MaxTimePerAction = instance.MaxTimePerAction
	runnerConfig.MaxActions = instance.MaxActions
	runnerConfig.RampLength = instance.RampLength

	// If delinquent action has been set on the test run instance this overrides the command line
	if instance.DelinquentAction != "" {
		runnerConfig.DelinquentAction = instance.DelinquentAction
	}

	setDefaults(runnerConfig)

	err := validateConfig(runnerConfig, instance, perfConfig)
	if err != nil {
		return nil, err
	}

	return runnerConfig, nil
}

func setDefaults(runnerConfig *conf.RunnerConfig) {
	if runnerConfig.FFNamespace == "" {
		runnerConfig.FFNamespace = "default"
	}

	if runnerConfig.FFNamespacePath == "" {
		basePath, err := url.JoinPath(runnerConfig.APIPrefix, "api/v1/namespaces", runnerConfig.FFNamespace)
		if err != nil {
			log.Error(err.Error())
		}
		runnerConfig.FFNamespacePath = basePath
	}

	if runnerConfig.TokenOptions.TokenPoolConnectorName == "" {
		runnerConfig.TokenOptions.TokenPoolConnectorName = "erc20_erc721"
	}
	if runnerConfig.MaxTimePerAction.Seconds() == 0 {
		runnerConfig.MaxTimePerAction = 60 * time.Second
	}

	for i, _ := range runnerConfig.Tests {
		if runnerConfig.Tests[i].ActionsPerLoop <= 0 {
			runnerConfig.Tests[i].ActionsPerLoop = 1
		}
	}
}

func validateConfig(cfg *conf.RunnerConfig, instance *conf.InstanceConfig, globalConfig *conf.PerformanceTestConfig) error {
	if cfg.TokenOptions.TokenType != "" && cfg.TokenOptions.TokenType != core.TokenTypeFungible.String() && cfg.TokenOptions.TokenType != core.TokenTypeNonFungible.String() {
		return fmt.Errorf("invalid token type. Choose from [%s %s]", core.TokenTypeFungible.String(), core.TokenTypeNonFungible.String())
	}
	if cfg.SigningKey != "" && cfg.PerWorkerSigningKeyPrefix != "" {
		return fmt.Errorf("must only specify one of 'signingKey' and 'perWorkerSigningKeyPrefix'")
	}
	if len(globalConfig.Nodes) > 0 && globalConfig.StackJSONPath != "" {
		return fmt.Errorf("FireFly performance CLI cannot be configured with manual nodes and a local FireFly stack")
	}
	if len(globalConfig.Nodes) > 0 && ((instance.ManualNodeIndex + 1) > len(globalConfig.Nodes)) {
		return fmt.Errorf("NodeIndex %d not valid - only %d nodes have been configured", instance.ManualNodeIndex, len(globalConfig.Nodes))
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
