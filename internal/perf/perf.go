package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/wsclient"
	log "github.com/sirupsen/logrus"
)

var mutex = &sync.Mutex{}
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
	msgTimeMap  map[string]time.Time
	poolName    string
	shutdown    chan bool
	tagPrefix   string
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
	wsconn, err := wsclient.New(context.Background(), wsConfig, nil, nil)
	if err != nil {
		log.Error("Could not create websocket connection: %s", err)
	}

	wsUUID := *fftypes.NewUUID()

	return &perfRunner{
		bfr:         make(chan int, config.Workers),
		cfg:         config,
		ctx:         context.Background(),
		endTime:     time.Now().Unix() + int64(config.Length.Seconds()),
		poolName:    poolName,
		shutdown:    make(chan bool),
		tagPrefix:   fmt.Sprintf("perf_%s", wsUUID.String()),
		msgTimeMap:  make(map[string]time.Time),
		wsconn:      wsconn,
		wsReceivers: wsReceivers,
		wsUUID:      wsUUID,
	}
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.cfg.Node)

	return nil
}

func (pr *perfRunner) Start() (err error) {
	// Create token pool, if needed
	if containsTargetCmd(pr.cfg.Cmds, conf.PerfCmdTokenMint) {
		err = pr.CreateTokenPool()
		if err != nil {
			return err
		}
	}
	// Create contract sub and listener, if needed
	if containsTargetCmd(pr.cfg.Cmds, conf.PerfCmdCustomContract) {
		listenerID, err := pr.createContractListener()
		if err != nil {
			return err
		}

		err = pr.createContractsSub(listenerID)
		if err != nil {
			return err
		}
	}

	// Create subscription for message confirmations
	err = pr.createMsgConfirmSub()
	if err != nil {
		return err
	}

	// Open websocket client for subscription
	err = pr.openWsClient()
	if err != nil {
		return err
	}
	go pr.eventLoop()

	for id := 0; id < pr.cfg.Workers; id++ {
		ptr := id % len(pr.cfg.Cmds)

		switch pr.cfg.Cmds[ptr] {
		case conf.PerfCmdBroadcast:
			go pr.RunBroadcast(id)
		case conf.PerfCmdPrivateMsg:
			go pr.RunPrivateMessage(id)
		case conf.PerfCmdTokenMint:
			go pr.RunTokenMint(id)
		case conf.PerfCmdCustomContract:
			go pr.RunCustomContract(id)
		}
	}

	i := 0
	lastCheckedTime := time.Now()
	for time.Now().Unix() < pr.endTime {
		pr.bfr <- i
		i++
		if time.Since(lastCheckedTime).Seconds() > 60 {
			pr.getDelinquentMsgs()
			lastCheckedTime = time.Now()
		}
	}

	pr.shutdown <- true

	return nil
}

func (pr *perfRunner) eventLoop() (err error) {
	log.Infoln("Event loop started...")
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

			var workerID int

			switch event.Type {
			case fftypes.EventTypeBlockchainEventReceived:
				if event.BlockchainEvent == nil {
					log.Errorf("\nBlockchain event not found --- Event ID: %s\n\t%d --- Ref: %s", event.ID.String(), event.Reference)
					return fmt.Errorf("blockchain event not found for event: %s", event.ID)
				}
				value := event.BlockchainEvent.Output.GetString("value")
				workerID, err = strconv.Atoi(value)
				if err != nil {
					log.Error(err)
					continue
				} else {
					log.Infof("\n\t%d - Received \n\t%d --- Event ID: %s\n\t%d --- Ref: %s", workerID, workerID, event.ID.String(), workerID, event.Reference)
				}
			default:
				workerID, err = strconv.Atoi(strings.ReplaceAll(event.Message.Header.Tag, pr.tagPrefix+"_", ""))
				if err != nil {
					log.Errorf("Could not parse message tag: %s", err)
					continue
				}
				pr.deleteMsgTime(event.Message.Header.ID.String())
				log.Infof("\n\t%d - Received \n\t%d --- Event ID: %s\n\t%d --- Message ID: %s", workerID, workerID, event.ID.String(), workerID, event.Message.Header.ID.String())
			}

			// Ack websocket event
			ack := &fftypes.WSClientActionAckPayload{
				WSClientActionBase: fftypes.WSClientActionBase{
					Type: fftypes.WSClientActionAck,
				},
				ID: event.ID,
				Subscription: &fftypes.SubscriptionRef{
					ID: event.Subscription.ID,
				},
			}
			ackJSON, _ := json.Marshal(ack)
			pr.wsconn.Send(pr.ctx, ackJSON)
			// Release worker so it can continue to its next task
			pr.wsReceivers[workerID] <- true
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
			var tokenRes fftypes.TokenTransfer
			var contractRes fftypes.ContractCallResponse

			switch action {
			case conf.PerfCmdBroadcast.String(), conf.PerfCmdPrivateMsg.String():
				json.Unmarshal(res.Body(), &msgRes)
				pr.updateMsgTime(msgRes.Header.ID.String())
				log.Infof("%d --> %s Sent with Message ID: %s", id, action, msgRes.Header.ID)
			case conf.PerfCmdTokenMint.String():
				json.Unmarshal(res.Body(), &tokenRes)
				pr.updateMsgTime(tokenRes.LocalID.String())
				log.Infof("%d --> %s Sent with Token ID: %s", id, action, tokenRes.LocalID)
			case conf.PerfCmdCustomContract.String():
				json.Unmarshal(res.Body(), &contractRes)
				log.Infof("%d --> Invoked contract: %s", id, contractRes.ID)
			}
			// Wait for worker to confirm the message before proceeding to next task
			<-pr.wsReceivers[id]
			log.Infof("%d <-- %s Finished", id, action)
		case <-pr.shutdown:
			return nil
		}
	}
}

func (pr *perfRunner) createMsgConfirmSub() (err error) {
	var readAhead uint16 = 50
	subPayload := fftypes.Subscription{
		SubscriptionRef: fftypes.SubscriptionRef{
			ID:        &pr.wsUUID,
			Name:      pr.tagPrefix,
			Namespace: NAMESPACE,
		},
		Options: fftypes.SubscriptionOptions{
			SubscriptionCoreOptions: fftypes.SubscriptionCoreOptions{
				ReadAhead: &readAhead,
			},
		},
		Ephemeral: false,
		Filter: fftypes.SubscriptionFilter{
			Events: fftypes.EventTypeMessageConfirmed.String(),
			Message: fftypes.MessageFilter{
				Tag: fmt.Sprintf("^%s_", pr.tagPrefix),
			},
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

	log.Infof("Created subscription: %s", pr.wsUUID.String())

	return nil
}

func (pr *perfRunner) openWsClient() (err error) {
	pr.wsconn.Connect()
	if err := pr.startSubscription(pr.tagPrefix); err != nil {
		return err
	}
	if err := pr.startSubscription(fmt.Sprintf("%s_contracts", pr.tagPrefix)); err != nil {
		return err
	}
	return nil
}

func (pr *perfRunner) startSubscription(name string) (err error) {
	var autoack = false
	startPayload := fftypes.WSClientActionStartPayload{
		WSClientActionBase: fftypes.WSClientActionBase{
			Type: fftypes.WSClientActionStart,
		},
		AutoAck:   &autoack,
		Name:      name,
		Namespace: "default",
	}
	start, _ := json.Marshal(startPayload)
	err = pr.wsconn.Send(pr.ctx, start)
	if err != nil {
		log.Errorf("Issuing opening websocket client: %s", err)
		return err
	}
	log.Infof(`Receiving Events on subscription: "%s"`, name)
	return nil
}

func containsTargetCmd(cmds []fftypes.FFEnum, target fftypes.FFEnum) bool {
	for _, cmd := range cmds {
		if cmd == target {
			return true
		}
	}

	return false
}

func getFFClient(node string) *resty.Client {
	client := resty.New()
	client.SetBaseURL(node)

	return client
}

func (pr *perfRunner) getDelinquentMsgs() {
	mutex.Lock()
	delinquentMsgs := make(map[string]time.Time)
	for msgId, timeLastSeen := range pr.msgTimeMap {
		if time.Since(timeLastSeen).Seconds() > 60 {
			delinquentMsgs[msgId] = timeLastSeen
		}
	}
	mutex.Unlock()
	dw, err := json.MarshalIndent(delinquentMsgs, "", "  ")
	if err != nil {
		log.Errorf("Error printing delinquent messages: %s", err)
		return
	}

	log.Warnf("Delinquent Messages:\n%s", string(dw))
}

func (pr *perfRunner) updateMsgTime(msgId string) {
	mutex.Lock()
	if len(msgId) > 0 {
		pr.msgTimeMap[msgId] = time.Now()
	}
	mutex.Unlock()
}

func (pr *perfRunner) deleteMsgTime(msgId string) {
	mutex.Lock()
	delete(pr.msgTimeMap, msgId)
	mutex.Unlock()
}

func (pr *perfRunner) createContractListener() (string, error) {
	subPayload := fmt.Sprintf(`{
		"location": {
			"address": "%s"
		},
		"event": {
			"name": "Changed",
			"description": "",
			"params": [
				{
					"name": "from",
					"schema": {
						"type": "string",
						"details": {
							"type": "address",
							"internalType": "address",
							"indexed": true
						}
					}
				},
				{
					"name": "value",
					"schema": {
						"type": "integer",
						"details": {
							"type": "uint256",
							"internalType": "uint256"
						}
					}
				}
			]
		}
	}`, pr.cfg.ContractOptions.Address)

	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/contracts/listeners", pr.cfg.Node))
	if err != nil {
		return "", err
	}
	var responseBody map[string]interface{}
	err = json.Unmarshal(res.Body(), &responseBody)
	if err != nil {
		return "", err
	}
	id := responseBody["id"].(string)
	log.Infof("Created contract listener: %s", id)
	return id, nil
}

func (pr *perfRunner) createContractsSub(listenerID string) (err error) {
	subPayload := fftypes.Subscription{
		SubscriptionRef: fftypes.SubscriptionRef{
			Name:      fmt.Sprintf("%s_contracts", pr.tagPrefix),
			Namespace: NAMESPACE,
		},
		Ephemeral: false,
		Filter: fftypes.SubscriptionFilter{
			Events: fftypes.EventTypeBlockchainEventReceived.String(),
			BlockchainEvent: fftypes.BlockchainEventFilter{
				Listener: listenerID,
			},
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

	log.Infof("Created subscription: %s", pr.wsUUID.String())

	return nil
}
