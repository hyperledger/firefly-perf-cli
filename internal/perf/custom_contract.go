package perf

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunCustomContract(id int) {
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
	}`, pr.cfg.ContractOptions.Address, id)
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, "contracts/invoke", id, conf.PerfCmdCustomContract.String())
}

func (pr *perfRunner) GetBlockchainEventValue(nodeURL, ref string) (res int, err error) {
	retries := 3
	for i := 0; i < retries-1; i++ {
		res, err = pr.getBlockchainEventValue(nodeURL, ref)
		if err == nil {
			return res, err
		}
		time.Sleep(1000 * time.Millisecond)
	}
	return res, err
}

func (pr *perfRunner) getBlockchainEventValue(nodeURL, ref string) (int, error) {
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Get(fmt.Sprintf("%s/api/v1/namespaces/default/blockchainevents/%s", nodeURL, ref))
	if err != nil {
		return -1, err
	}
	var responseBody map[string]interface{}
	err = json.Unmarshal(res.Body(), &responseBody)
	if err != nil {
		return -1, err
	}
	output, ok := responseBody["output"].(map[string]interface{})
	if !ok {
		return -1, errors.New("output was nil")
	}
	value, ok := output["value"].(string)
	if !ok {
		return -1, errors.New("output value was nil")
	}
	return strconv.Atoi(value)
}
