package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/core"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
)

type broadcast struct {
	testBase
}

func newBroadcastTestWorker(pr *perfRunner, workerID int) TestCase {
	return &broadcast{
		testBase: testBase{
			pr:       pr,
			workerID: workerID,
		},
	}
}

func (tc *broadcast) Name() string {
	return conf.PerfTestBroadcast.String()
}

func (tc *broadcast) IDType() TrackingIDType {
	return TrackingIDTypeMessageID
}

func (tc *broadcast) RunOnce() (string, error) {

	payload := fmt.Sprintf(`{
		"data":[
		   {
			  "value":{
				 "broadcastID":"%s"
			  }
		   }
		],
		"header":{
		   "tag":"%s"
		}
	 }`, tc.getMessageString(tc.pr.cfg.MessageOptions.LongMessage), fmt.Sprintf("%s_%d", tc.pr.tagPrefix, tc.workerID))
	var resMessage core.Message
	var resError fftypes.RESTError
	res, err := tc.pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload)).
		SetResult(&resMessage).
		SetError(&resError).
		Post(fmt.Sprintf("%s/%s/api/v1/namespaces/%s/messages/broadcast", tc.pr.client.BaseURL, tc.pr.cfg.APIPrefix, tc.pr.cfg.FFNamespace))
	if err != nil || res.IsError() {
		return "", fmt.Errorf("Error sending broadcast message [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return resMessage.Header.ID.String(), nil
}
