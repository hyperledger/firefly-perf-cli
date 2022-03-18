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
	RunBroadcast(nodeURL string, id int)
	RunPrivateMessage(nodeURL string, id int)
	// Tokens
	CreateTokenPool() error
	RunTokenMint(nodeURL string, id int)
}

type perfRunner struct {
	bfr             chan int
	cfg             *conf.PerfConfig
	client          *resty.Client
	ctx             context.Context
	endTime         int64
	msgTimeMap      map[string]time.Time
	poolName        string
	shutdown        chan bool
	tagPrefix       string
	wsconns         []wsclient.WSClient
	wsReceivers     []chan bool
	wsUUID          fftypes.UUID
	nodeURLs        []string
	subscriptionMap map[string]SubscriptionInfo
}

type SubscriptionInfo struct {
	NodeURL string
	Job     fftypes.FFEnum
}

func New(config *conf.PerfConfig) PerfRunner {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())

	// Create channel based dispatch for workers
	var wsReceivers []chan bool
	for i := 0; i < config.Workers; i++ {
		wsReceivers = append(wsReceivers, make(chan bool))
	}

	wsconns := make([]wsclient.WSClient, len(config.NodeURLs))

	for i, nodeURL := range config.NodeURLs {
		// Create websocket client
		wsConfig := conf.GenerateWSConfig(nodeURL, &config.WebSocket)
		wsconn, err := wsclient.New(context.Background(), wsConfig, nil, nil)
		if err != nil {
			log.Error("Could not create websocket connection: %s", err)
		}
		wsconns[i] = wsconn
	}

	wsUUID := *fftypes.NewUUID()

	return &perfRunner{
		bfr:             make(chan int, config.Workers),
		cfg:             config,
		ctx:             context.Background(),
		endTime:         time.Now().Unix() + int64(config.Length.Seconds()),
		poolName:        poolName,
		shutdown:        make(chan bool),
		tagPrefix:       fmt.Sprintf("perf_%s", wsUUID.String()),
		msgTimeMap:      make(map[string]time.Time),
		wsconns:         wsconns,
		wsReceivers:     wsReceivers,
		wsUUID:          wsUUID,
		nodeURLs:        config.NodeURLs,
		subscriptionMap: make(map[string]SubscriptionInfo),
	}
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.nodeURLs[0])

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

	for _, nodeURL := range pr.nodeURLs {
		// Create contract sub and listener, if needed
		var listenerID string
		if containsTargetCmd(pr.cfg.Cmds, conf.PerfCmdCustomEthereumContract) {
			listenerID, err = pr.createEthereumContractListener(nodeURL)
			if err != nil {
				return err
			}
			subID, err := pr.createContractsSub(nodeURL, listenerID)
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Job:     conf.PerfCmdCustomEthereumContract,
			}
		}

		if containsTargetCmd(pr.cfg.Cmds, conf.PerfCmdCustomFabricContract) {
			listenerID, err = pr.createFabricContractListener(nodeURL)
			if err != nil {
				return err
			}
			subID, err := pr.createContractsSub(nodeURL, listenerID)
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Job:     conf.PerfCmdCustomFabricContract,
			}
		}

		// Create subscription for message confirmations
		subID, err := pr.createMsgConfirmSub(nodeURL, pr.tagPrefix, fmt.Sprintf("^%s_", pr.tagPrefix))
		if err != nil {
			return err
		}
		pr.subscriptionMap[subID] = SubscriptionInfo{
			NodeURL: nodeURL,
			Job:     conf.PerfCmdBroadcast,
		}
		// Create subscription for blob message confirmations
		subID, err = pr.createMsgConfirmSub(nodeURL, fmt.Sprintf("blob_%s", pr.tagPrefix), fmt.Sprintf("^blob_%s_", pr.tagPrefix))
		if err != nil {
			return err
		}
		pr.subscriptionMap[subID] = SubscriptionInfo{
			NodeURL: nodeURL,
			Job:     conf.PerfBlobBroadcast,
		}
	}

	// Open websocket clients for all subscriptions
	for _, wsconn := range pr.wsconns {
		err = pr.openWsClient(wsconn)
		if err != nil {
			return err
		}
		go pr.eventLoop(wsconn)
	}

	for id := 0; id < pr.cfg.Workers; id++ {
		ptr := id % len(pr.cfg.Cmds)

		switch pr.cfg.Cmds[ptr] {
		case conf.PerfCmdBroadcast:
			go pr.RunBroadcast(pr.client.BaseURL, id)
		case conf.PerfCmdPrivateMsg:
			go pr.RunPrivateMessage(pr.client.BaseURL, id)
		case conf.PerfCmdTokenMint:
			go pr.RunTokenMint(pr.client.BaseURL, id)
		case conf.PerfCmdCustomEthereumContract:
			go pr.RunCustomEthereumContract(pr.client.BaseURL, id)
		case conf.PerfCmdCustomFabricContract:
			go pr.RunCustomFabricContract(pr.client.BaseURL, id)
		case conf.PerfBlobBroadcast:
			go pr.RunBlobBroadcast(pr.client.BaseURL, id)
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

func (pr *perfRunner) eventLoop(wsconn wsclient.WSClient) (err error) {
	log.Infoln("Event loop started...")
	for {
		select {
		// Wait to receive websocket event
		case msgBytes, ok := <-wsconn.Receive():
			if !ok {
				log.Errorf("Error receiving websocket")
				return
			}
			// Handle websocket event
			var event fftypes.EventDelivery
			json.Unmarshal(msgBytes, &event)

			workerID := -1

			switch event.Type {
			case fftypes.EventTypeBlockchainEventReceived:
				if event.BlockchainEvent == nil {
					log.Errorf("\nBlockchain event not found --- Event ID: %s\n\t%d --- Ref: %s", event.ID.String(), event.Reference)
					return fmt.Errorf("blockchain event not found for event: %s", event.ID)
				}
				var value string
				switch event.BlockchainEvent.Source {
				case "ethereum":
					value = event.BlockchainEvent.Output.GetString("value")
				case "fabric":
					value = event.BlockchainEvent.Output.GetString("name")
				}
				workerID, err = strconv.Atoi(value)
				if err != nil {
					log.Errorf("Could not parse event value: %s", err)
					b, _ := json.Marshal(&event)
					log.Infof("Full event: %s", b)
				} else {
					log.Infof("\n\t%d - Received \n\t%d --- Event ID: %s\n\t%d --- Ref: %s", workerID, workerID, event.ID.String(), workerID, event.Reference)
				}
			default:
				workerIDFromTag := ""
				subInfo, ok := pr.subscriptionMap[event.Subscription.ID.String()]
				if !ok {
					return fmt.Errorf("received an event on unknown subscription: %s", event.Subscription.ID)
				}
				switch subInfo.Job {
				case conf.PerfBlobBroadcast:
					workerIDFromTag = strings.ReplaceAll(event.Message.Header.Tag, fmt.Sprintf("blob_%s_", pr.tagPrefix), "")
				default:
					workerIDFromTag = strings.ReplaceAll(event.Message.Header.Tag, pr.tagPrefix+"_", "")
				}

				workerID, err = strconv.Atoi(workerIDFromTag)
				if err != nil {
					log.Errorf("Could not parse message tag: %s", err)
					b, _ := json.Marshal(&event)
					log.Infof("Full event: %s", b)
				}

				pr.deleteMsgTime(event.Message.Header.ID.String())
				log.Infof("\n\t%d - Received \n\t%d --- Event ID: %s\n\t%d --- Message ID: %s\n\t%d --- Data ID: %s", workerID, workerID, event.ID.String(), workerID, event.Message.Header.ID.String(), workerID, event.Message.Data[0].ID)
				if subInfo.Job == conf.PerfBlobBroadcast {
					pr.downloadAndVerifyBlob(subInfo.NodeURL, event.Message.Data[0].ID.String(), *event.Message.Data[0].Hash)
				}
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
			wsconn.Send(pr.ctx, ackJSON)
			// Release worker so it can continue to its next task
			if workerID >= 0 {
				pr.wsReceivers[workerID] <- true
			}
		case <-pr.ctx.Done():
			log.Errorf("Run loop exiting (context cancelled)")
			return
		}
	}
}

func (pr *perfRunner) sendAndWait(req *resty.Request, nodeURL, ep string, id int, action string) error {
	for {
		select {
		case <-pr.bfr:
			// Worker sends its task
			res, err := req.Post(fmt.Sprintf("%s/api/v1/namespaces/default/%s", nodeURL, ep))
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
			case conf.PerfCmdCustomEthereumContract.String(), conf.PerfCmdCustomFabricContract.String():
				json.Unmarshal(res.Body(), &contractRes)
				log.Infof("%d --> Invoked contract: %s", id, contractRes.ID)
			case conf.PerfBlobBroadcast.String():
				json.Unmarshal(res.Body(), &msgRes)
				pr.updateMsgTime(msgRes.Header.ID.String())
				log.Infof("%d --> Broadcasted blob: %s", id, msgRes.Header.ID)
			}
			// Wait for worker to confirm the message before proceeding to next task
			for i := 0; i < len(pr.nodeURLs); i++ {
				<-pr.wsReceivers[id]
			}
			log.Infof("%d <-- %s Finished", id, action)
		case <-pr.shutdown:
			return nil
		}
	}
}

func (pr *perfRunner) createMsgConfirmSub(nodeURL, name, tag string) (subID string, err error) {
	var sub fftypes.Subscription
	var readAhead uint16 = uint16(len(pr.wsReceivers))
	subPayload := fftypes.Subscription{
		SubscriptionRef: fftypes.SubscriptionRef{
			Name:      name,
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
				Tag: tag,
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
		SetResult(&sub).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/subscriptions", nodeURL))
	if err != nil {
		log.Errorf("Could not create subscription: %s", err)
		return "", err
	}

	log.Infof("Created subscription on %s: %s", nodeURL, pr.tagPrefix)

	return sub.ID.String(), nil
}

func (pr *perfRunner) openWsClient(wsconn wsclient.WSClient) (err error) {
	wsconn.Connect()
	if err := pr.startSubscription(wsconn, pr.tagPrefix); err != nil {
		return err
	}
	if err := pr.startSubscription(wsconn, fmt.Sprintf("blob_%s", pr.tagPrefix)); err != nil {
		return err
	}
	if err := pr.startSubscription(wsconn, fmt.Sprintf("contracts_%s", pr.tagPrefix)); err != nil {
		return err
	}
	return nil
}

func (pr *perfRunner) startSubscription(wsconn wsclient.WSClient, name string) (err error) {
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
	err = wsconn.Send(pr.ctx, start)
	if err != nil {
		log.Errorf("Issuing opening websocket client: %s", err)
		return err
	}
	log.Infof(`Receiving Events subscription: "%s"`, name)
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

func (pr *perfRunner) createEthereumContractListener(nodeURL string) (string, error) {
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
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/contracts/listeners", nodeURL))
	if err != nil {
		return "", err
	}
	var responseBody map[string]interface{}
	err = json.Unmarshal(res.Body(), &responseBody)
	if err != nil {
		return "", err
	}
	id := responseBody["id"].(string)
	log.Infof("Created contract listener on %s: %s", nodeURL, id)
	return id, nil
}

func (pr *perfRunner) createFabricContractListener(nodeURL string) (string, error) {
	subPayload := fmt.Sprintf(`{
		"location": {
			"channel": "%s",
			"chaincode": "%s"
		},
		"event": {
			"name": "AssetCreated"
		}
	}`, pr.cfg.ContractOptions.Channel, pr.cfg.ContractOptions.Chaincode)

	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/contracts/listeners", nodeURL))
	if err != nil {
		return "", err
	}
	var responseBody map[string]interface{}
	err = json.Unmarshal(res.Body(), &responseBody)
	if err != nil {
		return "", err
	}
	id := responseBody["id"].(string)
	log.Infof("Created contract listener on %s: %s", nodeURL, id)
	return id, nil
}

func (pr *perfRunner) createContractsSub(nodeURL, listenerID string) (subID string, err error) {
	var sub fftypes.Subscription
	subPayload := fftypes.Subscription{
		SubscriptionRef: fftypes.SubscriptionRef{
			Name:      fmt.Sprintf("contracts_%s", pr.tagPrefix),
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
		SetResult(&sub).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/subscriptions", nodeURL))
	if err != nil {
		log.Errorf("Could not create subscription on %s: %s", nodeURL, err)
		return "", err
	}

	log.Infof("Created contracts subscription on %s: %s", nodeURL, fmt.Sprintf("contracts_%s", pr.tagPrefix))

	return sub.ID.String(), nil
}
