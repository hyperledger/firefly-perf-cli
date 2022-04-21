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
	"net/url"
	"sort"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/wsclient"
)

type MessageOptions struct {
	LongMessage bool `json:"longMessage" yaml:"longMessage"`
}

type TokenOptions struct {
	TokenType string `json:"tokenType" yaml:"tokenType"`
}

type ContractOptions struct {
	Address   string `json:"address" yaml:"address"`
	Channel   string `json:"channel" yaml:"channel"`
	Chaincode string `json:"chaincode" yaml:"chaincode"`
}

type PerfRunnerConfig struct {
	Tests            []fftypes.FFEnum
	Length           time.Duration
	MessageOptions   MessageOptions
	Recipient        string
	RecipientAddress string
	TokenOptions     TokenOptions
	ContractOptions  ContractOptions
	WebSocket        FireFlyWsConf
	Workers          int
	Nodes            map[string]Node
	StackJSONPath    string
	DelinquentAction string
	Daemon           bool
	Sender           string
}

type PerformanceTestConfig struct {
	StackJSONPath string           `json:"stackJSONPath" yaml:"stackJSONPath"`
	Instances     []InstanceConfig `json:"instances" yaml:"instances"`
	WSConfig      FireFlyWsConf    `json:"wsConfig,omitempty" yaml:"wsConfig,omitempty"`
	Daemon        bool             `json:"daemon,omitempty" yaml:"daemon,omitempty"`
}

type InstanceConfig struct {
	Name             string           `yaml:"name" json:"name"`
	Tests            []fftypes.FFEnum `yaml:"tests" json:"test"`
	Length           time.Duration    `yaml:"length" json:"length"`
	MessageOptions   MessageOptions   `json:"messageOptions,omitempty" yaml:"messageOptions,omitempty"`
	Sender           string           `json:"sender" yaml:"sender"`
	Recipient        string           `json:"recipient,omitempty" yaml:"recipient,omitempty"`
	RecipientAddress string           `json:"recipientAddress,omitempty" yaml:"recipientAddress,omitempty"`
	TokenOptions     TokenOptions     `json:"tokenOptions,omitempty" yaml:"tokenOptions,omitempty"`
	ContractOptions  ContractOptions  `json:"contractOptions,omitempty" yaml:"contractOptions,omitempty"`
	Workers          int              `json:"workers" yaml:"workers"`
}

type Node struct {
	URL      string
	DID      string
	Name     string
	OrgName  string
	OrgDID   string
	Username string
	Password string
}

type FireFlyWsConf struct {
	APIEndpoint            string        `mapstructure:"apiEndpoint" json:"apiEndpoint" yaml:"apiEndpoint"`
	WSPath                 string        `mapstructure:"wsPath" json:"wsPath" yaml:"wsPath"`
	ReadBufferSize         int           `mapstructure:"readBufferSize" json:"readBufferSize" yaml:"readBufferSize"`
	WriteBufferSize        int           `mapstructure:"writeBufferSize" json:"writeBufferSize" yaml:"writeBufferSize"`
	InitialDelay           time.Duration `mapstructure:"initialDelay" json:"initialDelay" yaml:"initialDelay"`
	MaximumDelay           time.Duration `mapstructure:"maximumDelay" json:"maximumDelay" yaml:"maximumDelay"`
	InitialConnectAttempts int           `mapstructure:"initialConnectAttempts" json:"initialConnectAttempts" yaml:"initialConnectAttempts"`
	HeartbeatInterval      time.Duration `mapstructure:"heartbeatInterval" json:"heartbeatInterval" yaml:"heartbeatInterval"`
}

func GenerateWSConfig(nodeURL string, conf *FireFlyWsConf) *wsclient.WSConfig {
	t, _ := url.QueryUnescape(conf.WSPath)

	return &wsclient.WSConfig{
		HTTPURL:                nodeURL,
		WSKeyPath:              t,
		ReadBufferSize:         conf.ReadBufferSize,
		WriteBufferSize:        conf.WriteBufferSize,
		InitialDelay:           conf.InitialDelay,
		MaximumDelay:           conf.MaximumDelay,
		InitialConnectAttempts: conf.InitialConnectAttempts,
		HeartbeatInterval:      conf.HeartbeatInterval,
	}
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

func ValidPerfTestsString() []string {
	keys := make([]string, 0, len(ValidPerfTests))
	for key := range ValidPerfTests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
