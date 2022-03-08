package perf

import (
	"encoding/json"
	"fmt"
	"strconv"

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

func (pr *perfRunner) GetBlockchainEventValue(ref string) (int, error) {
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).Get(fmt.Sprintf("%s/api/v1/namespaces/default/blockchainevents/%s", pr.cfg.Node, ref))
	if err != nil {
		return -1, err
	}
	var responseBody map[string]interface{}
	err = json.Unmarshal(res.Body(), &responseBody)
	if err != nil {
		return -1, err
	}
	output := responseBody["output"].(map[string]interface{})
	value := output["value"].(string)
	return strconv.Atoi(value)
}
