package tokens

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenBurnTest() error {
	tm.displayMessage("Burning...")
	rate := vegeta.Rate{Freq: tm.cfg.Frequency, Per: time.Second}
	payload := fmt.Sprintf(`{
		"amount": "1",
		"pool": "%s"
	}`, tm.poolName)
	targeter := tm.getTokenTargeter("POST", "burn", payload)
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}
