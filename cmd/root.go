/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var perfRunner perf.PerfRunner
var rootConfig conf.PerfConfig

func GetFireflyAsciiArt() string {
	s := ""
	s += "\u001b[33m    _______           ________     \u001b[0m\n"   // yellow
	s += "\u001b[33m   / ____(_)_______  / ____/ /_  __\u001b[0m\n"   // yellow
	s += "\u001b[31m  / /_  / / ___/ _ \\/ /_  / / / / /\u001b[0m\n"  // red
	s += "\u001b[31m / __/ / / /  /  __/ __/ / / /_/ / \u001b[0m\n"   // red
	s += "\u001b[35m/_/   /_/_/   \\___/_/   /_/\\__, /  \u001b[0m\n" // magenta
	s += "\u001b[35m                          /____/   \u001b[0m\n"   // magenta

	return s
}

var rootCmd = &cobra.Command{
	Use:   "ff-perf",
	Short: "A CLI tool to generate synthetic load against a FireFly node",
	Long: GetFireflyAsciiArt() + `
FireFly Performance CLI is a tool to generate synthetic load against a FireFly node.
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		rootConfig.WebSocket = conf.FireFlyWsConf{
			APIEndpoint:            fmt.Sprintf("%s/api/v1", rootConfig.Node),
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
			err = validateConfig(rootConfig)
			if err != nil {
				return err
			}

			perfRunner = perf.New(&rootConfig)
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

func run() error {
	err := perfRunner.Init()
	if err != nil {
		return err
	}
	return perfRunner.Start()
}

func init() {
	viper.SetEnvPrefix("FP")
	viper.AutomaticEnv()

	logger := &log.Logger{
		Out:   os.Stderr,
		Level: log.DebugLevel,
		Formatter: &log.TextFormatter{
			DisableSorting:  false,
			ForceColors:     true,
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02T15:04:05.000",
		},
	}
	log.SetFormatter(logger.Formatter)

	rootCmd.Flags().DurationVarP(&rootConfig.Length, "length", "l", 60*time.Second, "Length of entire performance test")
	rootCmd.Flags().BoolVar(&rootConfig.MessageOptions.LongMessage, "longMessage", false, "Include long string in message")
	rootCmd.Flags().StringVarP(&rootConfig.Node, "node", "n", "http://localhost:5000", "FireFly node endpoint to test")
	rootCmd.Flags().StringVarP(&rootConfig.Recipient, "recipient", "r", "", "Recipient for FireFly messages")
	rootCmd.Flags().StringVarP(&rootConfig.RecipientAddress, "recipientAddress", "x", "", "Recipient address for FireFly transfers")
	rootCmd.Flags().StringVar(&rootConfig.TokenOptions.TokenType, "tokenType", fftypes.TokenTypeFungible.String(), fmt.Sprintf("[%s %s]", fftypes.TokenTypeFungible.String(), fftypes.TokenTypeNonFungible.String()))
	rootCmd.Flags().IntVarP(&rootConfig.Workers, "workers", "w", 1, "Number of workers at a time")
	rootCmd.Flags().StringVarP(&rootConfig.ContractOptions.Address, "address", "a", "", "Address of custom contract")
}

func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		log.Errorln(err)
		return 1
	}
	return 0
}

func validateCommands(cmds []string) error {
	cmdArr := []fftypes.FFEnum{}
	cmdSet := make(map[fftypes.FFEnum]bool, 0)
	for _, cmd := range cmds {
		if val, ok := conf.ValidPerfCommands[cmd]; ok {
			cmdSet[val] = true
		} else {
			return errors.New(fmt.Sprintf("Commands not valid. Choose from %v", conf.ValidCommandsString()))
		}
	}
	for cmd := range cmdSet {
		cmdArr = append(cmdArr, cmd)
	}

	if len(cmdArr) == 0 {
		return errors.New(fmt.Sprintf("Must specify at least one command. Choose from %v", conf.ValidCommandsString()))
	}
	rootConfig.Cmds = cmdArr

	return nil
}

func validateConfig(cfg conf.PerfConfig) error {
	if cfg.TokenOptions.TokenType != fftypes.TokenTypeFungible.String() && cfg.TokenOptions.TokenType != fftypes.TokenTypeNonFungible.String() {
		return errors.New(fmt.Sprintf("Invalid token type. Choose from [%s %s]", fftypes.TokenTypeFungible.String(), fftypes.TokenTypeNonFungible.String()))
	}

	return nil
}
