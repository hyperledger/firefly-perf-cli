package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunTokenMint(id int) {
	payload := fmt.Sprintf(`{
		"pool": "%s",
		"amount": "10"
	}`, pr.poolName)
	if pr.cfg.TokenOptions.AttachMessage {
		payload = fmt.Sprintf(`{
			"pool": "%s",
			"amount": "10",
			"message": {
				"data": [
					{
						"value": "MintTokenPerformanceTest-%d"
					}
				],
				"header": {
      		"tag": "%s",
    		}
			}
		}`, pr.poolName, id, fmt.Sprintf("%s_%d", pr.tagPrefix, id))
	}
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, "tokens/mint", id, conf.PerfCmdTokenMint.String())
}
