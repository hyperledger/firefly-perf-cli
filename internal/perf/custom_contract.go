package perf

import (
	"fmt"

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
