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

var NAMESPACE = "default"
var TRANSPORT_TYPE = "websockets"

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
	wsconn      wsclient.WSClient
	wsReceivers []chan bool
	wsUUID      fftypes.UUID
}

func New(config *conf.PerfConfig) PerfRunner {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())

	// Create channel based dispatch for workers
	var wsReceivers []chan bool
	for i := 0; i < config.Workers; i++ {
		wsReceivers = append(wsReceivers, make(chan bool))
	}
	// Create websocket client
	wsConfig := conf.GenerateWSConfig(&config.WebSocket)
	wsconn, err := wsclient.New(context.Background(), wsConfig, nil)
	if err != nil {
		log.Error("Could not create websocket connection: %s", err)
	}

	return &perfRunner{
		bfr:         make(chan int, config.Workers),
		cfg:         config,
		ctx:         context.Background(),
		endTime:     time.Now().Unix() + int64(config.Length.Seconds()),
		poolName:    poolName,
		shutdown:    make(chan bool),
		wsconn:      wsconn,
		wsReceivers: wsReceivers,
		wsUUID:      *fftypes.NewUUID(),
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

	// Create single subscription for test
	err = pr.createSub()
	if err != nil {
		return err
	}
	// Open websocket client for subscription
	err = pr.openWsClient()
	if err != nil {
		return err
	}

	for id := 0; id < pr.cfg.Workers; id++ {
		ptr := id % len(pr.cfg.Cmds)

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

func (pr *perfRunner) eventLoop(id int) (err error) {
	log.Infof("Event loop started for Worker #%d", id)
	for {
		select {
		// Wait to receive websocket event
		case msgBytes, ok := <-pr.wsconn.Receive():
			if !ok {
				log.Errorf("Error receiving websocket")
				return
			}
			// Handle websocket event
			var event fftypes.EventDelivery
			json.Unmarshal(msgBytes, &event)
			msgId, err := strconv.Atoi(event.Message.Header.Tag)
			if err != nil {
				log.Errorf("Could not parse message tag: %s", err)
				return err
			}
			// Ack websocket event
			ack, _ := json.Marshal(map[string]string{"type": "ack", "topic": event.ID.String()})
			log.Infof("\n\t%d - Received \n\t%d --- Event ID: %s\n\t%d --- Message ID: %s", msgId, msgId, event.ID.String(), msgId, event.Message.Header.ID.String())
			pr.wsconn.Send(pr.ctx, ack)
			// Release worker so it can continue to its next task
			pr.wsReceivers[msgId] <- true
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
			// Worker sends its task
			res, err := req.Post(fmt.Sprintf("%s/api/v1/namespaces/default/%s", pr.cfg.Node, ep))
			if err != nil {
				log.Errorf("Error sending POST /%s: %s", ep, err)
			}
			// Parse response for logging purposes
			var msgRes fftypes.Message
			json.Unmarshal(res.Body(), &msgRes)
			log.Infof("%d --> %s Sent with Message ID: %s", id, action, msgRes.Header.ID)
			// Wait for worker to confirm the message before proceeding to next task
			<-pr.wsReceivers[id]
			log.Infof("%d <-- %s Finished", id, action)
		case <-pr.shutdown:
			return nil
		}
	}
}

func (pr *perfRunner) createSub() (err error) {
	subPayload := fftypes.Subscription{
		SubscriptionRef: fftypes.SubscriptionRef{
			ID:        &pr.wsUUID,
			Name:      fmt.Sprintf("perf_%s", pr.wsUUID.String()),
			Namespace: NAMESPACE,
		},
		Ephemeral: false,
		Filter: fftypes.SubscriptionFilter{
			Events: fftypes.EventTypeMessageConfirmed.String(),
		},
		Transport: TRANSPORT_TYPE,
	}

	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/subscriptions", pr.cfg.Node))
	if err != nil {
		log.Errorf("Could not create subscription: %s", err)
		return err
	}

	return nil
}

func (pr *perfRunner) openWsClient() (err error) {
	pr.wsconn.Connect()

	var autoack = false
	startPayload := fftypes.WSClientActionStartPayload{
		WSClientActionBase: fftypes.WSClientActionBase{
			Type: fftypes.WSClientActionStart,
		},
		AutoAck:   &autoack,
		Name:      fmt.Sprintf("perf_%s", pr.wsUUID.String()),
		Namespace: "default",
	}
	start, _ := json.Marshal(startPayload)
	err = pr.wsconn.Send(pr.ctx, start)
	if err != nil {
		log.Errorf("Issuing opening websocket client: %s", err)
		return err
	}
	log.Infof("Receiving Events...")

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
