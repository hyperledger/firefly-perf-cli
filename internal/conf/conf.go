package conf

import (
	"net/url"
	"sort"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/wsclient"
)

type FilteredResult struct {
	Count int64       `json:"count"`
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
}

type PerfConfig struct {
	Cmds      []fftypes.FFEnum
	Duration  time.Duration
	Frequency int
	Jobs      int
	Node      string
	Recipient string
	WebSocket FireFlyWsConf
	Workers   int
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

func GenerateWSConfig(conf *FireFlyWsConf) *wsclient.WSConfig {
	t, _ := url.QueryUnescape(conf.WSPath)

	return &wsclient.WSConfig{
		HTTPURL:                conf.APIEndpoint,
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
	// PerfCmdTokenMintWithMessage mints tokens with attached message in a token pool
	PerfCmdTokenMintWithMessage fftypes.FFEnum = "token_mint_with_msg"
	// PerfCmdTokenTransfer mints tokens in a token pool
	PerfCmdTokenTransfer fftypes.FFEnum = "token_transfer"
	// PerfCmdTokenBurn burns tokens in a token pool
	PerfCmdTokenBurn fftypes.FFEnum = "token_burn"
)

var ValidPerfCommands = map[string]fftypes.FFEnum{
	PerfCmdBroadcast.String():            PerfCmdBroadcast,
	PerfCmdPrivateMsg.String():           PerfCmdPrivateMsg,
	PerfCmdTokenMint.String():            PerfCmdTokenMint,
	PerfCmdTokenMintWithMessage.String(): PerfCmdTokenMintWithMessage,
	PerfCmdTokenTransfer.String():        PerfCmdTokenTransfer,
	PerfCmdTokenBurn.String():            PerfCmdTokenBurn,
}

func ValidCommandsString() []string {
	keys := make([]string, 0, len(ValidPerfCommands))
	for key := range ValidPerfCommands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
