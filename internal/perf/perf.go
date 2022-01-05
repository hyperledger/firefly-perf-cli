package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	RunBroadcast(id int)
	RunPrivateMessage(id int)
	// Tokens
	CreateTokenPool() error
	RunTokenMint(id int)
	RunTokenTransfer(id int)
	RunTokenBurn(id int)
}

type perfRunner struct {
	bfr      chan int
	cfg      *conf.PerfConfig
	client   *resty.Client
	ctx      context.Context
	endTime  int64
	poolName string
	rcvBfr   []chan bool
	shutdown chan bool
	wsconns  []wsclient.WSClient
}

func New(config *conf.PerfConfig) PerfRunner {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())
	var receiveChannelArray []chan bool

	for i := 0; i < config.Workers; i++ {
		receiveChannelArray = append(receiveChannelArray, make(chan bool))
	}

	return &perfRunner{
		bfr:      make(chan int, config.Workers),
		cfg:      config,
		ctx:      context.Background(),
		endTime:  time.Now().Unix() + int64(config.Length.Seconds()),
		poolName: poolName,
		rcvBfr:   receiveChannelArray,
		shutdown: make(chan bool),
		wsconns:  make([]wsclient.WSClient, config.Workers),
	}
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.cfg.Node)

	return nil
}

func (pr *perfRunner) Start() (err error) {
	// Create token pool, if needed
	if containsTokenCmd(pr.cfg.Cmds) {
		err = pr.CreateTokenPool()
		if err != nil {
			return err
		}
	}

	for id := 0; id < pr.cfg.Workers; id++ {
		err = pr.openWsClient(id)
		if err != nil {
			return err
		}

		go pr.eventLoop(id)

		ptr := id % len(pr.cfg.Cmds)

		switch pr.cfg.Cmds[ptr] {
		case conf.PerfCmdGetTransactions:
			pr.wsconns[id].Close()
			go pr.RunGetTransactions(id)
		case conf.PerfCmdBroadcast:
			go pr.RunBroadcast(id)
		case conf.PerfCmdPrivateMsg:
			go pr.RunPrivateMessage(id)
		case conf.PerfCmdTokenMint:
			go pr.RunTokenMint(id)
		case conf.PerfCmdTokenTransfer:
			go pr.RunTokenTransfer(id)
		case conf.PerfCmdTokenBurn:
			go pr.RunTokenBurn(id)
		}
	}

	i := 0
	for time.Now().Unix() < pr.endTime {
		pr.bfr <- i
		i++
	}

	pr.shutdown <- true

	return nil
}

func (pr *perfRunner) openWsClient(idx int) (err error) {
	wsConfig := conf.GenerateWSConfig(&pr.cfg.WebSocket)
	pr.wsconns[idx], err = wsclient.New(pr.ctx, wsConfig, nil)
	if err != nil {
		return err
	}
	pr.wsconns[idx].Connect()

	var autoack = true
	startPayload := fftypes.WSClientActionStartPayload{
		WSClientActionBase: fftypes.WSClientActionBase{
			Type: fftypes.WSClientActionStart,
		},
		AutoAck:   &autoack,
		Ephemeral: true,
		Namespace: "default",
		Filter: fftypes.SubscriptionFilter{
			Tag: strconv.Itoa(idx),
		},
	}
	start, _ := json.Marshal(startPayload)

	err = pr.wsconns[idx].Send(context.Background(), start)
	if err != nil {
		log.Errorf("Issuing sending FF event start: %s", err)
		return err
	}
	log.Infof("Receiving Events for: %s\n", strconv.Itoa(idx))

	return nil
}

func (pr *perfRunner) eventLoop(id int) {
	// if confirmTransactions {
	counter := 0

	for {
		select {
		case msgBytes, ok := <-pr.wsconns[id].Receive():
			if !ok {
				log.Errorf("Error receiving websocket")
			}

			var event fftypes.EventDelivery
			json.Unmarshal(msgBytes, &event)

			counter++
			if counter == pr.cfg.Frequency {
				counter = 0
				pr.rcvBfr[id] <- true
			}
		case <-pr.ctx.Done():
			log.Errorf("Run loop exiting (context cancelled)")
		}
	}
	// }
}

func (pr *perfRunner) runAttacker(rate vegeta.Rate, targeter vegeta.Targeter, attacker vegeta.Attacker, id int) error {
	for {
		select {
		case <-pr.bfr:
			log.Infof("%d --> Running", id)
			var metrics vegeta.Metrics
			for res := range attacker.Attack(targeter, rate, pr.cfg.JobDuration, "FF") {
				metrics.Add(res)
			}
			metrics.Close()
			<-pr.rcvBfr[id]
			log.Infof("%d <-- Finished", id)
		case <-pr.shutdown:
			return nil
		}
	}
}

func (pr *perfRunner) getApiTargeter(method string, ep string, payload string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = method
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/%s", pr.cfg.Node, ep)
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
			cmd == conf.PerfCmdTokenTransfer ||
			cmd == conf.PerfCmdTokenBurn {
			return true
		}
	}
	return false
}

func getFFClient(node string) *resty.Client {
	client := resty.New()
	client.SetHostURL(node)

	return client
}
