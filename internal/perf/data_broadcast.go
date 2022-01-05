package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunBroadcast(id int) {
	payload := fmt.Sprintf(`{
		"data":[
		   {
			  "value":{
				 "broadcastID":"%s"
			  }
		   }
		],
		"header":{
		   "tag":"%d"
		}
	 }`, getMessageString(id, pr.cfg.MessageOptions.LongMessage), id)
	targeter := pr.getApiTargeter("POST", "messages/broadcast", payload)
	pr.runAttacker(targeter, id, conf.PerfCmdBroadcast.String())
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
