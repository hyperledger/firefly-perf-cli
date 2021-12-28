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
	"time"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
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

Powered by vegeta, ff-perf will use a configured RPS and duration to benchmark different functions of a FireFly Node.
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if rootConfig.Node == "" {
			return errors.New("Must provide FireFly node endpoint")
		}

		if perfRunner == nil && len(args) != 0 {
			rootConfig.Cmd = args[0]
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

	rootCmd.Flags().DurationVarP(&rootConfig.Duration, "duration", "d", 60*time.Second, "Duration of test (seconds)")
	rootCmd.Flags().IntVarP(&rootConfig.Frequency, "frequency", "f", 50, "Requests Per Second (RPS) frequency")
	rootCmd.Flags().IntVarP(&rootConfig.Jobs, "jobs", "j", 100, "Number of jobs to run")
	rootCmd.Flags().StringVarP(&rootConfig.Node, "node", "n", "", "FireFly node endpoint")
	rootCmd.Flags().StringVarP(&rootConfig.Recipient, "recipient", "r", "", "Recipient for FF messages")
	rootCmd.Flags().IntVarP(&rootConfig.Workers, "workers", "w", 1, "Number of workers at a time")
}

func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}
