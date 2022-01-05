package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunPrivateMessage(id int) {
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
	pr.runAttacker(targeter, id, conf.PerfCmdPrivateMsg.String())
}
