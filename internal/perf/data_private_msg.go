package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunPrivateMessage(id int) {
	rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
	payload := fmt.Sprintf(`{
		"data": [
			{
				"value": {
					"privateID": "%d"
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
			"tag":"%d"
		}
	}`, id, pr.cfg.Recipient, id)
	targeter := pr.getApiTargeter("POST", "messages/private", payload)
	attacker := vegeta.NewAttacker()
	pr.runAttacker(rate, targeter, *attacker, id)
}
