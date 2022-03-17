/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
	"github.com/hyperledger/firefly-perf-cli/internal/types"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"io/ioutil"
	"time"

	"github.com/spf13/cobra"
)

var perfRunner perf.PerfRunner
var runTestsConfig conf.PerfConfig

// runTestsCmd represents the run-tests command
var runTestsCmd = &cobra.Command{
	Use:   "run-tests",
	Short: "Runs the provided list of tests against a FireFly node to generate synthetic load",
	Long: GetFireflyAsciiArt() + `
run-tests runs the provided list of tests against a FireFly node to generate synthetic load
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		runTestsConfig.WebSocket = conf.FireFlyWsConf{
			WSPath:                 "/ws",
			ReadBufferSize:         16000,
			WriteBufferSize:        16000,
			InitialDelay:           250000000,
			MaximumDelay:           30000000000,
			InitialConnectAttempts: 5,
		}

		if perfRunner == nil {
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

			perfRunner = perf.New(&runTestsConfig)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTests()
	},
}

func runTests() error {
	err := perfRunner.Init()
	if err != nil {
		return err
	}
	return perfRunner.Start()
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
}

func validateCommands(cmds []string) error {
	cmdArr := []fftypes.FFEnum{}
	cmdSet := make(map[fftypes.FFEnum]bool, 0)
	for _, cmd := range cmds {
		if val, ok := conf.ValidPerfCommands[cmd]; ok {
			cmdSet[val] = true
		} else {
			return fmt.Errorf("commands not valid. Choose from %v", conf.ValidCommandsString())
		}
	}
	for cmd := range cmdSet {
		cmdArr = append(cmdArr, cmd)
	}

	if len(cmdArr) == 0 {
		return fmt.Errorf("must specify at least one command. Choose from %v", conf.ValidCommandsString())
	}
	runTestsConfig.Cmds = cmdArr

	return nil
}

func validateConfig(cfg conf.PerfConfig) error {
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
		if err := json.Unmarshal(d, &stack); err == nil {
			fmt.Printf("done\n")
		}
		return stack, nil
	}
}
