// Package prover implements the prover client to interact with the
// prover-server
package prover

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/aragon/zkmultisig-node/types"
)

// Client implements the prover http client, used to make requests to the
// prover server
type Client struct {
	url string
	c   *http.Client
}

// NewClient returns a new Client for the given proverURL
func NewClient(proverURL string) *Client {
	httpClient := &http.Client{}
	return &Client{
		url: proverURL,
		c:   httpClient,
	}
}

type errorMsg struct {
	Message string `json:"message"`
}

// GenProof sends the given ZKInputs to the prover-server to trigger the
// zkProof generation
func (c *Client) GenProof(zki *types.ZKInputs) (uint64, error) {
	// TODO check if there exists already a proof in db for the processID.
	// if so, check if time since insertedDatetime is bigger than T (eg. 10
	// minutes), if so, remove it and continue this function. If not,
	// return error saying that proof is still not ready

	jsonZKI, err := json.Marshal(zki)
	if err != nil {
		return 0, err
	}
	resp, err := c.c.Post(c.url+"/proof", "application/json", bytes.NewBuffer(jsonZKI))
	if err != nil {
		return 0, err
	}

	// resp.body.id contains the id to use to retrieve the proof later
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		var errMsg errorMsg
		if err = json.Unmarshal(body, &errMsg); err != nil {
			return 0, err
		}
		return 0, errors.New(errMsg.Message)
	}

	var m map[string]uint64
	err = json.Unmarshal(body, &m)
	if err != nil {
		return 0, err
	}

	return m["id"], nil
}

// GetProof retrieves the genereted proof (if already generated) from the
// prover-server for the given proofID
func (c *Client) GetProof(proofID uint64) (*types.ProofInDB, error) {
	return nil, nil
}
