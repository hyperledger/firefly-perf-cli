package perf

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunBroadcast(id int) {
	rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
	payload := fmt.Sprintf(`{
		"data":[
		   {
			  "value":{
				 "broadcastID":"%d"
			  }
		   }
		],
		"header":{
		   "tag":"%d"
		}
	 }`, id, id)
	targeter := pr.getApiTargeter("POST", "messages/broadcast", payload)
	attacker := vegeta.NewAttacker()
	pr.runAttacker(rate, targeter, *attacker, id)
}
