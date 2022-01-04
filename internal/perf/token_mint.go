package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenMint(id string) {
	for {
		select {
		case <-pr.bfr:
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
								"value": "PerformanceTest-%s"
							}
						]
					}
				}`, pr.poolName, id)
			}
			targeter := pr.getApiTargeter("POST", "tokens/mint", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, id, true)
		case <-pr.shutdown:
			return
		}
	}
}
