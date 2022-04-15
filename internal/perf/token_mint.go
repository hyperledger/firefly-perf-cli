package perf

import (
	"fmt"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

type tokenMint struct {
	testBase
}

func newTokenMintTestWorker(pr *perfRunner, workerID int) TestCase {
	return &tokenMint{
		testBase: testBase{
			pr:       pr,
			workerID: workerID,
		},
	}
}

func (tc *tokenMint) Name() string {
	return "Token Mint"
}

func (tc *tokenMint) IDType() TrackingIDType {
	return TrackingIDTypeTransferID
}

func (tc *tokenMint) RunOnce() (string, error) {
	payload := fmt.Sprintf(`{
			"pool": "%s",
			"amount": "10",
			"to": "%s",
			"message": {
				"data": [
					{
						"value": "MintTokenPerformanceTest-%d"
					}
				],
				"header": {
					"tag": "%s"
				}
			}
		}`, tc.pr.poolName, tc.pr.cfg.RecipientAddress, tc.WorkerID, fmt.Sprintf("%s_%d", tc.pr.tagPrefix, tc.WorkerID))
	var resTransfer fftypes.TokenTransfer
	var resError fftypes.RESTError
	res, err := tc.pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload)).
		SetResult(&resTransfer).
		SetError(&resError).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/tokens/mint", tc.pr.client.BaseURL))
	if err != nil || res.IsError() {
		return "", fmt.Errorf("Error sending token mint [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return resTransfer.LocalID.String(), nil
}
