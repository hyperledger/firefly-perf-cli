package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunBroadcast() {
	uuid := fftypes.NewUUID()
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"data":[
				   {
					  "value":{
						 "test":"json"
					  }
				   }
				],
				"header":{
				   "tag":"%s"
				}
			 }`, uuid.String())
			targeter := pr.getDataTargeter("POST", "broadcast", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, *uuid)
		case <-pr.shutdown:
			return
		}
	}
}
