package api

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/aragonzkresearch/ovote-node/censusbuilder"
	"github.com/aragonzkresearch/ovote-node/types"
	"github.com/aragonzkresearch/ovote-node/votesaggregator"
	"github.com/gin-gonic/gin"
	"go.vocdoni.io/dvote/log"
)

// API allows external requests to the Node
type API struct {
	r  *gin.Engine
	cb *censusbuilder.CensusBuilder
	va *votesaggregator.VotesAggregator
}

// New returns a new API with the endpoints, without starting to listen
func New(censusBuilder *censusbuilder.CensusBuilder,
	votesAggregator *votesaggregator.VotesAggregator) (*API, error) {
	if censusBuilder == nil && votesAggregator == nil {
		return nil, fmt.Errorf("Can not create the API. At least" +
			" censusBuilder or votesAggregator should be active to start" +
			" the API. Use --help to see the list of available flags.")
	}

	a := API{}
	r := gin.Default()

	if censusBuilder != nil {
		a.cb = censusBuilder
		// r.GET("/census", a.getCensuses) // TODO
		r.POST("/census", a.postNewCensus)
		r.GET("/census/:censusid", a.getCensus)
		r.POST("/census/:censusid", a.postAddKeys)
		r.POST("/census/:censusid/close", a.postCloseCensus)
		r.GET("/census/:censusid/merkleproof/:pubkey", a.getMerkleProofHandler)
	}

	if votesAggregator != nil {
		a.va = votesAggregator
		r.POST("/process/:processid", a.postVote)
		r.GET("/process/:processid", a.getProcess)
		r.POST("/proof/:processid", a.postGenProof)
		r.GET("/proof/:processid", a.getProof)
	}

	a.r = r

	return &a, nil
}

// Serve serves the API at the given port
func (a *API) Serve(port string) error {
	return a.r.Run(":" + port)
}

type errorMsg struct {
	Message string `json:"message"`
}

func returnErr(c *gin.Context, err error) {
	log.Warnw("HTTP API Bad request error", "err", err)
	c.JSON(http.StatusBadRequest, errorMsg{
		Message: err.Error(),
	})
}

func (a *API) postNewCensus(c *gin.Context) {
	var d AddKeysReq
	err := c.ShouldBindJSON(&d)
	if err != nil {
		returnErr(c, err)
		return
	}

	censusID, err := a.cb.NewCensus()
	if err != nil {
		returnErr(c, err)
		return
	}

	// TODO maybe remove the key addition, to force usage of separated
	// endpoints (newCensus, and then addKeys)
	go a.cb.AddPublicKeysAndStoreError(censusID, d.PublicKeys, d.Weights)

	c.JSON(http.StatusOK, censusID)
}

func (a *API) postAddKeys(c *gin.Context) {
	censusIDStr := c.Param("censusid")
	censusIDInt, err := strconv.Atoi(censusIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	censusID := uint64(censusIDInt)

	var d AddKeysReq
	err = c.ShouldBindJSON(&d)
	if err != nil {
		returnErr(c, err)
		return
	}

	go a.cb.AddPublicKeysAndStoreError(censusID, d.PublicKeys, d.Weights)

	c.JSON(http.StatusOK, censusID)
}

func (a *API) postCloseCensus(c *gin.Context) {
	censusIDStr := c.Param("censusid")
	censusIDInt, err := strconv.Atoi(censusIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	censusID := uint64(censusIDInt)

	if err = a.cb.CloseCensus(censusID); err != nil {
		returnErr(c, err)
		return
	}
	root, err := a.cb.CensusRoot(censusID)
	if err != nil {
		returnErr(c, err)
		return
	}
	log.Debugf("[CensusID=%d] closed. Root: %x", censusID, root)
	c.JSON(http.StatusOK, hex.EncodeToString(root))
}

func (a *API) getCensus(c *gin.Context) {
	censusIDStr := c.Param("censusid")
	censusID, err := strconv.Atoi(censusIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	censusInfo, err := a.cb.CensusInfo(uint64(censusID))
	if err != nil {
		returnErr(c, err)
		return
	}
	c.JSON(http.StatusOK, censusInfo)
}

func (a *API) getMerkleProofHandler(c *gin.Context) {
	censusIDStr := c.Param("censusid")
	censusIDInt, err := strconv.Atoi(censusIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	censusID := uint64(censusIDInt)

	pubK, err := types.HexToPublicKey(c.Param("pubkey"))
	if err != nil {
		returnErr(c, err)
		return
	}

	// check if census is closed
	if _, err := a.cb.CensusRoot(censusID); err != nil {
		returnErr(c, err)
		return
	}

	// get MerkleProof
	index, weight, proof, err := a.cb.GetProof(censusID, pubK)
	if err != nil {
		returnErr(c, err)
		return
	}
	// PublicKey not returned, as is already known by the user
	c.JSON(http.StatusOK,
		types.CensusProof{Index: index, PublicKey: pubK, Weight: weight, MerkleProof: proof})
}

func (a *API) postVote(c *gin.Context) {
	processIDStr := c.Param("processid")
	processIDInt, err := strconv.Atoi(processIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	processID := uint64(processIDInt)

	var vote types.VotePackage
	err = c.ShouldBindJSON(&vote)
	if err != nil {
		returnErr(c, err)
		return
	}

	err = a.va.AddVote(processID, vote)
	if err != nil {
		returnErr(c, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (a *API) getProcess(c *gin.Context) {
	processIDStr := c.Param("processid")
	processID, err := strconv.Atoi(processIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	processInfo, err := a.va.ProcessInfo(uint64(processID))
	if err != nil {
		returnErr(c, err)
		return
	}
	c.JSON(http.StatusOK, processInfo)
}

func (a *API) postGenProof(c *gin.Context) {
	processIDStr := c.Param("processid")
	processIDInt, err := strconv.Atoi(processIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	processID := uint64(processIDInt)

	// trigger proof generation
	err = a.va.GenerateProof(processID)
	if err != nil {
		returnErr(c, err)
		return
	}

	c.JSON(http.StatusOK, "proof generation started")
}

func (a *API) getProof(c *gin.Context) {
	processIDStr := c.Param("processid")
	processIDInt, err := strconv.Atoi(processIDStr)
	if err != nil {
		returnErr(c, err)
		return
	}
	processID := uint64(processIDInt)

	// return proof if ready, if not return message saying that is not
	// generated yet
	proof, err := a.va.GetProof(processID)
	if err != nil {
		returnErr(c, err)
		return
	}
	c.JSON(http.StatusOK, proof)
}
