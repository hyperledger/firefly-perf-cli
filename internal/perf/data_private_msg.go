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
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, "messages/private", id, conf.PerfCmdPrivateMsg.String())
}
