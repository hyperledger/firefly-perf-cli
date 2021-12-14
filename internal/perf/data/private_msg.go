package data

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (dm *dataManager) RunDataPrivateMessageTest() error {
	dm.displayMessage("Sending Private Messages...")
	rate := vegeta.Rate{Freq: dm.cfg.Frequency, Per: time.Second}
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
	}`, dm.cfg.Recipient)
	targeter := dm.getDataTargeter("POST", "private", payload)
	attacker := vegeta.NewAttacker()

	return dm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}
