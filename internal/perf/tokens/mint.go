package tokens

import (
	"fmt"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenMintTest() error {
	tm.displayMessage("Minting...")
	rate := vegeta.Rate{Freq: tm.config.Frequency, Per: time.Second}
	payload := fmt.Sprintf(`{
		"pool": "%s",
		"amount": "10"
	}`, tm.poolName)
	targeter := tm.getTokenTargeter("POST", "mint", payload)
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}
