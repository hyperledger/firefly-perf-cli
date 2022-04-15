package perf

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/go-resty/resty/v2"
	"github.com/hyperledger/firefly/pkg/fftypes"
)

type testBase struct {
	pr       *perfRunner
	workerID int
}

func (t *testBase) WorkerID() int {
	return t.workerID
}

func resStatus(res *resty.Response) int {
	if res == nil {
		return -1
	}
	return res.StatusCode()
}

func (t *testBase) getMessageString(isLongMsg bool) string {
	str := ""
	if isLongMsg {
		for i := 0; i < 100000; i++ {
			str = fmt.Sprintf("%s%d", str, t.WorkerID)
		}
		return str
	}
	for i := 0; i < 1000; i++ {
		str = fmt.Sprintf("%s%d", str, t.WorkerID)
	}
	return str
}

func (t *testBase) generateBlob(length *big.Int) ([]byte, [32]byte) {
	r, _ := rand.Int(rand.Reader, length)
	blob := make([]byte, r.Int64()+length.Int64())
	for i := 0; i < len(blob); i++ {
		blob[i] = byte('a' + i%26)
	}
	var blobHash fftypes.Bytes32 = sha256.Sum256(blob)
	return blob, blobHash
}

func (t *testBase) uploadBlob(blob []byte, hash [32]byte, nodeURL string) (string, error) {
	var data fftypes.Data
	formData := map[string]string{}
	// If there's no datatype, tell FireFly to automatically add a data payload
	formData["autometa"] = "true"
	formData["metadata"] = `{"mymeta": "data"}`

	resp, err := t.pr.client.R().
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

func (t *testBase) downloadAndVerifyBlob(nodeURL, id string, expectedHash [32]byte) error {
	var blob []byte
	res, err := t.pr.client.R().
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
