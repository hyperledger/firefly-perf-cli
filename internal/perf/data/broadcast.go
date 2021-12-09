package data

import (
	"fmt"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (dm *dataManager) RunDataBroadcastTest() error {
	fmt.Println("----------------------------")
	fmt.Println("Sending Broadcasts...")

	rate := vegeta.Rate{Freq: dm.config.Frequency, Per: time.Second}
	targeter := getBroadcastTargeter(dm.config.Node)
	attacker := vegeta.NewAttacker()

	return dm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}

func getBroadcastTargeter(node string) vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/messages/broadcast", node)
		payload := `{
			"data": [
				{
					"value": {
						"test": "json"
					}
				}
			]
		}`

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}
