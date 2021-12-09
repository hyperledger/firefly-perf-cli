package tokens

import (
	"fmt"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func NewTokensManager(client *resty.Client, config *conf.PerfConfig) (TokenManager, error) {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())
	tm := &tokenManager{
		client:   client,
		config:   config,
		poolName: poolName,
	}

	return tm, nil
}

type TokenManager interface {
	Start() error
	CreateTokenPool() error
	RunTokenMintTest() error
	RunTokenTransferTest() error
	RunTokenBurnTest() error
}

type tokenManager struct {
	client   *resty.Client
	config   *conf.PerfConfig
	poolName string
}

func (tm *tokenManager) Start() error {
	fmt.Println("********** Tokens **********")
	// Create Pool
	err := tm.CreateTokenPool()
	if err != nil {
		return err
	}
	// Mint
	err = tm.RunTokenMintTest()
	if err != nil {
		return err
	}
	// Transfer
	err = tm.RunTokenTransferTest()
	if err != nil {
		return err
	}
	// Burn
	err = tm.RunTokenBurnTest()
	if err != nil {
		return err
	}

	return err
}

func (tm *tokenManager) runAndReport(rate vegeta.Rate, targeter vegeta.Targeter, attacker vegeta.Attacker, currTime int64) error {
	var metrics vegeta.Metrics

	for res := range attacker.Attack(targeter, rate, tm.config.Duration, "FF") {
		metrics.Add(res)
	}
	start := time.Now()
	metrics.Close()

	ticker := time.NewTicker(1 * time.Second)
	done := make(chan bool)

	fmt.Println("Waiting for transactions to finish....")
	go func() {
		for {
			<-ticker.C
			pendingCount := tm.getPendingCount(currTime)
			if pendingCount == 0 {
				done <- true
			}
		}
	}()
	<-done

	t := time.Now()
	elapsed := t.Sub(start)

	reporter := vegeta.NewTextReporter(&metrics)
	err := reporter(os.Stdout)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Printf("Elapsed time between last message and 0 pending transactions: %s\n", elapsed)
	return nil
}

func (tm *tokenManager) getPendingCount(currTime int64) int64 {
	var txs *conf.FilteredResult
	res, err := tm.client.R().
		SetResult(&txs).
		Get(fmt.Sprintf("/api/v1/namespaces/default/transactions?count&status=Pending&created=>=%d", currTime))

	if err != nil || !res.IsSuccess() {
		fmt.Printf("Error getting pending count: %s\n", err)
	}

	return txs.Count
}

func (tm *tokenManager) displayMessage(msg string) {
	fmt.Println("----------------------")
	fmt.Println(msg)
}
