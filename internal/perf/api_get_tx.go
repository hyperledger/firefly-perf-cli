package perf

import (
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunGetTransactions(uuid fftypes.UUID) {
	for {
		select {
		case <-pr.bfr:
			rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
			targeter := pr.getApiTargeter("GET", "transactions", "")
			attacker := vegeta.NewAttacker()

			pr.runAndReport(rate, targeter, *attacker, uuid, false)
		case <-pr.shutdown:
			return
		}
	}
}
