package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunBroadcast(nodeURL string, id int) {
	payload := fmt.Sprintf(`{
		"data":[
		   {
			  "value":{
				 "broadcastID":"%s"
			  }
		   }
		],
		"header":{
		   "tag":"%s"
		}
	 }`, getMessageString(id, pr.cfg.MessageOptions.LongMessage), fmt.Sprintf("%s_%d", pr.tagPrefix, id))
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, nodeURL, "messages/broadcast", id, conf.PerfCmdBroadcast.String())
}

func getMessageString(id int, isLongMsg bool) string {
	str := ""
	if isLongMsg {
		for i := 0; i < 100000; i++ {
			str = fmt.Sprintf("%s%d", str, id)
		}
		return str
	}
	for i := 0; i < 1000; i++ {
		str = fmt.Sprintf("%s%d", str, id)
	}
	return str
}
