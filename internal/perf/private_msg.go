package perf

import (
	"fmt"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

type private struct {
	testBase
}

func newPrivateTestWorker(pr *perfRunner, workerID int) TestCase {
	return &private{
		testBase: testBase{
			pr:       pr,
			workerID: workerID,
		},
	}
}

func (tc *private) Name() string {
	return "Private Message"
}

func (tc *private) IDType() TrackingIDType {
	return TrackingIDTypeMessageID
}

func (tc *private) RunOnce() (string, error) {

	payload := fmt.Sprintf(`{
		"data": [
			{
				"value": {
					"privateID": "%s"
				}
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
			"tag":"%s"
		}
	}`, tc.getMessageString(tc.pr.cfg.MessageOptions.LongMessage), tc.pr.cfg.Recipient, fmt.Sprintf("%s_%d", tc.pr.tagPrefix, tc.workerID))
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
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/messages/private", tc.pr.client.BaseURL))
	if err != nil || res.IsError() {
		return "", fmt.Errorf("Error sending private message [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return resMessage.Header.ID.String(), nil
}
