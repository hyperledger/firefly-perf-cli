package perf

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type FilteredResult struct {
	Count int64       `json:"count"`
	Total int64       `json:"total"`
	Items interface{} `json:"items"`
}

type PerfOptions struct {
	Frequency int
	Duration  time.Duration
	Node      string
	Recipient string
}

type PerfRunner struct {
	conf   *PerfOptions
	client *resty.Client
}

func New(options *PerfOptions) *PerfRunner {
	return &PerfRunner{
		conf:   options,
		client: getFFClient(options.Node),
	}
}

func (pf *PerfRunner) Start() error {
	err := pf.runBroadcastTest()
	if err != nil {
		fmt.Printf("Error running broadcast benchmark: %s", err)
	}

	if pf.conf.Recipient != "" {
		err = pf.runPrivateMessageTest()
		if err != nil {
			fmt.Printf("Error running private messaging benchmark: %s", err)
		}
	}

	err = pf.runTokensTest()
	if err != nil {
		fmt.Printf("Error running tokens benchmark: %s", err)
	}

	return err
}

func (pf *PerfRunner) runTokensTest() error {
	fmt.Println("Running tokens performance benchmark")
	rate := vegeta.Rate{Freq: pf.conf.Frequency, Per: time.Second}

	poolName, err := pf.createTokenPool()
	if err != nil {
		return err
	}

	targeter := pf.getTokensTargeter(poolName)
	attacker := vegeta.NewAttacker()

	return pf.runAndReport(rate, targeter, *attacker)
}

func (pf *PerfRunner) runBroadcastTest() error {
	fmt.Println("Running broadcast performance benchmark")
	rate := vegeta.Rate{Freq: pf.conf.Frequency, Per: time.Second}

	targeter := pf.getBroadcastTargeter()
	attacker := vegeta.NewAttacker()

	return pf.runAndReport(rate, targeter, *attacker)
}

func (pf *PerfRunner) runPrivateMessageTest() error {
	fmt.Println("Running private message performance benchmark")
	rate := vegeta.Rate{Freq: pf.conf.Frequency, Per: time.Second}

	targeter := pf.getPrivateMessageTargeter()
	attacker := vegeta.NewAttacker()

	return pf.runAndReport(rate, targeter, *attacker)
}

func (pf *PerfRunner) runAndReport(rate vegeta.Rate, targeter vegeta.Targeter, attacker vegeta.Attacker) error {
	var metrics vegeta.Metrics

	for res := range attacker.Attack(targeter, rate, pf.conf.Duration, "FF") {
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
			pendingCount := pf.getPendingCount()
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

func (pf *PerfRunner) getTokensTargeter(poolName string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/tokens/mint", pf.conf.Node)
		payload := fmt.Sprintf(`{
			"pool": "%s",
			"amount": 10
		}`, poolName)

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}

func (pf *PerfRunner) getPrivateMessageTargeter() vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/messages/private", pf.conf.Node)
		payload := fmt.Sprintf(`{
			"data": [
				{
					"value": {
						"private": "message"
					}
				}
			],
			"group": {
				"members": [
					{
						"identity": "%s"
					}
				]
			}
		}`, pf.conf.Recipient)

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}

func (pf *PerfRunner) getBroadcastTargeter() vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/messages/broadcast", pf.conf.Node)
		payload := `{
			"data": [
				{
					"value": {
						"test": "json"
					}
				}
			]
		}`

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}

func (pf *PerfRunner) getPendingCount() int64 {
	var txs *FilteredResult
	res, err := pf.client.R().
		SetResult(&txs).
		Get("/api/v1/namespaces/default/transactions?count&status=Pending")

	if err != nil || !res.IsSuccess() {
		fmt.Printf("Error getting pending count: %s\n", err)
	}

	return txs.Count
}

func (pf *PerfRunner) createTokenPool() (pool string, err error) {
	poolName := "test"
	body := fftypes.TokenPool{
		Name: poolName,
		Type: fftypes.TokenTypeFungible,
	}

	res, err := pf.client.R().
		SetBody(&body).
		Post("/api/v1/namespaces/default/tokens/pools")

	if err != nil || !res.IsSuccess() {
		return "", errors.New("failed to create token pool")
	}
	return poolName, err
}

func getFFClient(node string) *resty.Client {
	client := resty.New()
	client.SetHostURL(node)

	return client
}
