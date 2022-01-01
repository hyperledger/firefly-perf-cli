package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/wsclient"
	log "github.com/sirupsen/logrus"
	vegeta "github.com/tsenart/vegeta/lib"
)

type PerfRunner interface {
	Init() error
	Start() error
	// Data
	RunBroadcast(uuid fftypes.UUID)
	RunPrivateMessage(uuid fftypes.UUID)
	// Tokens
	CreateTokenPool() error
	RunTokenMint(uuid fftypes.UUID)
	RunTokenMintWithMsg(uuid fftypes.UUID)
	RunTokenTransfer(uuid fftypes.UUID)
	RunTokenBurn(uuid fftypes.UUID)
}

type perfRunner struct {
	bfr      chan int
	cfg      *conf.PerfConfig
	client   *resty.Client
	poolName string
	shutdown chan bool
	wsconn   wsclient.WSClient
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

func (pr *perfRunner) Start() (err error) {
	// Connect to ws
	wsConfig := conf.GenerateWSConfig(&pr.cfg.WebSocket)
	pr.wsconn, err = wsclient.New(context.Background(), wsConfig, nil)
	if err != nil {
		return err
	}
	pr.wsconn.Connect()

	// Token pool
	if containsTokenCmd(pr.cfg.Cmds) {
		err = pr.CreateTokenPool()
		if err != nil {
			return err
		}
	}

	for i := 0; i < pr.cfg.Workers; i++ {
		workerID := fftypes.NewUUID()
		pr.startWsClient(*workerID)
		ptr := i % len(pr.cfg.Cmds)

		switch pr.cfg.Cmds[ptr] {
		case conf.PerfCmdBroadcast:
			go pr.RunBroadcast(*workerID)
		case conf.PerfCmdPrivateMsg:
			go pr.RunPrivateMessage(*workerID)
		case conf.PerfCmdTokenMint:
			go pr.RunTokenMint(*workerID)
		case conf.PerfCmdTokenMintWithMessage:
			go pr.RunTokenMintWithMsg(*workerID)
		case conf.PerfCmdTokenTransfer:
			go pr.RunTokenTransfer(*workerID)
		case conf.PerfCmdTokenBurn:
			go pr.RunTokenBurn(*workerID)
		}
	}

	for i := 0; i < pr.cfg.Jobs; i++ {
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

func (pr *perfRunner) startWsClient(uuid fftypes.UUID) {
	var autoack = true
	startPayload := fftypes.WSClientActionStartPayload{
		WSClientActionBase: fftypes.WSClientActionBase{
			Type: fftypes.WSClientActionStart,
		},
		AutoAck:   &autoack,
		Ephemeral: true,
		Namespace: "default",
		Filter: fftypes.SubscriptionFilter{
			Tag: uuid.String(),
		},
	}
	start, _ := json.Marshal(startPayload)

	log.Infof("Receiving Events for: %s\n", uuid.String())
	err := pr.wsconn.Send(context.Background(), start)
	if err != nil {
		log.Errorf("Issuing sending FF event start: %s", err)
	}
}

func (pr *perfRunner) runAndReport(rate vegeta.Rate, targeter vegeta.Targeter, attacker vegeta.Attacker, uuid fftypes.UUID) error {
	log.Infof("Began running %s", uuid.String())
	var metrics vegeta.Metrics

	for res := range attacker.Attack(targeter, rate, pr.cfg.Duration, "FF") {
		metrics.Add(res)
	}
	metrics.Close()
	log.Infof("Finished running %s", uuid.String())

	counter := 0
	for {
		select {
		case msgBytes, ok := <-pr.wsconn.Receive():
			counter++
			if !ok {
				log.Errorf("Error receiving websocket")
			}

			var event fftypes.EventDelivery
			err := json.Unmarshal(msgBytes, &event)
			if err != nil {
				return err
			}
			if counter == int(pr.cfg.Frequency) {
				return nil
			}
		}
	}
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

func containsTokenCmd(cmds []fftypes.FFEnum) bool {
	for _, cmd := range cmds {
		if cmd == conf.PerfCmdTokenMint ||
			cmd == conf.PerfCmdTokenMintWithMessage ||
			cmd == conf.PerfCmdTokenTransfer ||
			cmd == conf.PerfCmdTokenBurn {
			return true
		}
	}
	return false
}
