package perf

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

type customEthereum struct {
	testBase
}

func newCustomEthereumTestWorker(pr *perfRunner, workerID int) TestCase {
	return &customEthereum{
		testBase: testBase{
			pr:       pr,
			workerID: workerID,
		},
	}
}

func (tc *customEthereum) Name() string {
	return "Custom Ethereum"
}

func (tc *customEthereum) IDType() TrackingIDType {
	return TrackingIDTypeWorkerNumber
}

func (tc *customEthereum) RunOnce() (string, error) {
	payload := fmt.Sprintf(`{
		"location": {
			"address": "%s"
		},
		"method": {
			"name": "set",
			"params": [
				{
					"name": "newValue",
					"schema": {
						"type": "integer",
						"details": {
							"type": "uint256"
						}
					}
				}
			],
			"returns": []
		},
		"input": {
			"newValue": %v
		}
	}`, tc.pr.cfg.ContractOptions.Address, tc.workerID)
	var resContractCall fftypes.ContractCallResponse
	var resError fftypes.RESTError
	res, err := tc.pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload)).
		SetResult(&resContractCall).
		SetError(&resError).
		Post(fmt.Sprintf("%s/api/v1/namespaces/default/contracts/invoke", tc.pr.client.BaseURL))
	if err != nil || res.IsError() {
		return "", fmt.Errorf("Error invoking contract [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return strconv.Itoa(tc.workerID), nil
}
