package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunPrivateMessage() {
	pr.displayMessage("Sending Private Messages...")
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
		"data": [
			{
				"value": {
					"private": "message"
				}
			}
		],
		"group": {
			"members": [
				{
					"identity": "%s"
				}
			]
		}
	}`, pr.cfg.Recipient)
			targeter := pr.getDataTargeter("POST", "private", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, time.Now().Unix())
		case <-pr.shutdown:
			return
		}
	}
}
