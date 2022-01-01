package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenBurn(uuid fftypes.UUID) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"amount": "1",
				"pool": "%s"
			}`, pr.poolName)
			targeter := pr.getTokenTargeter("POST", "burn", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, uuid)
		case <-pr.shutdown:
			return
		}
	}
}
