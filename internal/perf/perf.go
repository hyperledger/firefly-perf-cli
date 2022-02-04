package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/wsclient"
	log "github.com/sirupsen/logrus"
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
}

type perfRunner struct {
	bfr         chan int
	cfg         *conf.PerfConfig
	client      *resty.Client
	ctx         context.Context
	endTime     int64
	poolName    string
	shutdown    chan bool
	wsconns     []wsclient.WSClient
	wsReceivers []chan bool
}

func New(config *conf.PerfConfig) PerfRunner {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())

	var wsReceivers []chan bool
	for i := 0; i < config.Workers; i++ {
		wsReceivers = append(wsReceivers, make(chan bool))
	}

	return &perfRunner{
		bfr:         make(chan int, config.Workers),
		cfg:         config,
		ctx:         context.Background(),
		endTime:     time.Now().Unix() + int64(config.Length.Seconds()),
		poolName:    poolName,
		shutdown:    make(chan bool),
		wsconns:     make([]wsclient.WSClient, config.Workers),
		wsReceivers: wsReceivers,
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
		ptr := id % len(pr.cfg.Cmds)

		eventType := getEventFilter(id, pr.cfg.Cmds[ptr])
		err = pr.openWsClient(id, eventType)
		if err != nil {
			return err
		}

		go pr.eventLoop(id)
		switch pr.cfg.Cmds[ptr] {
		case conf.PerfCmdBroadcast:
			go pr.RunBroadcast(id)
		case conf.PerfCmdPrivateMsg:
			go pr.RunPrivateMessage(id)
		case conf.PerfCmdTokenMint:
			go pr.RunTokenMint(id)
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

func (pr *perfRunner) eventLoop(id int) {
	log.Infof("Event loop for Worker #%d started\n", id)
	for {
		select {
		case msgBytes, ok := <-pr.wsconns[id].Receive():
			if !ok {
				log.Errorf("Error receiving websocket")
				return
			}

			var event fftypes.EventDelivery
			json.Unmarshal(msgBytes, &event)
			pr.wsReceivers[id] <- true
		case <-pr.ctx.Done():
			log.Errorf("Run loop exiting (context cancelled)")
			return
		}
	}
}

func (pr *perfRunner) sendAndWait(req *resty.Request, ep string, id int, action string) error {
	for {
		select {
		case <-pr.bfr:
			log.Infof("%d --> %s Running", id, action)
			req.Post(fmt.Sprintf("%s/api/v1/namespaces/default/%s", pr.cfg.Node, ep))
			<-pr.wsReceivers[id]
			log.Infof("%d <-- %s Finished", id, action)
		case <-pr.shutdown:
			return nil
		}
	}
}

func (pr *perfRunner) openWsClient(idx int, eventFilter fftypes.SubscriptionFilter) (err error) {
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
		Filter:    eventFilter,
	}
	start, _ := json.Marshal(startPayload)

	err = pr.wsconns[idx].Send(context.Background(), start)
	if err != nil {
		log.Errorf("Issuing sending FF event start: %s", err)
		return err
	}
	log.Infof("Receiving Events for: %s", strconv.Itoa(idx))

	return nil
}

func containsTokenCmd(cmds []fftypes.FFEnum) bool {
	for _, cmd := range cmds {
		if cmd == conf.PerfCmdTokenMint {
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

func getEventFilter(id int, cmd fftypes.FFEnum) fftypes.SubscriptionFilter {
	switch cmd {
	case conf.PerfCmdBroadcast, conf.PerfCmdPrivateMsg:
		return fftypes.SubscriptionFilter{
			Tag:    strconv.Itoa(id),
			Events: fftypes.EventTypeMessageConfirmed.String(),
		}
	case conf.PerfCmdTokenMint, conf.PerfCmdTokenTransfer, conf.PerfCmdTokenBurn:
		return fftypes.SubscriptionFilter{
			Events: fftypes.EventTypeTransferConfirmed.String(),
		}
	default:
		return fftypes.SubscriptionFilter{}
	}
}
