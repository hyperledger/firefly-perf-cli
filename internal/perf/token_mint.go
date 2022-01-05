package perf

import (
	"errors"
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	log "github.com/sirupsen/logrus"
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
				]
			}
		}`, pr.poolName, id)
	}
	targeter := pr.getApiTargeter("POST", "tokens/mint", payload)
	pr.runAttacker(targeter, id, conf.PerfCmdTokenMint.String())
}

func (pr *perfRunner) MintTokensForTransfer(transferType string) error {
	log.Infof("Minting tokens to %s", transferType)

	body := map[string]interface{}{
		"pool":   pr.poolName,
		"amount": "1000000000000",
	}

	res, err := pr.client.R().
		SetHeader("Request-Timeout", "15s").
		SetBody(&body).
		Post("/api/v1/namespaces/default/tokens/mint?confirm=true")

	if err != nil || !res.IsSuccess() {
		return errors.New(fmt.Sprintf("Failed to mint tokens for %s", transferType))
	}
	return nil
}
