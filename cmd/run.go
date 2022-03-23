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
	"context"
	"encoding/json"
	"fmt"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

var configFilePath string
var instanceName string
var instanceIndex int
var daemonOverride bool

var perfRunner perf.PerfRunner

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network",
	Long: GetFireflyAsciiArt() + `
Executes a instance within a performance test suite to generate synthetic load across multiple FireFly nodes within a network
	`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		config, err := loadPerfConfig(configFilePath)
		if err != nil {
			return err
		}

		if !config.Daemon {
			config.Daemon = daemonOverride
		}

		if instanceName != "" && instanceIndex != -1 {
			log.Warn("both the \"instance-name\" and \"instance-index\" flags were provided, using \"instance-name\"")
		}

		var instance *conf.InstanceConfig
		if instanceName != "" {
			for _, i := range config.Instances {
				if i.Name == instanceName {
					instance = &i
					break
				}
			}
			if instance == nil {
				return errors.Errorf("did not find instance named \"%s\" within the provided config", instanceName)
			}
		} else if instanceIndex != -1 {
			if instanceIndex >= len(config.Instances) || instanceIndex < 0 {
				return errors.Errorf("provided instance index \"%d\" is outside of the range of instances within the provided config", instanceIndex)
			}
			instance = &config.Instances[instanceIndex]
		} else {
			return errors.Errorf("please set either the \"instance-name\" or \"instance-index\" ")
		}

		runnerConfig, err := loadRunnerConfigFromInstance(instance, config)
		if err != nil {
			return err
		}

		if runnerConfig.StackJSONPath == "" {
			runnerConfig.NodeURLs = []string{"http://localhost:5000"}
		} else {
			stack, err := readStackJSON(runnerConfig.StackJSONPath)
			if err != nil {
				return err
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

		perfRunner = perf.New(runnerConfig)

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		err := perfRunner.Init()
		if err != nil {
			return err
		}

		if perfRunner.IsDaemon() {
			go runDaemonServer()
		}
		return perfRunner.Start()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&configFilePath, "config", "c", "", "Path to performance config that describes the network and test instances")
	runCmd.Flags().StringVarP(&instanceName, "instance-name", "n", "", "Instance within performance config to run against the network")
	runCmd.Flags().IntVarP(&instanceIndex, "instance-idx", "i", -1, "Index of the instance within performance config to run against the network")
	runCmd.Flags().BoolVarP(&daemonOverride, "daemon", "d", false, "Run in long-lived, daemon mode. Any provided test length is ignored.")

	runCmd.MarkFlagRequired("config")
}

func loadPerfConfig(filename string) (*conf.PerformanceTestConfig, error) {
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

func loadRunnerConfigFromInstance(instance *conf.InstanceConfig, perfConfig *conf.PerformanceTestConfig) (*conf.PerfRunnerConfig, error) {
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

	err := validateConfig(*runnerConfig)
	if err != nil {
		return nil, err
	}

	return runnerConfig, nil
}

func runDaemonServer() {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":5050",
		Handler: mux,
	}

	mux.HandleFunc("/status", func(writer http.ResponseWriter, request *http.Request) {
		status := struct {
			Up bool `json:"up"`
		}{Up: true}

		writer.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(writer)
		err := encoder.Encode(&status)
		if err != nil {
			log.Error(err)
		}

		return
	})
	mux.Handle("/metrics", promhttp.Handler())

	idleConnsClosed := make(chan struct{})
	go func() {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt)
		signal.Notify(signalCh, os.Kill)
		signal.Notify(signalCh, syscall.SIGTERM)
		signal.Notify(signalCh, syscall.SIGQUIT)
		signal.Notify(signalCh, syscall.SIGKILL)

		<-signalCh
		log.Warnf("Received shutdown signal, shutting down webserver in 500ms")
		time.Sleep(500 * time.Millisecond)
		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Errorf("HTTP server Shutdown: %v\n", err)
		}
		close(idleConnsClosed)
	}()

	log.Info("Starting daemon HTTP server")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Errorf("HTTP server ListenAndServe: %v\n", err)
	}

	<-idleConnsClosed
}
