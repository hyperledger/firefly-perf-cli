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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly-common/pkg/wsclient"
	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/core"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var mutex = &sync.Mutex{}
var limiter *rate.Limiter
var TRANSPORT_TYPE = "websockets"
var wsReadAhead = uint16(50)

var METRICS_NAMESPACE = "ffperf"
var METRICS_SUBSYSTEM = "runner"

var totalActionsCounter = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "actions_submitted_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var sentMintsCounter = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "sent_mints_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var sentMintErrorCounter = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "sent_mint_errors_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var mintBalanceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "mint_token_balance",
	Subsystem: METRICS_SUBSYSTEM,
})

var receivedEventsCounter = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "received_events_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var incompleteEventsCounter = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "incomplete_events_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var delinquentMsgsCounter = prometheus.NewGauge(prometheus.GaugeOpts{
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

func Init() {
	prometheus.Register(delinquentMsgsCounter)
	prometheus.Register(sentMintsCounter)
	prometheus.Register(sentMintErrorCounter)
	prometheus.Register(mintBalanceGauge)
	prometheus.Register(receivedEventsCounter)
	prometheus.Register(incompleteEventsCounter)
	prometheus.Register(totalActionsCounter)
	prometheus.Register(perfTestDurationHistogram)
}

func getMetricVal(collector prometheus.Collector) float64 {
	collectorChannel := make(chan prometheus.Metric, 1)
	collector.Collect(collectorChannel)
	metric := dto.Metric{}
	err := (<-collectorChannel).Write(&metric)
	if err != nil {
		log.Error("Error writing metric: %s", err)
	}
	if metric.Counter != nil {
		return *metric.Counter.Value
	} else if metric.Gauge != nil {
		return *metric.Gauge.Value
	}
	return 0
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
	ActionsPerLoop() int
}

type inflightTest struct {
	time     time.Time
	testCase TestCase
}

var mintStartingBalance int

type perfRunner struct {
	bfr                     chan int
	cfg                     *conf.RunnerConfig
	client                  *resty.Client
	ctx                     context.Context
	shutdown                context.CancelFunc
	stopping                bool
	startTime               int64
	endTime                 int64
	startRampTime           int64
	endRampTime             int64
	msgTimeMap              map[string]*inflightTest
	rampSummary             int64
	totalSummary            int64
	poolName                string
	poolConnectorName       string
	tagPrefix               string
	wsconns                 []wsclient.WSClient
	wsReceivers             []chan string
	wsUUID                  fftypes.UUID
	nodeURLs                []string
	subscriptionMap         map[string]SubscriptionInfo
	listenerIDsForNodes     map[string][]string
	subscriptionIDsForNodes map[string][]string
	daemon                  bool
	sender                  string
	totalWorkers            int
}

type SubscriptionInfo struct {
	NodeURL string
	Name    string
	Job     fftypes.FFEnum
}

func New(config *conf.RunnerConfig) PerfRunner {
	if config.LogLevel != "" {
		if level, err := log.ParseLevel(config.LogLevel); err == nil {
			log.SetLevel(level)
		}
	}

	poolName := fmt.Sprintf("pool-%s", fftypes.NewUUID())

	totalWorkers := 0
	for _, test := range config.Tests {
		totalWorkers += test.Workers
	}

	// Create channel based dispatch for workers
	var wsReceivers []chan string
	for i := 0; i < totalWorkers; i++ {
		wsReceivers = append(wsReceivers, make(chan string))
	}

	wsUUID := *fftypes.NewUUID()
	ctx, cancel := context.WithCancel(context.Background())

	startRampTime := time.Now().Unix()
	endRampTime := time.Now().Unix() + int64(config.RampLength.Seconds())
	startTime := endRampTime
	endTime := startTime + int64(config.Length.Seconds())

	pr := &perfRunner{
		bfr:               make(chan int, totalWorkers),
		cfg:               config,
		ctx:               ctx,
		shutdown:          cancel,
		startRampTime:     startRampTime,
		endRampTime:       endRampTime,
		startTime:         startTime,
		endTime:           endTime,
		poolName:          poolName,
		poolConnectorName: config.TokenOptions.TokenPoolConnectorName,
		tagPrefix:         fmt.Sprintf("perf_%s", wsUUID.String()),
		msgTimeMap:        make(map[string]*inflightTest),
		totalSummary:      0,
		wsReceivers:       wsReceivers,
		wsUUID:            wsUUID,
		nodeURLs:          config.NodeURLs,
		subscriptionMap:   make(map[string]SubscriptionInfo),
		daemon:            config.Daemon,
		sender:            config.SenderURL,
		totalWorkers:      totalWorkers,
	}

	wsconns := make([]wsclient.WSClient, len(config.NodeURLs))

	for i, nodeURL := range config.NodeURLs {
		// Create websocket client
		wsConfig := conf.GenerateWSConfig(nodeURL, &config.WebSocket)
		wsconn, err := wsclient.New(context.Background(), wsConfig, nil, pr.startSubscriptions)
		if err != nil {
			log.Errorf("Could not create websocket connection: %s", err)
		}
		wsconns[i] = wsconn
	}

	pr.wsconns = wsconns
	return pr
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.sender)
	pr.client.SetBasicAuth(pr.cfg.WebSocket.AuthUsername, pr.cfg.WebSocket.AuthPassword)
	// Set request retry with backoff
	pr.client.
		SetRetryCount(10).
		// You can override initial retry wait time.
		// Default is 100 milliseconds.
		SetRetryWaitTime(1 * time.Second).
		// MaxWaitTime can be overridden as well.
		// Default is 2 seconds.
		SetRetryMaxWaitTime(30 * time.Second).
		AddRetryCondition(
			// RetryConditionFunc type is for retry condition function
			// input: non-nil Response OR request execution error
			func(r *resty.Response, err error) bool {
				if r.IsError() || err != nil {
					if r.StatusCode() == 409 {
						// Do not retry on duplicates, because FireFly should already be processing the transaction
						return false
					}
					// Retry for all other errors
					log.Warnf("Retrying HTTP request. Status: '%v' Response: '%s' Error: '%v'", r.Status(), r.Body(), r.Error())
					return true
				}
				return false
			},
		)
	return nil
}

func (pr *perfRunner) Start() (err error) {
	// Create token pool, if needed
	if containsTargetTest(pr.cfg.Tests, conf.PerfTestTokenMint) {
		if pr.cfg.TokenOptions.ExistingPoolName == "" {
			err = pr.CreateTokenPool()
			if err != nil {
				return err
			}
		} else {
			pr.poolName = pr.cfg.TokenOptions.ExistingPoolName
		}
	}

	log.Infof("Running test:\n%+v", pr.cfg)
	pr.listenerIDsForNodes = make(map[string][]string)
	pr.subscriptionIDsForNodes = make(map[string][]string)
	for _, nodeURL := range pr.nodeURLs {
		pr.listenerIDsForNodes[nodeURL] = []string{}
		pr.subscriptionIDsForNodes[nodeURL] = []string{}

		if containsTargetTest(pr.cfg.Tests, conf.PerfTestTokenMint) {
			subID, subName, err := pr.createTokenMintSub(nodeURL)
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Name:    subName,
				Job:     conf.PerfTestTokenMint,
			}

			// Create subscription for message confirmations if supportsData == true
			if *pr.cfg.TokenOptions.SupportsData {
				log.Infof("Creating message subscription for data in token mints")
				subID, subName, err = pr.createMsgConfirmSub(nodeURL, pr.tagPrefix, fmt.Sprintf("^%s_", pr.tagPrefix))
				if err != nil {
					return err
				}
				pr.subscriptionMap[subID] = SubscriptionInfo{
					NodeURL: nodeURL,
					Name:    subName,
					Job:     conf.PerfTestBroadcast,
				}
			}

			if pr.cfg.TokenOptions.MaxTokenBalanceWait.Seconds() > 0 {
				mintStartingBalance, err = pr.getMintRecipientBalance()

				if err != nil {
					return err
				}
			}
		}

		// Create contract sub and listener, if needed
		var listenerID string
		if containsTargetTest(pr.cfg.Tests, conf.PerfTestCustomEthereumContract) {
			listenerID, err = pr.createEthereumContractListener(nodeURL)
			if err != nil {
				return err
			}
			subID, subName, err := pr.createContractsSub(nodeURL, listenerID)
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Name:    subName,
				Job:     conf.PerfTestCustomEthereumContract,
			}
		}

		if containsTargetTest(pr.cfg.Tests, conf.PerfTestCustomFabricContract) {
			listenerID, err = pr.createFabricContractListener(nodeURL)
			if err != nil {
				return err
			}
			subID, subName, err := pr.createContractsSub(nodeURL, listenerID)

			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Name:    subName,
				Job:     conf.PerfTestCustomFabricContract,
			}
		}

		if containsTargetTest(pr.cfg.Tests, conf.PerfTestBroadcast) {
			// Create subscription for message confirmations
			subID, subName, err := pr.createMsgConfirmSub(nodeURL, pr.tagPrefix, fmt.Sprintf("^%s_", pr.tagPrefix))
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Name:    subName,
				Job:     conf.PerfTestBroadcast,
			}
		}

		if containsTargetTest(pr.cfg.Tests, conf.PerfTestBlobBroadcast) {
			// Create subscription for blob message confirmations
			subID, subName, err := pr.createMsgConfirmSub(nodeURL, fmt.Sprintf("blob_%s", pr.tagPrefix), fmt.Sprintf("^blob_%s_", pr.tagPrefix))
			if err != nil {
				return err
			}
			pr.subscriptionMap[subID] = SubscriptionInfo{
				NodeURL: nodeURL,
				Name:    subName,
				Job:     conf.PerfTestBlobBroadcast,
			}
		}
	}

	// Open websocket clients for all subscriptions
	for i, wsconn := range pr.wsconns {
		err = pr.openWsClient(wsconn)
		if err != nil {
			return err
		}
		go pr.eventLoop(pr.nodeURLs[i], wsconn)
	}

	id := 0
	for _, test := range pr.cfg.Tests {
		log.Infof("Starting %d workers for case \"%s\"", test.Workers, test.Name)
		for iWorker := 0; iWorker < test.Workers; iWorker++ {
			var tc TestCase

			switch test.Name {
			case conf.PerfTestBroadcast:
				tc = newBroadcastTestWorker(pr, id, test.ActionsPerLoop)
			case conf.PerfTestPrivateMsg:
				tc = newPrivateTestWorker(pr, id, test.ActionsPerLoop)
			case conf.PerfTestTokenMint:
				tc = newTokenMintTestWorker(pr, id, test.ActionsPerLoop)
			case conf.PerfTestCustomEthereumContract:
				tc = newCustomEthereumTestWorker(pr, id, test.ActionsPerLoop)
			case conf.PerfTestCustomFabricContract:
				tc = newCustomFabricTestWorker(pr, id, test.ActionsPerLoop)
			case conf.PerfTestBlobBroadcast:
				tc = newBlobBroadcastTestWorker(pr, id, test.ActionsPerLoop)
			case conf.PerfTestBlobPrivateMsg:
				tc = newBlobPrivateTestWorker(pr, id, test.ActionsPerLoop)
			default:
				return fmt.Errorf("Unknown test case '%s'", test.Name)
			}

			delayPerWorker := pr.cfg.RampLength / time.Duration(test.Workers)

			go func(i int) {
				// Delay the start of the next worker by (ramp time) / (number of workers)
				if delayPerWorker > 0 {
					time.Sleep(delayPerWorker * time.Duration(i))
					log.Infof("Ramping up. Starting next worker after waiting %v", delayPerWorker)
				}
				err := pr.runLoop(tc)
				if err != nil {
					log.Errorf("Worker %d failed: %s", tc.WorkerID(), err)
				}
			}(iWorker)
			id++
		}
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

		// If we've been given a maximum number of actions to perform, check if we're done
		if pr.cfg.MaxActions > 0 && int64(getMetricVal(totalActionsCounter)) >= pr.cfg.MaxActions {
			break perfLoop
		}

		select {
		case <-signalCh:
			break perfLoop
		case pr.bfr <- i:
			i++
			if time.Since(lastCheckedTime).Seconds() > pr.cfg.MaxTimePerAction.Seconds() {
				if pr.detectDelinquentMsgs() && pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
					break perfLoop
				}
				lastCheckedTime = time.Now()
			}
			break
		case <-timeout:
			if pr.detectDelinquentMsgs() && pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
				break perfLoop
			}
			lastCheckedTime = time.Now()
			break
		}
	}

	// If configured, check that the balance of the mint recipient address is correct
	if pr.detectDelinquentBalance() {
		if pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
			log.Panic(fmt.Errorf("Token mint recipient balance didn't reach the expected value in the allowed time"))
		}
	}

	pr.stopping = true
	measuredActions := pr.totalSummary
	measuredTime := time.Since(time.Unix(pr.startTime, 0)).Seconds()
	measuredTps := pr.calculateCurrentTps(true)

	// we sleep on shutdown / completion to allow for Prometheus metrics to be scraped one final time
	// After 30 seconds workers should be completed, so we check for delinquent messages
	// one last time so metrics are up-to-date
	log.Warn("Runner stopping in 30s")
	time.Sleep(30 * time.Second)
	pr.detectDelinquentMsgs()

	log.Info("Cleaning up")

	pr.cleanup()

	log.Info("Shutdown summary:")
	log.Infof(" - Prometheus metric sent_mints_total        = %f\n", getMetricVal(sentMintsCounter))
	log.Infof(" - Prometheus metric sent_mint_errors_total  = %f\n", getMetricVal(sentMintErrorCounter))
	log.Infof(" - Prometheus metric mint_token_balance      = %f\n", getMetricVal(mintBalanceGauge))
	log.Infof(" - Prometheus metric received_events_total   = %f\n", getMetricVal(receivedEventsCounter))
	log.Infof(" - Prometheus metric incomplete_events_total = %f\n", getMetricVal(incompleteEventsCounter))
	log.Infof(" - Prometheus metric delinquent_msgs_total    = %f\n", getMetricVal(delinquentMsgsCounter))
	log.Infof(" - Prometheus metric actions_submitted_total = %f\n", getMetricVal(totalActionsCounter))
	log.Infof(" - Test duration (secs): %2f", measuredTime)
	log.Infof(" - Measured actions: %d", measuredActions)
	log.Infof(" - Measured actions/sec: %2f", measuredTps)

	return nil
}

func (pr *perfRunner) cleanup() {
	for _, nodeURL := range pr.nodeURLs {
		subIDs := pr.subscriptionIDsForNodes[nodeURL]
		lIDs := pr.listenerIDsForNodes[nodeURL]

		for _, subID := range subIDs {
			err := pr.deleteSubscription(nodeURL, subID)
			if err != nil {
				log.Warnf("failed to delete subscription with ID %s for node URL %s due to %s\n", subID, nodeURL, err.Error())
			} else {
				log.Infof("successfully deleted subscription with ID %s for node URL %s\n", subID, nodeURL)
			}
		}

		for _, lID := range lIDs {
			err := pr.deleteContractListener(nodeURL, lID)
			if err != nil {
				fmt.Printf("failed to delete listener with ID %s for node URL %s due to %s\n", lID, nodeURL, err.Error())
			} else {
				log.Infof("successfully deleted listener with ID %s for node URL %s\n", lID, nodeURL)
			}
		}
	}

}

func (pr *perfRunner) eventLoop(nodeURL string, wsconn wsclient.WSClient) (err error) {
	log.Infof("Event loop started for %s...", nodeURL)
	for {
		log.Trace("blocking until wsconn.Receive or ctx.Done()")
		select {
		// Wait to receive websocket event
		case msgBytes, ok := <-wsconn.Receive():
			if !ok {
				log.Errorf("Error receiving websocket")
				return
			}
			log.Trace("received from websocket")

			receivedEventsCounter.Inc()

			// Handle websocket event
			var event core.EventDelivery
			json.Unmarshal(msgBytes, &event)

			if pr.cfg.LogEvents {
				fmt.Println("Event: ", string(msgBytes))
			}

			workerID := -1

			switch event.Type {
			case core.EventTypeBlockchainEventReceived:
				if event.BlockchainEvent == nil {
					log.Errorf("\nBlockchain event not found --- Event ID: %s\n\t%d --- Ref: %s", event.ID.String(), event.Reference)
					return fmt.Errorf("blockchain event not found for event: %s", event.ID)
				}
				var value string
				switch event.BlockchainEvent.Source {
				case "ethereum":
					value = event.BlockchainEvent.Output.GetString("value")
				case "fabric":
					value = event.BlockchainEvent.Output.GetString("Owner")
				}
				workerID, err = strconv.Atoi(value)
				if err != nil {
					log.Errorf("Could not parse event value: %s", err)
					b, _ := json.Marshal(&event)
					log.Infof("Full event: %s", b)
				} else {
					log.Infof("\n\t%d - Received from %s\n\t%d --- Event ID: %s\n\t%d --- Ref: %s", workerID, wsconn.URL(), workerID, event.ID.String(), workerID, event.Reference)
				}
			case core.EventTypeTransferConfirmed:
				if pr.cfg.TokenOptions.SupportsURI {
					// If there's a URI in the event we'll have put the worker ID there
					uriElements := strings.Split(event.TokenTransfer.URI, "//")
					if len(uriElements) == 2 {
						workerID, err = strconv.Atoi(uriElements[1])
						if err != nil {
							log.Errorf("Could not parse event value: %s", err)
							b, _ := json.Marshal(&event)
							log.Infof("Full event: %s", b)
						} else {
							log.Infof("\n\t%d - Received from %s\n\t%d --- Event ID: %s\n\t%d --- Ref: %s", workerID, wsconn.URL(), workerID, event.ID.String(), workerID, event.Reference)
						}
					} else {
						log.Errorf("No URI in token transfer event: %s")
						b, _ := json.Marshal(&event)
						log.Errorf("Full event: %s", b)

						incompleteEventsCounter.Inc()

						if pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
							log.Panic(fmt.Errorf("Error - no URI found in token_transfer_confirmed event"))
						}
					}
				}
			default:
				workerIDFromTag := ""

				if event.Type.String() == "protocol_error" {
					// Can't do anything but shut down gracefully
					log.Error("Protocol error event - shutting down")
					pr.shutdown()
				} else {
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

					var dataID *fftypes.UUID
					if len(event.Message.Data) > 0 {
						dataID = event.Message.Data[0].ID
					}
					log.Infof("\n\t%d - Received %s \n\t%d --- Event ID: %s\n\t%d --- Message ID: %s\n\t%d --- Data ID: %s", workerID, wsconn.URL(), workerID, event.ID.String(), workerID, event.Message.Header.ID.String(), workerID, dataID)
				}
			}

			// Ack websocket event
			ack := &core.WSAck{
				WSActionBase: core.WSActionBase{
					Type: core.WSClientActionAck,
				},
				ID: event.ID,
				Subscription: &core.SubscriptionRef{
					ID: event.Subscription.ID,
				},
			}
			ackJSON, _ := json.Marshal(ack)
			wsconn.Send(context.Background(), ackJSON)
			pr.recordCompletedAction()
			// Release worker so it can continue to its next task
			if !pr.stopping && !pr.cfg.SkipMintConfirmations {
				if workerID >= 0 {
					pr.wsReceivers[workerID] <- nodeURL
				}
			}
		case <-pr.ctx.Done():
			log.Warnf("Run loop exiting (context cancelled)")
			wsconn.Close()
			return
		}
	}
}

func (pr *perfRunner) allActionsComplete() bool {
	if pr.cfg.MaxActions > 0 && int64(getMetricVal(totalActionsCounter)) >= pr.cfg.MaxActions {
		return true
	}
	return false
}

func (pr *perfRunner) runLoop(tc TestCase) error {
	idType := tc.IDType()
	testName := tc.Name()
	workerID := tc.WorkerID()

	loop := 0
	for {
		select {
		case <-pr.bfr:
			var confirmationsPerAction int
			var actionsCompleted int

			// Worker sends its task
			hist, histErr := perfTestDurationHistogram.GetMetricWith(prometheus.Labels{
				"test": testName,
			})

			if histErr != nil {
				log.Errorf("Error retrieving histogram: %s", histErr)
			}

			startTime := time.Now()
			trackingIDs := make([]string, 0)

			for actionsCompleted = 0; actionsCompleted < tc.ActionsPerLoop(); actionsCompleted++ {

				if pr.allActionsComplete() {
					break
				}

				trackingID, err := tc.RunOnce()

				if err != nil {
					if pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
						return err
					} else {
						log.Errorf("Worker %d error running job (logging but continuing): %s", workerID, err)
						err = nil
						continue
					}
				} else {
					trackingIDs = append(trackingIDs, trackingID)
					pr.markTestInFlight(tc, trackingID)
					log.Infof("%d --> %s Sent %s: %s", workerID, testName, idType, trackingID)
					totalActionsCounter.Inc()
				}
			}

			if testName == conf.PerfTestTokenMint.String() && pr.cfg.SkipMintConfirmations {
				// For minting tests a worker can (if configured) skip waiting for a matching response event
				// before making itself available for the next job
				confirmationsPerAction = 0
			} else {
				confirmationsPerAction = len(pr.nodeURLs)
				if testName == conf.PerfTestBlobPrivateMsg.String() || testName == conf.PerfTestPrivateMsg.String() {
					confirmationsPerAction = 2
				}
			}

			// Wait for worker to confirm the message before proceeding to next task

			for j := 0; j < actionsCompleted; j++ {
				var nextTrackingID string
				for i := 0; i < confirmationsPerAction; i++ {
					select {
					case <-pr.ctx.Done():
						return nil
					case <-pr.wsReceivers[workerID]:
						break
					}
				}
				if len(trackingIDs) > 0 {
					nextTrackingID = trackingIDs[0]
					trackingIDs = trackingIDs[1:]
					pr.stopTrackingRequest(nextTrackingID)
				}
			}
			log.Infof("%d <-- %s Finished (loop=%d)", workerID, testName, loop)

			if histErr == nil {
				hist.Observe(time.Since(startTime).Seconds())
			}
			loop++

			if testName == conf.PerfTestTokenMint.String() &&
				pr.cfg.TokenOptions.MaxTokenBalanceWait.Seconds() > 0 &&
				workerID == 0 && (loop%10 == 0) {
				log.Infof("Worker 0 updating current mint balance")
				// Worker 0 periodically updates the mint balance gauge
				currentBalance, err := pr.getMintRecipientBalance()
				if err != nil {
					log.Warnf("Failed to check token balance: %v", err)
				} else {
					mintBalanceGauge.Set(float64(currentBalance) - float64(mintStartingBalance))
				}
			}
		case <-pr.ctx.Done():
			return nil
		}
	}
}

func (pr *perfRunner) createMsgConfirmSub(nodeURL, name, tag string) (subID string, subName string, err error) {
	var sub core.Subscription
	readAhead := uint16(len(pr.wsReceivers))
	firstEvent := core.SubOptsFirstEventNewest
	subPayload := core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name:      name,
			Namespace: pr.cfg.FFNamespace,
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				ReadAhead:  &readAhead,
				FirstEvent: &firstEvent,
			},
		},
		Filter: core.SubscriptionFilter{
			Events: core.EventTypeMessageConfirmed.String(),
			Message: core.MessageFilter{
				Tag: tag,
			},
		},
		Transport: TRANSPORT_TYPE,
	}
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "subscriptions")
	if err != nil {
		return "", "", err
	}
	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&sub).
		Post(fullPath)
	if err != nil {
		log.Errorf("Could not create subscription: %s", err)
		return "", "", err
	}

	log.Infof("Created subscription on %s: %s", nodeURL, pr.tagPrefix)

	pr.subscriptionIDsForNodes[nodeURL] = append(pr.subscriptionIDsForNodes[nodeURL], sub.ID.String())
	return sub.ID.String(), sub.Name, nil
}

func (pr *perfRunner) openWsClient(wsconn wsclient.WSClient) (err error) {
	if err := wsconn.Connect(); err != nil {
		return err
	}
	return nil
}

func (pr *perfRunner) startSubscriptions(ctx context.Context, wsconn wsclient.WSClient) (err error) {
	for _, nextSub := range pr.subscriptionMap {
		if strings.HasPrefix(nextSub.Name, "mint_") {
			if err := pr.startSubscription(wsconn, fmt.Sprintf("mint_%s", pr.tagPrefix)); err != nil {
				return err
			}
		} else if strings.HasPrefix(nextSub.Name, "blob_") {
			if err := pr.startSubscription(wsconn, fmt.Sprintf("blob_%s", pr.tagPrefix)); err != nil {
				return err
			}
		} else if strings.HasPrefix(nextSub.Name, "contracts_") {
			if err := pr.startSubscription(wsconn, fmt.Sprintf("contracts_%s", pr.tagPrefix)); err != nil {
				return err
			}
		} else if strings.HasPrefix(nextSub.Name, pr.tagPrefix) {
			if err := pr.startSubscription(wsconn, pr.tagPrefix); err != nil {
				return err
			}
		}
	}

	return nil
}

func (pr *perfRunner) startSubscription(wsconn wsclient.WSClient, name string) (err error) {
	log.Infof("Starting subscription %s", name)
	var autoack = false
	startPayload := core.WSStart{
		WSActionBase: core.WSActionBase{
			Type: core.WSClientActionStart,
		},
		AutoAck:   &autoack,
		Name:      name,
		Namespace: pr.cfg.FFNamespace,
	}
	start, _ := json.Marshal(startPayload)
	err = wsconn.Send(pr.ctx, start)
	if err != nil {
		log.Errorf("Issue opening websocket client: %s", err)
		return err
	}
	log.Infof(`Receiving Events subscription: "%s"`, name)
	return nil
}

func containsTargetTest(tests []conf.TestCaseConfig, target fftypes.FFEnum) bool {
	for _, test := range tests {
		if test.Name == target {
			return true
		}
	}

	return false
}

func getFFClient(node string) *resty.Client {
	client := resty.New()
	client.SetBaseURL(node)
	client.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	return client
}

func (pr *perfRunner) detectDelinquentMsgs() bool {
	mutex.Lock()
	delinquentMsgs := make(map[string]time.Time)
	for trackingID, inflight := range pr.msgTimeMap {
		if time.Since(inflight.time).Seconds() > pr.cfg.MaxTimePerAction.Seconds() {
			delinquentMsgs[trackingID] = inflight.time
		}
	}
	mutex.Unlock()

	dw, err := json.MarshalIndent(delinquentMsgs, "", "  ")
	if err != nil {
		log.Errorf("Error printing delinquent messages: %s", err)
		return len(delinquentMsgs) > 0
	}

	if len(delinquentMsgs) > 0 {
		log.Warnf("Delinquent Messages:\n%s", string(dw))
	}

	return len(delinquentMsgs) > 0
}

// A minting test configured to check the balance of the mint recipient address
// Since this could take some time after all mint requests have been submitted we allow until
// the end of the test duration for the balance to reach the expected value.
// Function returns true if the expected balance isn't correct, otherwise false
func (pr *perfRunner) detectDelinquentBalance() bool {

	if containsTargetTest(pr.cfg.Tests, conf.PerfTestTokenMint) && pr.cfg.TokenOptions.MaxTokenBalanceWait.Seconds() > 0 {
		balanceEndTime := time.Now().Unix() + int64(pr.cfg.TokenOptions.MaxTokenBalanceWait.Seconds())
		// var currentBalance int
		fmt.Printf("Waiting for up to %v for the balance of %s to reach the expected value\n", pr.cfg.TokenOptions.MaxTokenBalanceWait, pr.cfg.RecipientAddress)
	balanceCheckLoop:
		for time.Now().Unix() < balanceEndTime {

			timeout := time.After(10 * time.Second)
			select {
			case <-pr.ctx.Done():
				return true
			case <-timeout:
				currentBalance, err := pr.getMintRecipientBalance()
				mintBalanceGauge.Set(float64(currentBalance) - float64(mintStartingBalance))

				if err != nil {
					log.Errorf("Failed to query token balance: %v\n", err)
				} else {
					if getMetricVal(sentMintsCounter) == getMetricVal(mintBalanceGauge) {
						log.Infof("Balance reached\n")
						break balanceCheckLoop
					}
				}
			}
		}

		if getMetricVal(sentMintsCounter) != getMetricVal(mintBalanceGauge) {
			log.Errorf("Token mint recipient balance didn't reach the expected value in the allowed time\n")
			return true
		} else {
			return false
		}
	} else {
		return false
	}
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

func (pr *perfRunner) recordCompletedAction() {
	if pr.ramping() {
		pr.rampSummary++
	} else {
		pr.totalSummary++
	}
	mutex.Lock()
	mutex.Unlock()
	pr.calculateCurrentTps(true)
}

func (pr *perfRunner) stopTrackingRequest(trackingID string) {
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

	var errResponse fftypes.RESTError
	var responseBody map[string]interface{}
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "contracts/listeners")
	if err != nil {
		return "", err
	}
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&responseBody).
		SetError(&errResponse).
		Post(fullPath)
	if err != nil {
		return "", err
	}
	if res.IsError() {
		return "", fmt.Errorf("Failed: %s", errResponse)
	}
	id := responseBody["id"].(string)
	log.Infof("Created contract listener on %s: %s", nodeURL, id)
	pr.listenerIDsForNodes[nodeURL] = append(pr.listenerIDsForNodes[nodeURL], id)

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

	var errResponse fftypes.RESTError
	var responseBody map[string]interface{}
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "contracts/listeners")
	if err != nil {
		return "", err
	}
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&responseBody).
		SetError(&errResponse).
		Post(fullPath)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(res.Body(), &responseBody)
	if err != nil {
		return "", err
	}
	if res.IsError() {
		return "", fmt.Errorf("failed: %s", errResponse)
	}
	id := responseBody["id"].(string)
	log.Infof("Created contract listener on %s: %s", nodeURL, id)
	pr.listenerIDsForNodes[nodeURL] = append(pr.listenerIDsForNodes[nodeURL], id)

	return id, nil
}

func (pr *perfRunner) deleteSubscription(nodeURL string, subscriptionID string) error {
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "subscriptions", subscriptionID)
	if err != nil {
		return err
	}
	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept": "application/json",
		}).
		Delete(fullPath)
	return err
}

func (pr *perfRunner) deleteContractListener(nodeURL string, listenerID string) error {
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "contracts/listeners", listenerID)
	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept": "application/json",
		}).
		Delete(fullPath)
	return err
}

func (pr *perfRunner) createContractsSub(nodeURL, listenerID string) (subID string, subName string, err error) {
	log.Infof("Creating contract subscription %s: %s", nodeURL, fmt.Sprintf("contracts_%s", pr.tagPrefix))
	var sub core.Subscription
	readAhead := uint16(len(pr.wsReceivers))
	firstEvent := core.SubOptsFirstEventNewest
	subPayload := core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name:      fmt.Sprintf("contracts_%s", pr.tagPrefix),
			Namespace: pr.cfg.FFNamespace,
		},
		Filter: core.SubscriptionFilter{
			Events: core.EventTypeBlockchainEventReceived.String(),
			BlockchainEvent: core.BlockchainEventFilter{
				Listener: listenerID,
			},
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				ReadAhead:  &readAhead,
				FirstEvent: &firstEvent,
			},
		},
		Transport: TRANSPORT_TYPE,
	}
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "subscriptions")
	if err != nil {
		return "", "", err
	}
	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&sub).
		Post(fullPath)
	if err != nil {
		log.Errorf("Could not create subscription on %s: %s", nodeURL, err)
		return "", "", err
	}

	log.Infof("Created contracts subscription on %s: %s", nodeURL, fmt.Sprintf("contracts_%s", pr.tagPrefix))

	pr.subscriptionIDsForNodes[nodeURL] = append(pr.subscriptionIDsForNodes[nodeURL], sub.ID.String())
	return sub.ID.String(), sub.Name, nil
}

func (pr *perfRunner) createTokenMintSub(nodeURL string) (subID string, subName string, err error) {
	log.Infof("Creating token mint subscription %s: %s", nodeURL, fmt.Sprintf("mint_%s", pr.tagPrefix))
	readAhead := uint16(len(pr.wsReceivers))
	firstEvent := core.SubOptsFirstEventNewest
	var sub core.Subscription
	subPayload := core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name:      fmt.Sprintf("mint_%s", pr.tagPrefix),
			Namespace: pr.cfg.FFNamespace,
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				ReadAhead:  &readAhead,
				FirstEvent: &firstEvent,
			},
		},
		Filter: core.SubscriptionFilter{
			Events: core.EventTypeTransferConfirmed.String(),
		},
		Transport: TRANSPORT_TYPE,
	}
	fullPath, err := url.JoinPath(nodeURL, pr.cfg.FFNamespacePath, "subscriptions")
	if err != nil {
		return "", "", err
	}
	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&sub).
		Post(fullPath)
	if err != nil {
		log.Errorf("Could not create subscription on %s: %s", nodeURL, err)
		return "", "", err
	}

	log.Infof("Created minted tokens subscription on %s: %s", nodeURL, fmt.Sprintf("mint_%s", pr.tagPrefix))

	pr.subscriptionIDsForNodes[nodeURL] = append(pr.subscriptionIDsForNodes[nodeURL], sub.ID.String())
	return sub.ID.String(), sub.Name, nil
}

type PaginatedResponse struct {
	Total int `ffstruct:"PaginatedResponse" json:"total,omitempty"`
}

func (pr *perfRunner) getMintRecipientBalance() (int, error) {
	var payload string

	var response PaginatedResponse
	var resError fftypes.RESTError
	fullPath, err := url.JoinPath(pr.client.BaseURL, pr.cfg.FFNamespacePath, "tokens/balances")
	if err != nil {
		return 0, nil
	}
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetQueryParams(map[string]string{
			"count": "true",
			"limit": "1",
			"key":   pr.cfg.TokenOptions.RecipientAddress,
		}).
		SetBody([]byte(payload)).
		SetResult(&response).
		SetError(&resError).
		Get(fullPath)
	if err != nil || res.IsError() {
		return 0, fmt.Errorf("Error querying token balance [%d]: %s (%+v)", resStatus(res), err, &resError)
	}

	return response.Total, nil
}

func (pr *perfRunner) IsDaemon() bool {
	return pr.daemon
}

func (pr *perfRunner) getIdempotencyKey(workerId int, iteration int) string {
	// Left pad worker ID to 5 digits (supporting up to 99,999 workers)
	workerIdStr := fmt.Sprintf("%05d", workerId)
	// Left pad iteration ID to 9 digits (supporting up to 999,999,999 iterations)
	iterationIdStr := fmt.Sprintf("%09d", iteration)
	return fmt.Sprintf("%v-%s-%s", pr.startTime, workerIdStr, iterationIdStr)
}

func (pr *perfRunner) calculateCurrentTps(logValue bool) float64 {
	// If we're still ramping, give the current rate during the ramp
	// If we're done ramping, calculate TPS from the end of the ramp onward
	var startTime int64
	var measuredActions int64
	if pr.ramping() {
		measuredActions = pr.rampSummary
		startTime = pr.startRampTime
	} else {
		measuredActions = pr.totalSummary
		startTime = pr.startTime
	}
	duration := time.Since(time.Unix(startTime, 0)).Seconds()
	currentTps := float64(measuredActions) / duration
	if logValue {
		log.Infof("Current TPS: %v Measured Actions: %v Duration: %v", currentTps, measuredActions, duration)
	}
	return currentTps
}

func (pr *perfRunner) ramping() bool {
	if time.Now().Before(time.Unix(pr.endRampTime, 0)) {
		return true
	}
	return false
}
