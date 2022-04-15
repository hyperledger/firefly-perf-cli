package perf

import (
	"fmt"
	"strconv"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

type customFabric struct {
	testBase
}

func newCustomFabricTestWorker(pr *perfRunner, workerID int) TestCase {
	return &customFabric{
		testBase: testBase{
			pr:       pr,
			workerID: workerID,
		},
	}
}

func (tc *customFabric) Name() string {
	return "Custom Ethereum"
}

func (tc *customFabric) IDType() TrackingIDType {
	return TrackingIDTypeWorkerNumber
}

func (tc *customFabric) RunOnce() (string, error) {
	payload := fmt.Sprintf(`{
		"location": {
			"channel": "%s",
			"chaincode": "%s"
		},
		"method": {
			"name": "CreateAsset",
			"params": [
				{
					"name": "name",
					"schema": {
						"type": "string",
						"details": {
							"type": "string"
						}
					}
				}
			],
			"returns": []
		},
		"input": {
			"name": "%v"
		}
	}`, tc.pr.cfg.ContractOptions.Channel, tc.pr.cfg.ContractOptions.Chaincode, tc.workerID)
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
