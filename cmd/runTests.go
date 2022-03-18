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
	"time"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
	"github.com/hyperledger/firefly-perf-cli/internal/types"
	"github.com/hyperledger/firefly/pkg/fftypes"

	"github.com/spf13/cobra"
)

var runTestsPerfRunner perf.PerfRunner
var runTestsConfig conf.PerfRunnerConfig

// runTestsCmd represents the run-tests command
var runTestsCmd = &cobra.Command{
	Use:   "run-tests",
	Short: "Executes the provided list of tests against a FireFly node to generate synthetic load",
	Long: GetFireflyAsciiArt() + `
Executes the provided list of tests against a FireFly node to generate synthetic load.
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		runTestsConfig.WebSocket = conf.FireFlyWsConf{
			WSPath:                 "/ws",
			ReadBufferSize:         16000,
			WriteBufferSize:        16000,
			InitialDelay:           250 * time.Millisecond,
			MaximumDelay:           30 * time.Second,
			InitialConnectAttempts: 5,
			HeartbeatInterval:      5 * time.Second,
		}

		if runTestsPerfRunner == nil {
			err := validateCommands(args)
			if err != nil {
				return err
			}
			err = validateConfig(runTestsConfig)
			if err != nil {
				return err
			}

			if runTestsConfig.StackJSONPath == "" {
				runTestsConfig.NodeURLs = []string{"http://localhost:5000"}
			} else {
				stack, err := readStackJSON(runTestsConfig.StackJSONPath)
				if err != nil {
					return err
				}
				runTestsConfig.NodeURLs = make([]string, len(stack.Members))
				for i, member := range stack.Members {
					runTestsConfig.NodeURLs[i] = fmt.Sprintf("http://localhost:%v", member.ExposedFireflyPort)
				}
			}

			runTestsPerfRunner = perf.New(&runTestsConfig)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTests()
	},
}

func runTests() error {
	err := runTestsPerfRunner.Init()
	if err != nil {
		return err
	}
	return runTestsPerfRunner.Start()
}

func init() {
	rootCmd.AddCommand(runTestsCmd)

	runTestsCmd.Flags().DurationVarP(&runTestsConfig.Length, "length", "l", 60*time.Second, "Length of entire performance test")
	runTestsCmd.Flags().BoolVar(&runTestsConfig.MessageOptions.LongMessage, "longMessage", false, "Include long string in message")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.Recipient, "recipient", "r", "", "Recipient for FireFly messages")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.RecipientAddress, "recipientAddress", "x", "", "Recipient address for FireFly transfers")
	runTestsCmd.Flags().StringVar(&runTestsConfig.TokenOptions.TokenType, "tokenType", fftypes.TokenTypeFungible.String(), fmt.Sprintf("[%s %s]", fftypes.TokenTypeFungible.String(), fftypes.TokenTypeNonFungible.String()))
	runTestsCmd.Flags().IntVarP(&runTestsConfig.Workers, "workers", "w", 1, "Number of workers at a time")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.ContractOptions.Address, "address", "a", "", "Address of custom contract")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.ContractOptions.Channel, "channel", "", "", "Fabric channel for custom contract")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.ContractOptions.Chaincode, "chaincode", "", "", "Chaincode name for custom contract")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.StackJSONPath, "stackJSON", "s", "", "Path to stack.json file that describes the network to test")
	runTestsCmd.Flags().StringVarP(&runTestsConfig.DelinquentAction, "delinquent", "", "exit", "Action to take when delinquent messages are detected. Valid options: [exit log]")
}

func validateCommands(cmds []string) error {
	cmdArr := []fftypes.FFEnum{}
	cmdSet := make(map[fftypes.FFEnum]bool, 0)
	for _, cmd := range cmds {
		if val, ok := conf.ValidPerfTests[cmd]; ok {
			cmdSet[val] = true
		} else {
			return fmt.Errorf("test commands not valid. Choose from %v", conf.ValidPerfTestsString())
		}
	}
	for cmd := range cmdSet {
		cmdArr = append(cmdArr, cmd)
	}

	if len(cmdArr) == 0 {
		return fmt.Errorf("must specify at least one command. Choose from %v", conf.ValidPerfTestsString())
	}
	runTestsConfig.Tests = cmdArr

	return nil
}

func validateConfig(cfg conf.PerfRunnerConfig) error {
	if cfg.TokenOptions.TokenType != fftypes.TokenTypeFungible.String() && cfg.TokenOptions.TokenType != fftypes.TokenTypeNonFungible.String() {
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
