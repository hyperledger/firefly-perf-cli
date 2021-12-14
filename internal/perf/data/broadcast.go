package data

import (
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (dm *dataManager) RunDataBroadcastTest() error {
	dm.displayMessage("Sending Broadcasts...")
	rate := vegeta.Rate{Freq: dm.cfg.Frequency, Per: time.Second}
	payload := `{
		"data": [
			{
				"value": {
					"test": "json"
				}
			}
		]
	}`
	targeter := dm.getDataTargeter("POST", "broadcast", payload)
	attacker := vegeta.NewAttacker()

	return dm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}
