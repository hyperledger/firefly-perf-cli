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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

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
	Use:   "ffperf",
	Short: "A CLI tool to generate synthetic load against and triage performance issues within a FireFly network",
	Long: GetFireflyAsciiArt() + `
FireFly Performance CLI is a tool to generate synthetic load against and triage performance issues within a FireFly network.
	`,
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
}

func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		log.Errorln(err)
		return 1
	}
	return 0
}
