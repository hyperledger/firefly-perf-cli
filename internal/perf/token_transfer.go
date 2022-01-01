package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenTransfer(uuid fftypes.UUID) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"amount": "5",
				"pool": "%s",
				"to": "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
			}`, pr.poolName)
			targeter := pr.getTokenTargeter("POST", "transfers", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, uuid)
		case <-pr.shutdown:
			return
		}
	}
}
