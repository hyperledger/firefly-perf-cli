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
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	log "github.com/sirupsen/logrus"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
)

type customEthereum struct {
	testBase
}

func newCustomEthereumTestWorker(pr *perfRunner, workerID int, actionsPerLoop int) TestCase {
	return &customEthereum{
		testBase: testBase{
			pr:             pr,
			workerID:       workerID,
			actionsPerLoop: actionsPerLoop,
		},
	}
}

func (tc *customEthereum) Name() string {
	return conf.PerfTestCustomEthereumContract.String()
}

func (tc *customEthereum) IDType() TrackingIDType {
	return TrackingIDTypeWorkerNumber
}

func (tc *customEthereum) RunOnce(iterationCount int) (string, error) {
	idempotencyKey := tc.pr.getIdempotencyKey(tc.workerID, iterationCount)
	invokeOptionsJSON := ""
	if tc.pr.cfg.InvokeOptions != nil {
		b, err := json.Marshal(tc.pr.cfg.InvokeOptions)
		if err == nil {
			invokeOptionsJSON = fmt.Sprintf(",\n		 \"options\": %s", b)
		}
	}
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
		},
		"key": "%s",
		"idempotencyKey": "%s"%s
	}`, tc.pr.cfg.ContractOptions.Address, tc.workerID, tc.pr.cfg.SigningKey, idempotencyKey, invokeOptionsJSON)
	var resContractCall map[string]interface{}
	var resError fftypes.RESTError
	fullPath, err := url.JoinPath(tc.pr.client.BaseURL, tc.pr.cfg.FFNamespacePath, "contracts/invoke")
	if err != nil {
		return "", err
	}
	res, err := tc.pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload)).
		SetResult(&resContractCall).
		SetError(&resError).
		Post(fullPath)
	if err != nil || res.IsError() {
		if res.StatusCode() == 409 {
			log.Warnf("Request already received by FireFly: %+v", &resError)
		} else {
			return "", fmt.Errorf("Error invoking contract [%d]: %s (%+v)", resStatus(res), err, &resError)
		}
	}
	return strconv.Itoa(tc.workerID), nil
}
