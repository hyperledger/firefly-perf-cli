package tokens

import (
	"fmt"
	"net/http"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

func (tm *tokenManager) RunTokenTransferTest() error {
	fmt.Println("-----------------------------")
	fmt.Println("Transferring Tokens...")

	rate := vegeta.Rate{Freq: tm.config.Frequency, Per: time.Second}
	targeter := tm.getTokenTransferTargeter()
	attacker := vegeta.NewAttacker()

	return tm.runAndReport(rate, targeter, *attacker, time.Now().Unix())
}

func (tm *tokenManager) getTokenTransferTargeter() vegeta.Targeter {
	return func(t *vegeta.Target) error {
		if t == nil {
			return vegeta.ErrNilTarget
		}

		t.Method = "POST"
		t.URL = fmt.Sprintf("%s/api/v1/namespaces/default/tokens/transfers", tm.config.Node)
		payload := fmt.Sprintf(`{
			"amount": "5",
			"pool": "%s",
			"to": "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		}`, tm.poolName)

		t.Body = []byte(payload)

		header := http.Header{}
		header.Add("Accept", "application/json")
		header.Add("Content-Type", "application/json")
		t.Header = header

		return nil
	}
}
