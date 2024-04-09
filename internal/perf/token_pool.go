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
	"fmt"
	"net/url"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly/pkg/core"
	log "github.com/sirupsen/logrus"
)

func (pr *perfRunner) CreateTokenPool() error {
	log.Infof("Creating Token Pool: %s", pr.poolName)
	var config fftypes.JSONObject = make(map[string]interface{})

	body := core.TokenPool{
		Connector: pr.cfg.TokenOptions.TokenPoolConnectorName,
		Name:      pr.poolName,
		Type:      getTokenTypeEnum(pr.cfg.TokenOptions.TokenType),
		Config:    config,
	}

	if pr.cfg.TokenOptions.Config.PoolAddress != "" {
		config["address"] = pr.cfg.TokenOptions.Config.PoolAddress
	}

	if pr.cfg.TokenOptions.Config.PoolBlockNumber != "" {
		config["blockNumber"] = pr.cfg.TokenOptions.Config.PoolBlockNumber
	}
	fullPath, err := url.JoinPath(pr.client.BaseURL, pr.cfg.FFNamespacePath, "tokens/pools")
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s?confirm=true&publish=true", fullPath)
	res, err := pr.client.R().
		SetBody(&body).
		Post(url)

	if err != nil || !res.IsSuccess() {
		return errors.New("Failed to create token pool")
	}
	return err
}

func getTokenTypeEnum(tokenType string) fftypes.FFEnum {
	if tokenType == "nonfungible" {
		return core.TokenTypeNonFungible
	}
	return core.TokenTypeFungible
}
