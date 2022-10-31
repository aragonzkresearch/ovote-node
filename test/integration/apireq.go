package integration

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"

	"github.com/aragonzkresearch/ovote-node/api"
	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-iden3-crypto/babyjub"
)

func get(c *qt.C, url string) *http.Response {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	c.Assert(err, qt.IsNil)
	resp, err := client.Do(req)
	c.Assert(err, qt.IsNil)
	return resp
}

func post(c *qt.C, url string, jsonReqData []byte) *http.Response {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonReqData))
	c.Assert(err, qt.IsNil)
	resp, err := client.Do(req)
	c.Assert(err, qt.IsNil)
	return resp
}

func doPostNewCensus(c *qt.C, nodeURL string, pubKs []babyjub.PublicKey,
	weights []*big.Int) uint64 {
	reqData := api.AddKeysReq{PublicKeys: pubKs, Weights: weights}
	jsonReqData, err := json.Marshal(reqData)
	c.Assert(err, qt.IsNil)

	resp := post(c, nodeURL+"/census", jsonReqData)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	// get the censusID from the response
	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, qt.IsNil)
	var censusID uint64
	err = json.Unmarshal(body, &censusID)
	c.Assert(err, qt.IsNil)
	return censusID
}

func doPostCloseCensus(c *qt.C, nodeURL string, censusID uint64) []byte {
	censusIDStr := strconv.Itoa(int(censusID))
	resp := post(c, nodeURL+"/census/"+censusIDStr+"/close", nil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, qt.IsNil)
	var rootHex string
	err = json.Unmarshal(body, &rootHex)
	c.Assert(err, qt.IsNil)
	root, err := hex.DecodeString(rootHex)
	c.Assert(err, qt.IsNil)
	return root
}

func doGetCensusProof(c *qt.C, nodeURL string, censusID uint64,
	pubK babyjub.PublicKey) types.CensusProof {
	censusIDStr := strconv.Itoa(int(censusID))
	pubKComp := pubK.Compress()
	pubKHex := hex.EncodeToString(pubKComp[:])

	resp := get(c, nodeURL+"/census/"+censusIDStr+"/merkleproof/"+pubKHex)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)

	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, qt.IsNil)
	var cp types.CensusProof
	err = json.Unmarshal(body, &cp)
	c.Assert(err, qt.IsNil)
	return cp
}

func doPostVote(c *qt.C, nodeURL string, processID uint64, vote types.VotePackage) {
	processIDStr := strconv.Itoa(int(processID))
	jsonReqData, err := json.Marshal(vote)
	c.Assert(err, qt.IsNil)

	resp := post(c, nodeURL+"/process/"+processIDStr, jsonReqData)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)
}
