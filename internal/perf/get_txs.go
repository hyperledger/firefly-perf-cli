package perf

import "github.com/hyperledger/firefly-perf-cli/internal/conf"

func (pr *perfRunner) RunGetTransactions(id int) {
	targeter := pr.getApiTargeter("GET", "transactions", "")
	pr.runAttacker(targeter, id, conf.PerfCmdGetTransactions.String())
}
