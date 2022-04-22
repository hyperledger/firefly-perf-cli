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
	"github.com/hyperledger/firefly-perf-cli/internal/conf"

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
	return conf.PerfTestTokenMint.String()
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
		}`, tc.pr.poolName, tc.pr.cfg.RecipientAddress, tc.workerID, fmt.Sprintf("%s_%d", tc.pr.tagPrefix, tc.workerID))
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
