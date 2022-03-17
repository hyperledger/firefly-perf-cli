/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hyperledger/firefly-perf-cli/internal/version"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var shortened = false
var output = "json"

// Info creates a formattable struct for version output
type Info struct {
	Version string `json:"Version,omitempty" yaml:"Version,omitempty"`
	Commit  string `json:"Commit,omitempty" yaml:"Commit,omitempty"`
	Date    string `json:"Date,omitempty" yaml:"Date,omitempty"`
	License string `json:"License,omitempty" yaml:"License,omitempty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version info",
	Long:  "Prints the version info of the CLI binary",
	RunE: func(cmd *cobra.Command, args []string) error {
		if shortened {
			fmt.Println(version.Version)
		} else {
			info := &Info{
				Version: version.Version,
				Commit:  version.Commit,
				Date:    version.Date,
				License: version.License,
			}

			var (
				bytes []byte
				err   error
			)

			switch output {
			case "json":
				bytes, err = json.MarshalIndent(info, "", "  ")
				break
			case "yaml":
				bytes, err = yaml.Marshal(info)
				break
			default:
				return errors.New(fmt.Sprintf("invalid output '%s'", output))
			}

			if err != nil {
				return err
			}

			fmt.Println(string(bytes))
		}

		return nil
	},
}

func init() {
	versionCmd.Flags().BoolVarP(&shortened, "short", "s", false, "print only the version")
	versionCmd.Flags().StringVarP(&output, "output", "o", "json", "output format (\"yaml\"|\"json\")")
	rootCmd.AddCommand(versionCmd)
}
