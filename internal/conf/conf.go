package conf

import (
	"net/url"
	"sort"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/wsclient"
)

type MessageOptions struct {
	LongMessage bool
}

type TokenOptions struct {
	TokenType string
}

type ContractOptions struct {
	Address string
}

type PerfConfig struct {
	Cmds             []fftypes.FFEnum
	Length           time.Duration
	MessageOptions   MessageOptions
	Recipient        string
	RecipientAddress string
	TokenOptions     TokenOptions
	ContractOptions  ContractOptions
	WebSocket        FireFlyWsConf
	Workers          int
	NodeURLs         []string
	StackJSONPath    string
}

type FireFlyWsConf struct {
	APIEndpoint            string        `mapstructure:"apiEndpoint"`
	WSPath                 string        `mapstructure:"wsPath"`
	ReadBufferSize         int           `mapstructure:"readBufferSize"`
	WriteBufferSize        int           `mapstructure:"writeBufferSize"`
	InitialDelay           time.Duration `mapstructure:"initialDelay"`
	MaximumDelay           time.Duration `mapstructure:"maximumDelay"`
	InitialConnectAttempts int           `mapstructure:"initialConnectAttempts"`
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
	}
}

var (
	// PerfCmdBroadcast sends broadcast messages
	PerfCmdBroadcast fftypes.FFEnum = "msg_broadcast"
	// PerfCmdPrivateMsg sends private messages to a recipient in the consortium
	PerfCmdPrivateMsg fftypes.FFEnum = "msg_private"
	// PerfCmdTokenMint mints tokens in a token pool
	PerfCmdTokenMint fftypes.FFEnum = "token_mint"
	// PerfCmdCustomContract invokes and queries a custom smart contract
	PerfCmdCustomContract fftypes.FFEnum = "custom_contract"
)

var ValidPerfCommands = map[string]fftypes.FFEnum{
	PerfCmdBroadcast.String():      PerfCmdBroadcast,
	PerfCmdPrivateMsg.String():     PerfCmdPrivateMsg,
	PerfCmdTokenMint.String():      PerfCmdTokenMint,
	PerfCmdCustomContract.String(): PerfCmdCustomContract,
}

func ValidCommandsString() []string {
	keys := make([]string, 0, len(ValidPerfCommands))
	for key := range ValidPerfCommands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
