package perf

import (
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunGetTransactions(id string) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			targeter := pr.getApiTargeter("GET", "transactions", "")
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, id, false)
		case <-pr.shutdown:
			return
		}
	}
}
