package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunPrivateMessage(nodeURL string, id int) {
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
			"tag":"%s"
		}
	}`, id, pr.cfg.Recipient, fmt.Sprintf("%s_%d", pr.tagPrefix, id))
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, nodeURL, "messages/private", id, conf.PerfCmdPrivateMsg.String())
}
