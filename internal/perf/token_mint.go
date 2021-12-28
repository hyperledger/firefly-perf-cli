package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenMint() {
	pr.displayMessage("Minting...")
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"pool": "%s",
				"amount": "10"
			}`, pr.poolName)
			targeter := pr.getTokenTargeter("POST", "mint", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, time.Now().Unix())
		case <-pr.shutdown:
			return
		}
	}
}
