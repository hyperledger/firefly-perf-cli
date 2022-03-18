package perf

import (
	"fmt"
	"math/big"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
)

func (pr *perfRunner) RunBlobPrivateMessage(nodeURL string, id int) {

	blob, hash := pr.generateBlob(big.NewInt(1024))
	dataID, err := pr.uploadBlob(blob, hash, nodeURL)
	if err != nil {
		fmt.Print(err)
		return
	}

	payload := fmt.Sprintf(`{
		"data":[
		   {
			   "id": "%s"
		   }
		],
		"group": {
			"members": [
				{
					"identity": "%s"
				}
			]
		},
		"header":{
		   "tag": "%s"
		}
	 }`, dataID, pr.cfg.Recipient, fmt.Sprintf("blob_%s_%d", pr.tagPrefix, id))
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, nodeURL, "messages/private", id, conf.PerfBlobBroadcast.String())
}
