package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenBurn(id string) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"amount": "1",
				"pool": "%s"
			}`, pr.poolName)
			targeter := pr.getApiTargeter("POST", "tokens/burn", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, id, true)
		case <-pr.shutdown:
			return
		}
	}
}
