package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenTransfer(id string) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"amount": "5",
				"pool": "%s",
				"to": "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			}`, pr.poolName)
			targeter := pr.getApiTargeter("POST", "tokens/transfers", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, id, true)
		case <-pr.shutdown:
			return
		}
	}
}
