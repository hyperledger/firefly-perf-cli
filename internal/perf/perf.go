package perf

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

type PerfRunner interface {
	Init() error
	Start() error
	// Data
	RunBroadcast()
	RunPrivateMessage()
	// Tokens
	CreateTokenPool() error
	RunTokenMint()
	RunTokenMintWithMsg()
	RunTokenTransfer()
	RunTokenBurn()
}

type perfRunner struct {
	bfr      chan int
	cfg      *conf.PerfConfig
	client   *resty.Client
	ctx      context.Context
	poolName string
	shutdown chan bool
}

func New(config *conf.PerfConfig) PerfRunner {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())
	return &perfRunner{
		bfr:      make(chan int, config.Workers),
		cfg:      config,
		poolName: poolName,
		shutdown: make(chan bool),
	}
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.cfg.Node)

	return nil
}

func (pr *perfRunner) Start() error {
	if pr.cfg.Cmd == "mint" || pr.cfg.Cmd == "mint_with_msg" {
		pr.poolName = fmt.Sprintf("pool-%s", fftypes.NewUUID())
		err := pr.CreateTokenPool()
		if err != nil {
			return err
		}
	}

	diff := pr.cfg.Jobs - pr.cfg.Workers

	for i := 0; i < pr.cfg.Workers; i++ {
		switch pr.cfg.Cmd {
		case "broadcast":
			go pr.RunBroadcast()
		case "private_msg":
			go pr.RunPrivateMessage()
		case "mint":
			go pr.RunTokenMint()
		case "mint_with_msg":
			go pr.RunTokenMintWithMsg()
		default:
			return fmt.Errorf("Invalid Command")
		}
	}

	for i := 0; i < pr.cfg.Jobs+diff; i++ {
		pr.bfr <- i
	}
	pr.shutdown <- true

	return nil
}

func getFFClient(node string) *resty.Client {
	client := resty.New()
	client.SetHostURL(node)

	return client
}

func (pr *perfRunner) runAndReport(rate vegeta.Rate, targeter vegeta.Targeter, attacker vegeta.Attacker, currTime int64) error {
	var metrics vegeta.Metrics

	for res := range attacker.Attack(targeter, rate, pr.cfg.Duration, "FF") {
		metrics.Add(res)
	}
	start := time.Now()
	metrics.Close()

	ticker := time.NewTicker(3 * time.Second)
	done := make(chan bool)

	fmt.Println("Waiting for transactions to finish....")
	go func() {
		for {
			<-ticker.C
			pendingCount := pr.getPendingCount(currTime)
			if pendingCount == 0 {
				done <- true
			}
		}
	}()
	<-done

	t := time.Now()
	elapsed := t.Sub(start)

	fmt.Printf("Elapsed time between last message and 0 pending transactions: %s\n", elapsed)
	return nil
}

func (pr *perfRunner) getPendingCount(currTime int64) int64 {
	var txs *conf.FilteredResult
	res, err := pr.client.R().
		SetResult(&txs).
		Get(fmt.Sprintf("/api/v1/namespaces/default/transactions?count&status=Pending&created=>=%d", currTime))

	if err != nil || !res.IsSuccess() {
		fmt.Printf("Error getting pending count: %s\n", err)
	}

	return txs.Count
}

func (pr *perfRunner) getDataTargeter(method string, ep string, payload string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = method
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/messages/%s", pr.cfg.Node, ep)
		t.Body = []byte(payload)
		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}
func (pr *perfRunner) getTokenTargeter(method string, ep string, payload string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = method
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/tokens/%s", pr.cfg.Node, ep)
		t.Body = []byte(payload)
		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}

func (pr *perfRunner) displayMessage(msg string) {
	fmt.Println(msg)
}
