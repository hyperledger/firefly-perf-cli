package perf

import (
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (pr *perfRunner) RunGetTransactions(id int) {
	rate := vegeta.Rate{Freq: pr.cfg.Frequency, Per: time.Second}
	targeter := pr.getApiTargeter("GET", "transactions", "")
	attacker := vegeta.NewAttacker()
	pr.runAttacker(rate, targeter, *attacker, id)
}
