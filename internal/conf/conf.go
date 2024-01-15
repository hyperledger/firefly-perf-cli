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

package conf

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"time"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/wsclient"
)

type RunnerConfig struct {
	LogLevel                  string
	Tests                     []TestCaseConfig
	Length                    time.Duration
	MessageOptions            MessageOptions
	InvokeOptions             interface{}
	RecipientOrg              string
	RecipientAddress          string
	SigningKey                string
	PerWorkerSigningKeyPrefix string
	TokenOptions              TokenOptions
	ContractOptions           ContractOptions
	WebSocket                 FireFlyWsConfig
	NodeURLs                  []string
	StackJSONPath             string
	DelinquentAction          string
	Daemon                    bool
	LogEvents                 bool
	SenderURL                 string
	APIPrefix                 string
	FFNamespace               string
	FFNamespacePath           string
	MaxTimePerAction          time.Duration
	MaxActions                int64
	RampLength                time.Duration
	SkipMintConfirmations     bool
}

type PerformanceTestConfig struct {
	LogLevel      string           `yaml:"logLevel" json:"logLevel"`
	StackJSONPath string           `json:"stackJSONPath" yaml:"stackJSONPath"`
	Instances     []InstanceConfig `json:"instances" yaml:"instances"`
	WSConfig      FireFlyWsConfig  `json:"wsConfig,omitempty" yaml:"wsConfig,omitempty"`
	Daemon        bool             `json:"daemon,omitempty" yaml:"daemon,omitempty"`
	Nodes         []NodeConfig     `yaml:"nodes" json:"nodes"`
	LogEvents     bool             `json:"logEvents,omitempty" yaml:"logEvents,omitempty"`
}

type InstanceConfig struct {
	Name                      string           `yaml:"name" json:"name"`
	Tests                     []TestCaseConfig `yaml:"tests" json:"tests"`
	Length                    time.Duration    `yaml:"length" json:"length"`
	MessageOptions            MessageOptions   `json:"messageOptions,omitempty" yaml:"messageOptions,omitempty"`
	InvokeOptions             interface{}      `json:"invokeOptions,omitempty" yaml:"invokeOptions,omitempty"`
	Sender                    int              `json:"sender" yaml:"sender"`
	ManualNodeIndex           int              `json:"manualNodeIndex" yaml:"manualNodeIndex"`
	Recipient                 *int             `json:"recipient,omitempty" yaml:"recipient,omitempty"`
	SigningKey                string           `json:"signingKey,omitempty" yaml:"signingKey,omitempty"`
	TokenOptions              TokenOptions     `json:"tokenOptions,omitempty" yaml:"tokenOptions,omitempty"`
	ContractOptions           ContractOptions  `json:"contractOptions,omitempty" yaml:"contractOptions,omitempty"`
	APIPrefix                 string           `json:"apiPrefix,omitempty" yaml:"apiPrefix,omitempty"`
	FFNamespace               string           `json:"fireflyNamespace,omitempty" yaml:"fireflyNamespace,omitempty"`
	FFNamespaceBasePath       string           `json:"namespaceBasePath,omitempty" yaml:"namespaceBasePath,omitempty"`
	MaxTimePerAction          time.Duration    `json:"maxTimePerAction,omitempty" yaml:"maxTimePerAction,omitempty"`
	MaxActions                int64            `json:"maxActions,omitempty" yaml:"maxActions,omitempty"`
	RampLength                time.Duration    `json:"rampLength,omitempty" yaml:"rampLength,omitempty"`
	SkipMintConfirmations     bool             `json:"skipMintConfirmations" yaml:"skipMintConfirmations"`
	DelinquentAction          string           `json:"delinquentAction,omitempty" yaml:"delinquentAction,omitempty"`
	PerWorkerSigningKeyPrefix string           `json:"perWorkerSigningKeyPrefix,omitempty" yaml:"perWorkerSigningKeyPrefix,omitempty"`
}

type TestCaseConfig struct {
	Name           fftypes.FFEnum `json:"name" yaml:"name"`
	Workers        int            `json:"workers" yaml:"workers"`
	ActionsPerLoop int            `json:"actionsPerLoop" yaml:"actionsPerLoop"`
}

type NodeConfig struct {
	Name         string `json:"name" yaml:"name"`
	APIEndpoint  string `json:"apiEndpoint,omitempty" yaml:"apiEndpoint,omitempty"`
	AuthUsername string `json:"authUsername,omitempty" yaml:"authUsername,omitempty"`
	AuthPassword string `json:"authPassword,omitempty" yaml:"authPassword,omitempty"`
	AuthToken    string `json:"authToken,omitempty" yaml:"authToken,omitempty"`
}

type MessageOptions struct {
	LongMessage bool `json:"longMessage" yaml:"longMessage"`
}

type TokenOptions struct {
	TokenType              string        `json:"tokenType" yaml:"tokenType"`
	TokenPoolConnectorName string        `json:"poolConnectorName" yaml:"poolConnectorName"`
	SupportsData           *bool         `json:"supportsData" yaml:"supportsData"` // Needs to be a pointer to allow defaulting to 'true'
	SupportsURI            bool          `json:"supportsURI" yaml:"supportsURI"`
	ExistingPoolName       string        `json:"existingPoolName" yaml:"existingPoolName"`
	RecipientAddress       string        `json:"mintRecipient,omitempty" yaml:"mintRecipient,omitempty"`
	Config                 TokenConfig   `json:"config" yaml:"config"`
	MaxTokenBalanceWait    time.Duration `json:"maxTokenBalanceWait,omitempty" yaml:"maxTokenBalanceWait,omitempty"`
}

type TokenConfig struct {
	PoolAddress     string `json:"address" yaml:"address"`
	PoolBlockNumber string `json:"blockNumber" yaml:"blockNumber"`
}

type ContractOptions struct {
	Address   string `json:"address" yaml:"address"`
	Channel   string `json:"channel,omitempty" yaml:"channel,omitempty"`
	Chaincode string `json:"chaincode,omitempty" yaml:"chaincode,omitempty"`
}

type FireFlyWsConfig struct {
	WSPath                 string        `mapstructure:"wsPath" json:"wsPath" yaml:"wsPath"`
	ReadBufferSize         int           `mapstructure:"readBufferSize" json:"readBufferSize" yaml:"readBufferSize"`
	WriteBufferSize        int           `mapstructure:"writeBufferSize" json:"writeBufferSize" yaml:"writeBufferSize"`
	InitialDelay           time.Duration `mapstructure:"initialDelay" json:"initialDelay" yaml:"initialDelay"`
	MaximumDelay           time.Duration `mapstructure:"maximumDelay" json:"maximumDelay" yaml:"maximumDelay"`
	InitialConnectAttempts int           `mapstructure:"initialConnectAttempts" json:"initialConnectAttempts" yaml:"initialConnectAttempts"`
	HeartbeatInterval      time.Duration `mapstructure:"heartbeatInterval" json:"heartbeatInterval" yaml:"heartbeatInterval"`
	AuthUsername           string        `mapstructure:"authUsername" json:"authUsername" yaml:"authUsername"`
	AuthPassword           string        `mapstructure:"authPassword" json:"authPassword" yaml:"authPassword"`
	AuthToken              string        `mapstructure:"authToken" json:"authToken" yaml:"authToken"`
	DisableTLSVerification bool          `mapstructure:"disableTLSVerification" json:"disableTLSVerification" yaml:"disableTLSVerification"`
	ConnectionTimeout      time.Duration `mapstructure:"connectionTimeout" json:"connectionTimeout" yaml:"connectionTimeout"`
}

func GenerateWSConfig(nodeURL string, conf *FireFlyWsConfig) *wsclient.WSConfig {
	t, _ := url.QueryUnescape(conf.WSPath)

	wsConfig := wsclient.WSConfig{
		HTTPURL:                nodeURL,
		WSKeyPath:              t,
		ReadBufferSize:         conf.ReadBufferSize,
		WriteBufferSize:        conf.WriteBufferSize,
		InitialDelay:           conf.InitialDelay,
		MaximumDelay:           conf.MaximumDelay,
		InitialConnectAttempts: conf.InitialConnectAttempts,
		HeartbeatInterval:      conf.HeartbeatInterval,
		ConnectionTimeout:      conf.ConnectionTimeout,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: conf.DisableTLSVerification,
		},
	}

	if conf.AuthToken != "" {
		wsConfig.HTTPHeaders = fftypes.JSONObject{
			"Authorization": fmt.Sprintf("Bearer %s", conf.AuthToken),
		}
	} else {
		wsConfig.AuthUsername = conf.AuthUsername
		wsConfig.AuthPassword = conf.AuthPassword
	}

	return &wsConfig
}

var (
	// PerfTestBroadcast sends broadcast messages
	PerfTestBroadcast fftypes.FFEnum = "msg_broadcast"
	// PerfTestPrivateMsg sends private messages to a recipient in the consortium
	PerfTestPrivateMsg fftypes.FFEnum = "msg_private"
	// PerfTestTokenMint mints tokens in a token pool
	PerfTestTokenMint fftypes.FFEnum = "token_mint"
	// PerfTestCustomEthereumContract invokes a custom smart contract and checks events emitted by it
	PerfTestCustomEthereumContract fftypes.FFEnum = "custom_ethereum_contract"
	// PerfTestCustomFabricContract invokes a custom smart contract and checks events emitted by it
	PerfTestCustomFabricContract fftypes.FFEnum = "custom_fabric_contract"
	// PerfTestBlobBroadcast broadcasts a blob
	PerfTestBlobBroadcast fftypes.FFEnum = "blob_broadcast"
	// PerfTestBlobPrivateMsg privately sends a blob
	PerfTestBlobPrivateMsg fftypes.FFEnum = "blob_private"
)

var (
	// DelinquentActionExit causes ffperf to exit after detecting delinquent messages
	DelinquentActionExit fftypes.FFEnum = "exit"
	// DelinquentActionLog causes ffperf to log and move on after delinquent messages
	DelinquentActionLog fftypes.FFEnum = "log"
)

var ValidPerfTests = map[string]fftypes.FFEnum{
	PerfTestBroadcast.String():              PerfTestBroadcast,
	PerfTestPrivateMsg.String():             PerfTestPrivateMsg,
	PerfTestTokenMint.String():              PerfTestTokenMint,
	PerfTestCustomEthereumContract.String(): PerfTestCustomEthereumContract,
	PerfTestCustomFabricContract.String():   PerfTestCustomFabricContract,
	PerfTestBlobBroadcast.String():          PerfTestBlobBroadcast,
	PerfTestBlobPrivateMsg.String():         PerfTestBlobPrivateMsg,
}
