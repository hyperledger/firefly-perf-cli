package tokens

import (
	"errors"
	"fmt"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

func (tm *tokenManager) CreateTokenPool() error {
	fmt.Println("--------------------------")
	fmt.Printf("Benchmarking Tokens: Creating Pool %s\n", tm.poolName)
	body := fftypes.TokenPool{
		Name: tm.poolName,
		Type: fftypes.TokenTypeFungible,
	}

	res, err := tm.client.R().
		SetHeader("Request-Timeout", "15s").
		SetBody(&body).
		Post("/api/v1/namespaces/default/tokens/pools?confirm=true")

	if err != nil || !res.IsSuccess() {
		return errors.New("Failed to create token pool")
	}
	return err
}
