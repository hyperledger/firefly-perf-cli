// Copyright © 2022 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

var METRICS_NAMESPACE = "ffperf"
var METRICS_SUBSYSTEM = "runner"

var deliquentMsgsCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "deliquent_msgs_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var perfTestDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: METRICS_NAMESPACE,
	Subsystem: METRICS_SUBSYSTEM,
	Name:      "perf_test_duration_seconds",
	Buckets:   []float64{1.0, 2.0, 5.0, 10.0, 30.0},
}, []string{"test"})

func init() {
	prometheus.Register(deliquentMsgsCounter)
	prometheus.Register(perfTestDurationHistogram)
}

type PerfRunner interface {
	Init() error
	Start() error
}

type TrackingIDType string

const (
	TrackingIDTypeMessageID    TrackingIDType = "Message ID"
	TrackingIDTypeTransferID   TrackingIDType = "Token Transfer ID"
	TrackingIDTypeWorkerNumber                = "Worker ID"
)

type TestCase interface {
	WorkerID() int
	RunOnce() (trackingID string, err error)
	IDType() TrackingIDType
	Name() string
}

type inflightTest struct {
	time     time.Time
	testCase TestCase
}

type perfRunner struct {
	bfr             chan int
	cfg             *conf.PerfRunnerConfig
	client          *resty.Client
	ctx             context.Context
	shutdown        context.CancelFunc
	endTime         int64
	msgTimeMap      map[string]*inflightTest
	poolName        string
	tagPrefix       string
	wsconns         map[string]wsclient.WSClient
	wsReceivers     []chan string
	wsUUID          fftypes.UUID
	nodes           map[string]conf.Node
	subscriptionMap map[string]SubscriptionInfo
	daemon          bool
	sender          string
}

type SubscriptionInfo struct {
	NodeURL string
	Job     fftypes.FFEnum
}

func New(config *conf.PerfRunnerConfig) PerfRunner {
	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())

	// Create channel based dispatch for workers
	var wsReceivers []chan string
	for i := 0; i < config.Workers; i++ {
		wsReceivers = append(wsReceivers, make(chan string))
	}

	wsUUID := *fftypes.NewUUID()
	ctx, cancel := context.WithCancel(context.Background())

	pr := &perfRunner{
		bfr:             make(chan int, config.Workers),
		cfg:             config,
		ctx:             ctx,
		shutdown:        cancel,
		endTime:         time.Now().Unix() + int64(config.Length.Seconds()),
		poolName:        poolName,
		tagPrefix:       fmt.Sprintf("perf_%s", wsUUID.String()),
		msgTimeMap:      make(map[string]*inflightTest),
		wsReceivers:     wsReceivers,
		wsUUID:          wsUUID,
		nodes:           config.Nodes,
		subscriptionMap: make(map[string]SubscriptionInfo),
		daemon:          config.Daemon,
		sender:          config.Sender,
	}

	wsconns := make(map[string]wsclient.WSClient, len(config.Nodes))

	for did, node := range config.Nodes {
		// Create websocket client
		wsConfig := conf.GenerateWSConfig(node.URL, &config.WebSocket)
		wsconn, err := wsclient.New(pr.ctx, wsConfig, nil, pr.startSubscriptions)
		if err != nil {
			log.Error("Could not create websocket connection: %s", err)
		}
		wsconns[did] = wsconn
	}

	pr.wsconns = wsconns
	return pr
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.nodes[pr.sender].URL)

	if pr.nodes[pr.sender].Username != "" && pr.nodes[pr.sender].Password != "" {
		pr.client.SetBasicAuth(pr.nodes[pr.sender].Username, pr.nodes[pr.sender].Password)
	}

	return nil
}

func (pr *perfRunner) Start() (err error) {
	// Create token pool, if needed
	if containsTargetCmd(pr.cfg.Tests, conf.PerfTestTokenMint) {
		err = pr.CreateTokenPool()
		if err != nil {
			return err
		}
	}

	for _, node := range pr.nodes {
		// Create contract sub and listener, if needed
		var listenerID string
		if containsTargetCmd(pr.cfg.Tests, conf.PerfTestCustomEthereumContract) {
			listenerID, err = pr.createEthereumContractListener(node.URL)
			if err != nil {
				return err
			}
			subID, err := pr.createContractsSub(node.URL, listenerID)
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: node.URL,
				Job:     conf.PerfTestCustomEthereumContract,
			}
		}

		if containsTargetCmd(pr.cfg.Tests, conf.PerfTestCustomFabricContract) {
			listenerID, err = pr.createFabricContractListener(node.URL)
			if err != nil {
				return err
			}
			subID, err := pr.createContractsSub(node.URL, listenerID)

			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: node.URL,
				Job:     conf.PerfTestCustomFabricContract,
			}
		}

		// Create subscription for message confirmations
		subID, err := pr.createMsgConfirmSub(node.URL, pr.tagPrefix, fmt.Sprintf("^%s_", pr.tagPrefix))
		if err != nil {
			return err
		}
		pr.subscriptionMap[subID] = SubscriptionInfo{
			NodeURL: node.URL,
			Job:     conf.PerfTestBroadcast,
		}
		// Create subscription for blob message confirmations
		subID, err = pr.createMsgConfirmSub(node.URL, fmt.Sprintf("blob_%s", pr.tagPrefix), fmt.Sprintf("^blob_%s_", pr.tagPrefix))
		if err != nil {
			return err
		}
		pr.subscriptionMap[subID] = SubscriptionInfo{
			NodeURL: node.URL,
			Job:     conf.PerfTestBlobBroadcast,
		}
	}

	// Open websocket clients for all subscriptions
	for nodeURL, wsconn := range pr.wsconns {
		err = pr.openWsClient(wsconn)
		if err != nil {
			return err
		}
		go pr.eventLoop(nodeURL, wsconn)
	}

	for id := 0; id < pr.cfg.Workers; id++ {
		ptr := id % len(pr.cfg.Tests)
		var tc TestCase

		switch pr.cfg.Tests[ptr] {
		case conf.PerfTestBroadcast:
			tc = newBroadcastTestWorker(pr, id)
		case conf.PerfTestPrivateMsg:
			tc = newPrivateTestWorker(pr, id)
		case conf.PerfTestTokenMint:
			tc = newTokenMintTestWorker(pr, id)
		case conf.PerfTestCustomEthereumContract:
			tc = newCustomEthereumTestWorker(pr, id)
		case conf.PerfTestCustomFabricContract:
			tc = newCustomFabricTestWorker(pr, id)
		case conf.PerfTestBlobBroadcast:
			tc = newBlobBroadcastTestWorker(pr, id)
		case conf.PerfTestBlobPrivateMsg:
			tc = newBlobPrivateTestWorker(pr, id)
		}

		go func() {
			err := pr.runLoop(tc)
			if err != nil {
				log.Errorf("Worker %d failed: %s", tc.WorkerID(), err)
			}
		}()
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	signal.Notify(signalCh, os.Kill)
	signal.Notify(signalCh, syscall.SIGTERM)
	signal.Notify(signalCh, syscall.SIGQUIT)
	signal.Notify(signalCh, syscall.SIGKILL)

	i := 0
	lastCheckedTime := time.Now()

perfLoop:
	for pr.daemon || time.Now().Unix() < pr.endTime {
		timeout := time.After(60 * time.Second)

		select {
		case <-signalCh:
			break perfLoop
		case pr.bfr <- i:
			i++
			if time.Since(lastCheckedTime).Seconds() > 60 {
				if pr.detectDeliquentMsgs() && pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
					break perfLoop
				}
				lastCheckedTime = time.Now()
			}
			break
		case <-timeout:
			if pr.detectDeliquentMsgs() && pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
				break perfLoop
			}
			lastCheckedTime = time.Now()
			break
		}
	}

	log.Info("Perf tests stopping...")
	log.Warn("Shutting down all workers...")
	log.Warn("Stopping event loops...")
	pr.shutdown()

	// we sleep on shutdown / completion to allow for Prometheus metrics to be scraped one final time
	// about 10s into our sleep all workers should be completed, so we check for delinquent messages
	// one last time so metrics are up-to-date
	log.Warn("Runner stopping in 30s")
	time.Sleep(10 * time.Second)
	pr.detectDeliquentMsgs()
	time.Sleep(20 * time.Second)

	return nil
}

func (pr *perfRunner) eventLoop(nodeURL string, wsconn wsclient.WSClient) (err error) {
	log.Infof("Event loop started for %s...", nodeURL)
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
					log.Infof("\n\t%d - Received from %s\n\t%d --- Event ID: %s\n\t%d --- Ref: %s", workerID, wsconn.URL(), workerID, event.ID.String(), workerID, event.Reference)
				}
			default:
				workerIDFromTag := ""
				subInfo, ok := pr.subscriptionMap[event.Subscription.ID.String()]
				if !ok {
					return fmt.Errorf("received an event on unknown subscription: %s", event.Subscription.ID)
				}
				switch subInfo.Job {
				case conf.PerfTestBlobBroadcast, conf.PerfTestBlobPrivateMsg:
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

				log.Infof("\n\t%d - Received %s \n\t%d --- Event ID: %s\n\t%d --- Message ID: %s\n\t%d --- Data ID: %s", workerID, wsconn.URL(), workerID, event.ID.String(), workerID, event.Message.Header.ID.String(), workerID, event.Message.Data[0].ID)
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
				pr.wsReceivers[workerID] <- nodeURL
			}
		case <-pr.ctx.Done():
			log.Errorf("Run loop exiting (context cancelled)")
			return
		}
	}
}

func (pr *perfRunner) runLoop(tc TestCase) error {
	idType := tc.IDType()
	testName := tc.Name()
	workerID := tc.WorkerID()
	loop := 0
	for {
		select {
		case <-pr.bfr:
			// Worker sends its task
			hist, histErr := perfTestDurationHistogram.GetMetricWith(prometheus.Labels{
				"test": testName,
			})

			if histErr != nil {
				log.Errorf("Error retreiving histogram: %s", histErr)
			}
			startTime := time.Now()
			trackingID, err := tc.RunOnce()

			if err != nil {
				return err
			}

			pr.markTestInFlight(tc, trackingID)
			log.Infof("%d --> %s Sent %s: %s", workerID, testName, idType, trackingID)

			confirmations := len(pr.nodes)
			if testName == conf.PerfTestBlobPrivateMsg.String() || testName == conf.PerfTestPrivateMsg.String() {
				confirmations = 2
			}

			// Wait for worker to confirm the message before proceeding to next task
			for i := 0; i < confirmations; i++ {
				select {
				case <-pr.ctx.Done():
					return nil
				case <-pr.wsReceivers[workerID]:
					break
				}
			}
			log.Infof("%d <-- %s Finished (loop=%d)", workerID, testName, loop)
			pr.markTestComplete(trackingID)
			if histErr == nil {
				hist.Observe(time.Since(startTime).Seconds())
			}
			loop++
		case <-pr.ctx.Done():
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
	if err := wsconn.Connect(); err != nil {
		return err
	}
	return nil
}

func (pr *perfRunner) startSubscriptions(ctx context.Context, wsconn wsclient.WSClient) (err error) {
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

func (pr *perfRunner) detectDeliquentMsgs() bool {
	mutex.Lock()
	delinquentMsgs := make(map[string]time.Time)
	for trackingID, inflight := range pr.msgTimeMap {
		if time.Since(inflight.time).Seconds() > 60 {
			delinquentMsgs[trackingID] = inflight.time
		}
	}
	mutex.Unlock()

	dw, err := json.MarshalIndent(delinquentMsgs, "", "  ")
	if err != nil {
		log.Errorf("Error printing delinquent messages: %s", err)
		return len(delinquentMsgs) > 0
	}

	log.Warnf("Delinquent Messages:\n%s", string(dw))

	return len(delinquentMsgs) > 0
}

func (pr *perfRunner) markTestInFlight(tc TestCase, trackingID string) {
	mutex.Lock()
	if len(trackingID) > 0 {
		pr.msgTimeMap[trackingID] = &inflightTest{
			testCase: tc,
			time:     time.Now(),
		}
	}
	mutex.Unlock()
}

func (pr *perfRunner) markTestComplete(trackingID string) {
	mutex.Lock()
	delete(pr.msgTimeMap, trackingID)
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
		},
		"topic": "%s"
	}`, pr.cfg.ContractOptions.Address, fftypes.NewUUID())

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
		},
		"topic": "%s"
	}`, pr.cfg.ContractOptions.Channel, pr.cfg.ContractOptions.Chaincode, fftypes.NewUUID())

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

func (pr *perfRunner) IsDaemon() bool {
	return pr.daemon
}
