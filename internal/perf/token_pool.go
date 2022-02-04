package perf

import (
	"errors"

	"github.com/hyperledger/firefly/pkg/fftypes"
	log "github.com/sirupsen/logrus"
)

func (pr *perfRunner) CreateTokenPool() error {
	log.Infof("Creating Token Pool: %s", pr.poolName)
	body := fftypes.TokenPool{
		Connector: "erc1155",
		Name:      pr.poolName,
		Type:      getTokenTypeEnum(pr.cfg.TokenOptions.TokenType),
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

func getTokenTypeEnum(tokenType string) fftypes.FFEnum {
	if tokenType == "nonfungible" {
		return fftypes.TokenTypeNonFungible
	}
	return fftypes.TokenTypeFungible
}
