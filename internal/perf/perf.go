package perf

import (
	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly-perf-cli/internal/perf/data"
	"github.com/hyperledger/firefly-perf-cli/internal/perf/tokens"
)

type PerfRunner struct {
	config *conf.PerfConfig
	client *resty.Client
	dm     data.DataManager
	tm     tokens.TokenManager
}

func New(config *conf.PerfConfig) *PerfRunner {
	pr := &PerfRunner{
		config: config,
	}
	return pr
}

func (pr *PerfRunner) Init() (err error) {
	pr.client = getFFClient(pr.config.Node)
	pr.dm, err = data.NewDataManager(pr.client, pr.config)
	if err != nil {
		return err
	}
	pr.tm, err = tokens.NewTokensManager(pr.client, pr.config)
	if err != nil {
		return err
	}

	return nil
}

func (pr *PerfRunner) Start() error {
	// Data
	err := pr.dm.Start()
	if err != nil {
		return err
	}
	// Tokens
	err = pr.tm.Start()
	if err != nil {
		return err
	}

	return err
}

func getFFClient(node string) *resty.Client {
	client := resty.New()
	client.SetHostURL(node)

	return client
}
