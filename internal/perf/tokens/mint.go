package tokens

import (
	"fmt"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenMintTest() error {
	tm.displayMessage("Minting...")
	rate := vegeta.Rate{Freq: tm.config.Frequency, Per: time.Second}
	targeter := tm.getTokenMintingTargeter()
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}

func (tm *tokenManager) getTokenMintingTargeter() vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/tokens/mint", tm.config.Node)
		payload := fmt.Sprintf(`{
			"pool": "%s",
			"amount": "10"
		}`, tm.poolName)

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}
