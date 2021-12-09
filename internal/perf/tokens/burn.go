package tokens

import (
	"fmt"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenBurnTest() error {
	fmt.Println("--------------------------")
	fmt.Println("Burning Tokens...")

	rate := vegeta.Rate{Freq: tm.config.Frequency, Per: time.Second}
	targeter := tm.getTokenBurnTargeter()
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}

func (tm *tokenManager) getTokenBurnTargeter() vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/tokens/burn", tm.config.Node)
		payload := fmt.Sprintf(`{
			"amount": "1",
			"pool": "%s"
		}`, tm.poolName)

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}
