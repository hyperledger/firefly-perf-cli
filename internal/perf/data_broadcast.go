package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunBroadcast(id string) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
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
			 }`, pr.getMessageString(id), id)
			targeter := pr.getApiTargeter("POST", "messages/broadcast", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, id, true)
		case <-pr.shutdown:
			return
		}
	}
}

func (pr *perfRunner) getMessageString(id string) string {
	if pr.cfg.MessageOptions.LongMessage {
		str := ""
		for i := 0; i < 100; i++ {
			str = str + id
		}
		return str
	}
	return id
}
