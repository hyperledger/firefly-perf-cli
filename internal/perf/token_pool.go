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
	"errors"

	"github.com/hyperledger/firefly/pkg/fftypes"
	log "github.com/sirupsen/logrus"
)

func (pr *perfRunner) CreateTokenPool() error {
	log.Infof("Creating Token Pool: %s", pr.poolName)
	body := fftypes.TokenPool{
		Connector: "erc20_erc721",
		Name:      pr.poolName,
		Type:      getTokenTypeEnum(pr.cfg.TokenOptions.TokenType),
	}

	res, err := pr.client.R().
		SetHeader("Request-Timeout", "15s").
		SetBody(&body).
		Post("/api/v1/namespaces/default/tokens/pools?confirm=true")

	if err != nil || !res.IsSuccess() {
		return errors.New("Failed to create token pool")
	}
	return err
}

func getTokenTypeEnum(tokenType string) fftypes.FFEnum {
	if tokenType == "nonfungible" {
		return fftypes.TokenTypeNonFungible
	}
	return fftypes.TokenTypeFungible
}
