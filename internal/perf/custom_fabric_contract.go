package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunCustomFabricContract(nodeURL string, id int) {
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
	}`, pr.cfg.ContractOptions.Channel, pr.cfg.ContractOptions.Chaincode, id)
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, nodeURL, "contracts/invoke", id, conf.PerfCmdCustomFabricContract.String())
}
