package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunPrivateMessage() {
	uuid := fftypes.NewUUID()
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
				},
				"header":{
					"tag":"%s"
				}
			}`, pr.cfg.Recipient, uuid)
			targeter := pr.getDataTargeter("POST", "private", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, *uuid)
		case <-pr.shutdown:
			return
		}
	}
}
