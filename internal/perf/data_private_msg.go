package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunPrivateMessage(uuid fftypes.UUID) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"data": [
					{
						"value": {
							"privateUUID": "%s"
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
			}`, uuid.String(), pr.cfg.Recipient, uuid.String())
			targeter := pr.getApiTargeter("POST", "messages/private", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, uuid, true)
		case <-pr.shutdown:
			return
		}
	}
}
