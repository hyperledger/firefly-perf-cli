package data

import (
	"fmt"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (dm *dataManager) RunDataPrivateMessageTest() error {
	fmt.Println("Running private message performance benchmark")

	rate := vegeta.Rate{Freq: dm.config.Frequency, Per: time.Second}
	targeter := dm.getPrivateMessageTargeter()
	attacker := vegeta.NewAttacker()

	return dm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}

func (dm *dataManager) getPrivateMessageTargeter() vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/messages/private", dm.config.Node)
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
		}`, dm.config.Recipient)

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}
