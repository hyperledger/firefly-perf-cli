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
	"math"
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

var METRICS_NAMESPACE = "ffperf"
var METRICS_SUBSYSTEM = "runner"

var totalActionsCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "actions_submitted_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var sentMintsCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "sent_mints_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var sentMintErrorCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "sent_mint_errors_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var mintBalanceGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "mint_token_balance",
	Subsystem: METRICS_SUBSYSTEM,
})

var receivedEventsCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "received_events_total",
	Subsystem: METRICS_SUBSYSTEM,
})

var incompleteEventsCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: METRICS_NAMESPACE,
	Name:      "incomplete_events_total",
	Subsystem: METRICS_SUBSYSTEM,
})

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
	bfr               chan int
	cfg               *conf.RunnerConfig
	client            *resty.Client
	ctx               context.Context
	shutdown          context.CancelFunc
	startTime         int64
	endTime           int64
	msgTimeMap        map[string]*inflightTest
	totalSummary      int64
	poolName          string
	poolConnectorName string
	tagPrefix         string
	wsconns           []wsclient.WSClient
	wsReceivers       []chan string
	wsUUID            fftypes.UUID
	nodeURLs          []string
	subscriptionMap   map[string]SubscriptionInfo
	daemon            bool
	sender            string
	totalWorkers      int
}

type SubscriptionInfo struct {
	NodeURL string
	Name    string
	Job     fftypes.FFEnum
}

func New(config *conf.RunnerConfig) PerfRunner {
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

	pr := &perfRunner{
		bfr:               make(chan int, totalWorkers),
		cfg:               config,
		ctx:               ctx,
		shutdown:          cancel,
		startTime:         time.Now().Unix(),
		endTime:           time.Now().Unix() + int64(config.Length.Seconds()),
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
		wsconn, err := wsclient.New(pr.ctx, wsConfig, nil, pr.startSubscriptions)
		if err != nil {
			log.Error("Could not create websocket connection: %s", err)
		}
		wsconns[i] = wsconn
	}

	pr.wsconns = wsconns
	return pr
}

func (pr *perfRunner) Init() (err error) {
	pr.client = getFFClient(pr.sender)
	pr.client.SetBasicAuth(pr.cfg.WebSocket.AuthUsername, pr.cfg.WebSocket.AuthPassword)

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

	for _, nodeURL := range pr.nodeURLs {

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

			go func() {
				err := pr.runLoop(tc)
				if err != nil {
					log.Errorf("Worker %d failed: %s", tc.WorkerID(), err)
				}
			}()
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

	// If configured, check that the balance of the mint recipient address is correct
	if pr.detectDeliquentBalance() {
		if pr.cfg.DelinquentAction == conf.DelinquentActionExit.String() {
			log.Panic(fmt.Errorf("Token mint recipient balance didn't reach the expected value in the allowed time"))
		}
	}

	pr.shutdown()

	// we sleep on shutdown / completion to allow for Prometheus metrics to be scraped one final time
	// about 10s into our sleep all workers should be completed, so we check for delinquent messages
	// one last time so metrics are up-to-date
	endTime := time.Now().Unix()
	log.Warn("Runner stopping in 30s")
	time.Sleep(10 * time.Second)
	pr.detectDeliquentMsgs()
	time.Sleep(20 * time.Second)

	log.Info("Shutdown summary:")
	log.Infof(" - Prometheus metric sent_mints_total        = %f\n", getMetricVal(sentMintsCounter))
	log.Infof(" - Prometheus metric sent_mint_errors_total  = %f\n", getMetricVal(sentMintErrorCounter))
	log.Infof(" - Prometheus metric mint_token_balance      = %f\n", getMetricVal(mintBalanceGauge))
	log.Infof(" - Prometheus metric received_events_total   = %f\n", getMetricVal(receivedEventsCounter))
	log.Infof(" - Prometheus metric incomplete_events_total = %f\n", getMetricVal(incompleteEventsCounter))
	log.Infof(" - Prometheus metric deliquent_msgs_total    = %f\n", getMetricVal(deliquentMsgsCounter))
	log.Infof(" - Prometheus metric actions_submitted_total = %f\n", getMetricVal(totalActionsCounter))
	log.Infof(" - Test duration (secs): %d", endTime-pr.startTime)
	log.Infof(" - Completed actions: %d", pr.totalSummary)
	log.Infof(" - Completed actions/sec: %2f", float64(pr.totalSummary)/float64(endTime-pr.startTime))

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
			wsconn.Send(pr.ctx, ackJSON)
			// Release worker so it can continue to its next task
			if !pr.cfg.SkipMintConfirmations {
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

// Calculate what the current rate limit is
func (pr *perfRunner) getCurrentRate() int64 {
	if pr.cfg.RateRampUpTime.Seconds() == 0 {
		return pr.cfg.StartRate
	}

	timeSinceStart := time.Since(time.Unix(pr.startTime, 0))

	// % of way through ramp up time
	perc := timeSinceStart.Seconds() / float64(pr.cfg.RateRampUpTime.Seconds()) * 100

	// Rate to use now
	rateNow := math.Min(float64(pr.cfg.StartRate)+(float64((pr.cfg.EndRate-pr.cfg.StartRate))/float64(100)*perc), float64(pr.cfg.EndRate))

	return int64(rateNow)
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

	limiter = rate.NewLimiter(rate.Limit(pr.getCurrentRate()), 1)

	loop := 0
	for {
		select {
		case <-pr.bfr:
			var confirmationsPerAction int
			var actionsCompleted int
			limiter.SetLimit(rate.Limit(pr.getCurrentRate()))

			// Worker sends its task
			hist, histErr := perfTestDurationHistogram.GetMetricWith(prometheus.Labels{
				"test": testName,
			})

			if histErr != nil {
				log.Errorf("Error retreiving histogram: %s", histErr)
			}

			startTime := time.Now()
			trackingIDs := make([]string, 0)

			for actionsCompleted = 0; actionsCompleted < tc.ActionsPerLoop(); actionsCompleted++ {

				// If configured with rate limiting, wait until the rate limiter allows us to proceed
				if pr.cfg.StartRate > 0 {
					limiter.Wait(pr.ctx)
				}

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
					totalActionsCounter.Inc()
				}

				pr.markTestInFlight(tc, trackingID)
				log.Infof("%d --> %s Sent %s: %s", workerID, testName, idType, trackingID)
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
				nextTrackingID = trackingIDs[0]
				if len(trackingIDs) > 0 {
					trackingIDs = trackingIDs[1:]
				}
				pr.markTestComplete(nextTrackingID)
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
					log.Warnf("Failed to check token balance: ", err)
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
	subPayload := core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name:      name,
			Namespace: pr.cfg.FFNamespace,
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				ReadAhead: &readAhead,
			},
		},
		Ephemeral: true,
		Filter: core.SubscriptionFilter{
			Events: core.EventTypeMessageConfirmed.String(),
			Message: core.MessageFilter{
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
		Post(fmt.Sprintf("%s/%sapi/v1/namespaces/%s/subscriptions", nodeURL, pr.cfg.APIPrefix, pr.cfg.FFNamespace))
	if err != nil {
		log.Errorf("Could not create subscription: %s", err)
		return "", "", err
	}

	log.Infof("Created subscription on %s: %s", nodeURL, pr.tagPrefix)

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

func (pr *perfRunner) detectDeliquentMsgs() bool {
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
func (pr *perfRunner) detectDeliquentBalance() bool {

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

func (pr *perfRunner) markTestComplete(trackingID string) {
	mutex.Lock()
	pr.totalSummary++
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
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&responseBody).
		SetError(&errResponse).
		Post(fmt.Sprintf("%s/%sapi/v1/namespaces/%s/contracts/listeners", nodeURL, pr.cfg.APIPrefix, pr.cfg.FFNamespace))
	if err != nil {
		return "", err
	}
	if res.IsError() {
		return "", fmt.Errorf("Failed: %s", errResponse)
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
		Post(fmt.Sprintf("%s/%sapi/v1/namespaces/%s/contracts/listeners", nodeURL, pr.cfg.APIPrefix, pr.cfg.FFNamespace))
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

func (pr *perfRunner) createContractsSub(nodeURL, listenerID string) (subID string, subName string, err error) {
	log.Infof("Creating contract subscription %s: %s", nodeURL, fmt.Sprintf("contracts_%s", pr.tagPrefix))
	var sub core.Subscription
	subPayload := core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name:      fmt.Sprintf("contracts_%s", pr.tagPrefix),
			Namespace: pr.cfg.FFNamespace,
		},
		Ephemeral: true,
		Filter: core.SubscriptionFilter{
			Events: core.EventTypeBlockchainEventReceived.String(),
			BlockchainEvent: core.BlockchainEventFilter{
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
		Post(fmt.Sprintf("%s/%sapi/v1/namespaces/%s/subscriptions", nodeURL, pr.cfg.APIPrefix, pr.cfg.FFNamespace))
	if err != nil {
		log.Errorf("Could not create subscription on %s: %s", nodeURL, err)
		return "", "", err
	}

	log.Infof("Created contracts subscription on %s: %s", nodeURL, fmt.Sprintf("contracts_%s", pr.tagPrefix))

	return sub.ID.String(), sub.Name, nil
}

func (pr *perfRunner) createTokenMintSub(nodeURL string) (subID string, subName string, err error) {
	log.Infof("Creating token mint subscription %s: %s", nodeURL, fmt.Sprintf("mint_%s", pr.tagPrefix))
	readAhead := uint16(50)
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

	_, err = pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody(subPayload).
		SetResult(&sub).
		Post(fmt.Sprintf("%s/%sapi/v1/namespaces/%s/subscriptions", nodeURL, pr.cfg.APIPrefix, pr.cfg.FFNamespace))
	if err != nil {
		log.Errorf("Could not create subscription on %s: %s", nodeURL, err)
		return "", "", err
	}

	log.Infof("Created minted tokens subscription on %s: %s", nodeURL, fmt.Sprintf("mint_%s", pr.tagPrefix))

	return sub.ID.String(), sub.Name, nil
}

type PaginatedResponse struct {
	Total int `ffstruct:"PaginatedResponse" json:"total,omitempty"`
}

func (pr *perfRunner) getMintRecipientBalance() (int, error) {
	var payload string

	var response PaginatedResponse
	var resError fftypes.RESTError
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
		Get(fmt.Sprintf("%s/%sapi/v1/namespaces/%s/tokens/balances", pr.client.BaseURL, pr.cfg.APIPrefix, pr.cfg.FFNamespace))
	if err != nil || res.IsError() {
		return 0, fmt.Errorf("Error querying token balance [%d]: %s (%+v)", resStatus(res), err, &resError)
	}

	return response.Total, nil
}

func (pr *perfRunner) IsDaemon() bool {
	return pr.daemon
}
