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
