package tokens

import (
	"fmt"
	"time"

	"github.com/hyperledger/firefly/pkg/fftypes"
	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenMintWithMsgTest() error {
	tm.displayMessage("Minting with message...")
	rate := vegeta.Rate{Freq: tm.config.Frequency, Per: time.Second}
	payload := fmt.Sprintf(`{
		"pool": "%s",
		"amount": "10",
		"message": {
			"data": [
				{
					"value": "PerformanceTest-%s"
				}
			]
		}
	}`, tm.poolName, fftypes.NewUUID())
	targeter := tm.getTokenTargeter("POST", "mint", payload)
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}
