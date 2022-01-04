package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunPrivateMessage(id string) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"data": [
					{
						"value": {
							"privateID": "%s"
						}
					}
				],
				"group": {
					"members": [
						{
							"identity": "%s"
						}
					]
				},
				"header":{
					"tag":"%s"
				}
			}`, id, pr.cfg.Recipient, id)
			targeter := pr.getApiTargeter("POST", "messages/private", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, id, true)
		case <-pr.shutdown:
			return
		}
	}
}
