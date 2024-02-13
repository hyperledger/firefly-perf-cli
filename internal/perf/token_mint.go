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
	"net/url"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/core"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
)

type tokenMint struct {
	testBase
}

func newTokenMintTestWorker(pr *perfRunner, workerID int, actionsPerLoop int) TestCase {
	return &tokenMint{
		testBase: testBase{
			pr:             pr,
			workerID:       workerID,
			actionsPerLoop: actionsPerLoop,
		},
	}
}

func (tc *tokenMint) Name() string {
	return conf.PerfTestTokenMint.String()
}

func (tc *tokenMint) IDType() TrackingIDType {
	return TrackingIDTypeTransferID
}

func (tc *tokenMint) GetSigningKey() string {
	if tc.pr.cfg.SigningKey != "" {
		// Use the hard-coded signing key
		return tc.pr.cfg.SigningKey
	}
	if tc.pr.cfg.PerWorkerSigningKeyPrefix != "" {
		// Use the per-worker signing key prefix with our worker ID appended
		return fmt.Sprintf("%s%d", tc.pr.cfg.PerWorkerSigningKeyPrefix, tc.testBase.workerID)
	}
	return ""
}

func (tc *tokenMint) RunOnce(iterationCount int) (string, error) {
	var payload string
	mintAmount := 10
	if tc.pr.cfg.TokenOptions.TokenType == core.TokenTypeNonFungible.String() {
		mintAmount = 1
	}

	common := fmt.Sprintf(`{
		"pool": "%s",
		"amount": "%d",
		"to": "%s",
		"key": "%s"`, tc.pr.poolName, mintAmount, tc.pr.cfg.RecipientAddress, tc.GetSigningKey())

	message := ""
	uri := ""

	if tc.pr.cfg.TokenOptions.SupportsData == nil || *tc.pr.cfg.TokenOptions.SupportsData { // Convoluted check that allows a bool to default to true
		message = fmt.Sprintf(`,
			"message": {
				"data": [
					{
						"value": "MintTokenPerformanceTest-%d"
					}
				],
				"header": {
					"tag": "%s"
				}
			}`, tc.workerID, fmt.Sprintf("%s_%d", tc.pr.tagPrefix, tc.workerID))
	}

	if tc.pr.cfg.TokenOptions.SupportsURI {
		uri = fmt.Sprintf(`,
				"uri": "ff-perf-cli://%d"`, tc.workerID)
	}

	payload = fmt.Sprintf("%s%s%s}", common, message, uri)

	var resTransfer core.TokenTransfer
	var resError fftypes.RESTError
	fullPath, err := url.JoinPath(tc.pr.client.BaseURL, tc.pr.cfg.FFNamespacePath, "tokens/mint")
	if err != nil {
		return "", err
	}
	res, err := tc.pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload)).
		SetResult(&resTransfer).
		SetError(&resError).
		Post(fullPath)
	sentMintsCounter.Inc()
	if err != nil || res.IsError() {
		sentMintErrorCounter.Inc()
		return "", fmt.Errorf("Error sending token mint [%d]: %s (%+v)", resStatus(res), err, &resError)
	}
	return resTransfer.LocalID.String(), nil
}
