package tokens

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenTransferTest() error {
	tm.displayMessage("Transferring...")
	rate := vegeta.Rate{Freq: tm.config.Frequency, Per: time.Second}
	payload := fmt.Sprintf(`{
		"amount": "5",
		"pool": "%s",
		"to": "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}`, tm.poolName)
	targeter := tm.getTokenTargeter("POST", "transfers", payload)
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}
