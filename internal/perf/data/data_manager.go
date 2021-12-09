package data

import (
	"fmt"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	vegeta "github.com/tsenart/vegeta/lib"
)

func NewDataManager(client *resty.Client, config *conf.PerfConfig) (DataManager, error) {
	dm := &dataManager{
		client: client,
		config: config,
	}

	return dm, nil
}

type DataManager interface {
	Start() error
	RunDataBroadcastTest() error
	RunDataPrivateMessageTest() error
}

type dataManager struct {
	client *resty.Client
	config *conf.PerfConfig
}

func (dm *dataManager) Start() error {
	fmt.Println("\n********** Data **********\n")
	// Broadcast
	err := dm.RunDataBroadcastTest()
	if err != nil {
		return err
	}
	// Private Messages
	if dm.config.Recipient != "" {
		err := dm.RunDataPrivateMessageTest()
		if err != nil {
			return err
		}
	}

	return nil
}

func (dm *dataManager) runAndReport(rate vegeta.Rate, targeter vegeta.Targeter, attacker vegeta.Attacker, currTime int64) error {
	var metrics vegeta.Metrics

	for res := range attacker.Attack(targeter, rate, dm.config.Duration, "FF") {
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
			pendingCount := dm.getPendingCount(currTime)
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

// Transactions Status
func (dm *dataManager) getPendingCount(currTime int64) int64 {
	var txs *conf.FilteredResult
	res, err := dm.client.R().
		SetResult(&txs).
		Get(fmt.Sprintf("/api/v1/namespaces/default/transactions?count&status=Pending&created=>=%d", currTime))

	if err != nil || !res.IsSuccess() {
		fmt.Printf("Error getting pending count: %s\n", err)
	}

	return txs.Count
}
