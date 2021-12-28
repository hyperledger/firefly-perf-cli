package perf

import (
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunBroadcast() {
	pr.displayMessage("Sending Broadcasts...")
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := `{
					"data": [
						{
							"value": {
								"test": "json"
							}
						}
					]
				}`
			targeter := pr.getDataTargeter("POST", "broadcast", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, time.Now().Unix())
		case <-pr.shutdown:
			return
		}
	}
}
