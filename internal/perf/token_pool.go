package perf

import (
	"errors"
	"fmt"

	"github.com/hyperledger/firefly/pkg/fftypes"
)

func (pr *perfRunner) CreateTokenPool() error {
	pr.displayMessage(fmt.Sprintf("Creating Token Pool: %s", pr.poolName))
	body := fftypes.TokenPool{
		Connector: "erc1155",
		Name:      pr.poolName,
		Type:      fftypes.TokenTypeFungible,
	}

	res, err := pr.client.R().
		SetHeader("Request-Timeout", "15s").
		SetBody(&body).
		Post("/api/v1/namespaces/default/tokens/pools?confirm=true")

	if err != nil || !res.IsSuccess() {
		return errors.New("Failed to create token pool")
	}
	return err
}
