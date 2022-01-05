package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenMint(id int) {
	rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
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
						"value": "PerformanceTest-%d"
					}
				]
			}
		}`, pr.poolName, id)
	}
	targeter := pr.getApiTargeter("POST", "tokens/mint", payload)
	attacker := vegeta.NewAttacker()
	pr.runAttacker(rate, targeter, *attacker, id)
}
