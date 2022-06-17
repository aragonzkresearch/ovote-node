// Package prover implements the prover client to interact with the
// prover-server
package prover

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/groupoidlabs/ovote-node/types"
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
func (c *Client) GenProof(processID uint64, zki *types.ZKInputs) (uint64, error) {
	jsonZKI, err := json.Marshal(zki)
	if err != nil {
		return 0, err
	}
	resp, err := c.c.Post(
		c.url+"/proof", "application/json", bytes.NewBuffer(jsonZKI))
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

// GetProof retrieves the genereted proof and publicInputs (if already
// generated) from the prover-server for the given proofID
func (c *Client) GetProof(proofID uint64) ([]byte, []byte, error) {
	// request proof
	proof, err := c.getProof(proofID)
	if err != nil {
		return nil, nil, err
	}
	// request publicInputs
	publicInputs, err := c.getPublicInputs(proofID)
	if err != nil {
		return nil, nil, err
	}

	return proof, publicInputs, nil
}

func (c *Client) getProof(proofID uint64) ([]byte, error) {
	resp, err := c.c.Get(
		c.url + "/proof/" + strconv.Itoa(int(proofID)))
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		var errMsg errorMsg
		if err = json.Unmarshal(body, &errMsg); err != nil {
			return nil, err
		}
		return nil, errors.New(errMsg.Message)
	}
	return body, nil
}

func (c *Client) getPublicInputs(proofID uint64) ([]byte, error) {
	resp, err := c.c.Get(
		c.url + "/proof/" + strconv.Itoa(int(proofID)) + "/public")
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		var errMsg errorMsg
		if err = json.Unmarshal(body, &errMsg); err != nil {
			return nil, err
		}
		return nil, errors.New(errMsg.Message)
	}
	return body, nil
}
