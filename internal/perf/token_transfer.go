package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunTokenTransfer(id int) {
	payload := fmt.Sprintf(`{
		"amount": "1",
		"pool": "%s",
		"to": "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}`, pr.poolName)
	targeter := pr.getApiTargeter("POST", "tokens/transfers", payload)
	pr.runAttacker(targeter, id, conf.PerfCmdTokenTransfer.String())
}
