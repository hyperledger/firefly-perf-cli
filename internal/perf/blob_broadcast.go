package perf

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/hyperledger/firefly-perf-cli/internal/conf"
	"github.com/hyperledger/firefly/pkg/fftypes"
)

func (pr *perfRunner) RunBlobBroadcast(nodeURL string, id int) {

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
		"header":{
		   "tag": "%s"
		}
	 }`, dataID, fmt.Sprintf("blob_%s_%d", pr.tagPrefix, id))
	req := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
		}).
		SetBody([]byte(payload))
	pr.sendAndWait(req, nodeURL, "messages/broadcast", id, conf.PerfBlobBroadcast.String())
}

func (pr *perfRunner) generateBlob(length *big.Int) ([]byte, [32]byte) {
	r, _ := rand.Int(rand.Reader, length)
	blob := make([]byte, r.Int64()+length.Int64())
	for i := 0; i < len(blob); i++ {
		blob[i] = byte('a' + i%26)
	}
	var blobHash fftypes.Bytes32 = sha256.Sum256(blob)
	return blob, blobHash
}

func (pr *perfRunner) uploadBlob(blob []byte, hash [32]byte, nodeURL string) (string, error) {
	var data fftypes.Data
	formData := map[string]string{}
	// If there's no datatype, tell FireFly to automatically add a data payload
	formData["autometa"] = "true"
	formData["metadata"] = `{"mymeta": "data"}`

	resp, err := pr.client.R().
		SetFormData(formData).
		SetFileReader("file", "myfile.txt", bytes.NewReader(blob)).
		SetResult(&data).
		Post(nodeURL + "/api/v1/namespaces/default/data")
	if err != nil {
		return "", nil
	}
	if resp.StatusCode() != 201 {
		return "", fmt.Errorf(string(resp.Body()))
	}
	if *data.Blob.Hash != hash {
		return "", fmt.Errorf("blob hash was not equal")
	}
	return data.ID.String(), nil
}

func (pr *perfRunner) downloadAndVerifyBlob(nodeURL, id string, expectedHash [32]byte) error {
	var blob []byte
	res, err := pr.client.R().
		SetHeaders(map[string]string{
			"Accept":       "application/octet",
			"Content-Type": "application/json",
		}).
		SetResult(&blob).
		Get(fmt.Sprintf("%s/api/v1/namespaces/default/data/%s/blob", nodeURL, id))
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf(string(res.Body()))
	}
	actualHash := sha256.Sum256(blob)
	if actualHash != expectedHash {
		return fmt.Errorf("blob hash '%s' did not match expected hash '%s'", actualHash, expectedHash)
	}
	return nil
}
