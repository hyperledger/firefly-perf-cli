package perf

import (
	"fmt"
	"math/big"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

type blobBroadcast struct {
	testBase
}

func newBlobBroadcastTestWorker(pr *perfRunner, workerID int) TestCase {
	return &blobBroadcast{
		testBase: testBase{
			pr:       pr,
			workerID: workerID,
		},
	}
}

func (tc *blobBroadcast) Name() string {
	return "Blob Broadcast"
}

func (tc *blobBroadcast) IDType() TrackingIDType {
	return TrackingIDTypeMessageID
}

func (tc *blobBroadcast) RunOnce() (string, error) {

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
		"header":{
		   "tag": "%s"
		}
	 }`, dataID, fmt.Sprintf("blob_%s_%d", tc.pr.tagPrefix, tc.workerID))
	var resMessage fftypes.Message
	var resError fftypes.RESTError
	res, err := tc.pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload)).
		SetResult(&resMessage).
		SetError(&resError).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/messages/broadcast", tc.pr.client.BaseURL))
	if err != nil || res.IsError() {
		return "", fmt.Errorf("Error sending broadcast message with blob attachment [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return resMessage.Header.ID.String(), nil
}
