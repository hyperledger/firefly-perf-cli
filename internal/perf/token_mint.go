package perf

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunTokenMint(uuid fftypes.UUID) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			payload := fmt.Sprintf(`{
				"pool": "%s",
				"amount": "10"
			}`, pr.poolName)
			targeter := pr.getTokenTargeter("POST", "mint", payload)
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, uuid)
		case <-pr.shutdown:
			return
		}
	}
}
