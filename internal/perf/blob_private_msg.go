package perf

import (
	"fmt"
	"math/big"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/core"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
)

type blobPrivate struct {
	testBase
}

func newBlobPrivateTestWorker(pr *perfRunner, workerID int, actionsPerLoop int) TestCase {
	return &blobPrivate{
		testBase: testBase{
			pr:             pr,
			workerID:       workerID,
			actionsPerLoop: actionsPerLoop,
		},
	}
}

func (tc *blobPrivate) Name() string {
	return conf.PerfTestBlobPrivateMsg.String()
}

func (tc *blobPrivate) IDType() TrackingIDType {
	return TrackingIDTypeMessageID
}

func (tc *blobPrivate) RunOnce() (string, error) {

	blob, hash := tc.generateBlob(big.NewInt(1024))
	dataID, err := tc.uploadBlob(blob, hash, tc.pr.client.BaseURL)
	if err != nil {
		return "", fmt.Errorf("Error uploading blob: %s", err)
	}

	payload := fmt.Sprintf(`{
		"data":[
		   {
			   "id": "%s"
		   }
		],
		"group": {
			"members": [
				{
					"identity": "%s"
				}
			]
		},
		"header":{
		   "tag": "%s"
		}
	 }`, dataID, tc.pr.cfg.RecipientOrg, fmt.Sprintf("blob_%s_%d", tc.pr.tagPrefix, tc.workerID))
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
		Post(fmt.Sprintf("%s/%s/api/v1/namespaces/%s/messages/private", tc.pr.client.BaseURL, tc.pr.cfg.APIPrefix, tc.pr.cfg.FFNamespace))
	if err != nil || res.IsError() {
		return "", fmt.Errorf("Error sending private message with blob attachment [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return resMessage.Header.ID.String(), nil
}
