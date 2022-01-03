package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunBroadcast(uuid fftypes.UUID) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"data":[
				   {
					  "value":{
						 "broadcastUUID":"%s"
					  }
				   }
				],
				"header":{
				   "tag":"%s"
				}
			 }`, pr.getMessageString(uuid), uuid.String())
			targeter := pr.getApiTargeter("POST", "messages/broadcast", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, uuid, true)
		case <-pr.shutdown:
			return
		}
	}
}

func (pr *perfRunner) getMessageString(uuid fftypes.UUID) string {
	if pr.cfg.MessageOptions.LongMessage {
		str := ""
		for i := 0; i < 20; i++ {
			str = str + uuid.String()
		}
		return str
	}
	return uuid.String()
}
