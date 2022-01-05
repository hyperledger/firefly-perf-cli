package perf

import (
	"fmt"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunTokenBurn(id int) {
	payload := fmt.Sprintf(`{
		"amount": "1",
		"pool": "%s"
	}`, pr.poolName)
	targeter := pr.getApiTargeter("POST", "tokens/burn", payload)
	pr.runAttacker(targeter, id, conf.PerfCmdTokenBurn.String())
}
