package api

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/gin-gonic/gin"
	"github.com/groupoidlabs/ovote-node/census"
	"github.com/groupoidlabs/ovote-node/censusbuilder"
	"github.com/groupoidlabs/ovote-node/db"
	"github.com/groupoidlabs/ovote-node/test"
	"github.com/groupoidlabs/ovote-node/types"
	"github.com/groupoidlabs/ovote-node/votesaggregator"
	"github.com/iden3/go-iden3-crypto/babyjub"
	_ "github.com/mattn/go-sqlite3"
	kvdb "go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
	"go.vocdoni.io/dvote/log"
)

func newTestAPI(c *qt.C, chainID uint64) (API, *db.SQLite) {
	// create the CensusBuilder
	opts := kvdb.Options{Path: c.TempDir()}
	database, err := pebbledb.New(opts)
	c.Assert(err, qt.IsNil)
	cb, err := censusbuilder.New(database, c.TempDir())
	c.Assert(err, qt.IsNil)
	r := gin.Default()

	// create the VotesAggregator
	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	va, err := votesaggregator.New(sqlite, chainID, nil)
	c.Assert(err, qt.IsNil)

	return API{r: r, cb: cb, va: va}, sqlite
}

func doPostNewCensus(c *qt.C, a API, pubKs []babyjub.PublicKey, weights []*big.Int) uint64 {
	reqData := newCensusReq{PublicKeys: pubKs, Weights: weights}
	jsonReqData, err := json.Marshal(reqData)
	c.Assert(err, qt.IsNil)

	req, err := http.NewRequest("POST", "/census", bytes.NewBuffer(jsonReqData))
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	// get the censusID from the response
	body, err := ioutil.ReadAll(w.Body)
	c.Assert(err, qt.IsNil)
	var censusID uint64
	err = json.Unmarshal(body, &censusID)
	c.Assert(err, qt.IsNil)
	return censusID
}

func doPostAddKeys(c *qt.C, a API, censusID uint64, pubKs []babyjub.PublicKey, weights []*big.Int) {
	censusIDStr := strconv.Itoa(int(censusID))
	reqData := newCensusReq{PublicKeys: pubKs, Weights: weights}
	jsonReqData, err := json.Marshal(reqData)
	c.Assert(err, qt.IsNil)
	req, err := http.NewRequest("POST", "/census/"+censusIDStr, bytes.NewBuffer(jsonReqData))
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)
}

func doPostCloseCensus(c *qt.C, a API, censusID uint64) []byte {
	censusIDStr := strconv.Itoa(int(censusID))
	req, err := http.NewRequest("POST", "/census/"+censusIDStr+"/close", nil)
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	body, err := ioutil.ReadAll(w.Body)
	c.Assert(err, qt.IsNil)
	var rootHex string
	err = json.Unmarshal(body, &rootHex)
	c.Assert(err, qt.IsNil)
	root, err := hex.DecodeString(rootHex)
	c.Assert(err, qt.IsNil)
	return root
}

func doGetProof(c *qt.C, a API, censusID uint64, pubK babyjub.PublicKey) types.CensusProof {
	censusIDStr := strconv.Itoa(int(censusID))
	pubKComp := pubK.Compress()
	pubKHex := hex.EncodeToString(pubKComp[:])

	req, err := http.NewRequest("GET", "/census/"+censusIDStr+"/merkleproof/"+pubKHex, nil)
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	body, err := ioutil.ReadAll(w.Body)
	c.Assert(err, qt.IsNil)
	var cp types.CensusProof
	err = json.Unmarshal(body, &cp)
	c.Assert(err, qt.IsNil)
	return cp
}

func doPostVote(c *qt.C, a API, processID uint64, vote types.VotePackage) {
	processIDStr := strconv.Itoa(int(processID))
	jsonReqData, err := json.Marshal(vote)
	c.Assert(err, qt.IsNil)
	req, err := http.NewRequest("POST", "/process/"+processIDStr, bytes.NewBuffer(jsonReqData))
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		fmt.Println("doPostVote Error:", w.Code, w.Body)
	}
	c.Assert(w.Code, qt.Equals, http.StatusOK)
}

func doGetProcess(c *qt.C, a API, processID uint64) types.Process {
	processIDStr := strconv.Itoa(int(processID))

	req, err := http.NewRequest("GET", "/process/"+processIDStr, nil)
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	body, err := ioutil.ReadAll(w.Body)
	c.Assert(err, qt.IsNil)
	var process types.Process
	err = json.Unmarshal(body, &process)
	c.Assert(err, qt.IsNil)
	return process
}

func prepareVotes(c *qt.C, chainID, processID uint64, keys test.Keys,
	proofs []types.CensusProof) []types.VotePackage {
	var votes []types.VotePackage
	for i := 0; i < len(keys.PrivateKeys); i++ {
		voteBytes := []byte("test")
		msgToSign, err := types.HashVote(chainID, processID, voteBytes)
		c.Assert(err, qt.IsNil)
		sigUncomp := keys.PrivateKeys[i].SignPoseidon(msgToSign)
		sig := sigUncomp.Compress()

		vote := types.VotePackage{
			Signature: sig,
			CensusProof: types.CensusProof{
				Index:       proofs[i].Index,
				PublicKey:   &keys.PublicKeys[i],
				MerkleProof: proofs[i].MerkleProof,
			},
			Vote: voteBytes,
		}
		votes = append(votes, vote)
	}
	return votes
}

func TestPostNewCensusHandler(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, _ := newTestAPI(c, chainID)
	a.r.POST("/census", a.postNewCensus)

	nKeys := 100
	// generate the publicKeys
	log.Debugf("Generating %d PublicKeys", nKeys)
	keys := test.GenUserKeys(nKeys)
	// create a new census with the keys
	doPostNewCensus(c, a, keys.PublicKeys, keys.Weights)
}

func TestPostAddKeysHandler(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, _ := newTestAPI(c, chainID)
	a.r.POST("/census", a.postNewCensus)
	a.r.POST("/census/:censusid", a.postAddKeys)

	nKeys := 150
	// generate the publicKeys
	log.Debugf("Generating %d PublicKeys", nKeys)
	keys := test.GenUserKeys(nKeys)

	// create a new census with the first 100 keys
	censusID := doPostNewCensus(c, a, keys.PublicKeys[:100], keys.Weights[:100])

	// Add the rest of the keys
	doPostAddKeys(c, a, censusID, keys.PublicKeys[100:], keys.Weights[100:])
}

func TestPostCloseCensusHandler(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, _ := newTestAPI(c, chainID)
	a.r.POST("/census", a.postNewCensus)
	a.r.POST("/census/:censusid", a.postAddKeys)
	a.r.POST("/census/:censusid/close", a.postCloseCensus)

	nKeys := 100
	// generate the publicKeys
	log.Debugf("Generating %d PublicKeys", nKeys)
	keys := test.GenUserKeys(nKeys)

	// create a new census with the first 100 keys
	censusID := doPostNewCensus(c, a, keys.PublicKeys, keys.Weights)

	time.Sleep(1 * time.Second)

	_ = doPostCloseCensus(c, a, censusID)
}

func TestGetProofHandler(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, _ := newTestAPI(c, chainID)
	a.r.POST("/census", a.postNewCensus)
	a.r.POST("/census/:censusid/close", a.postCloseCensus)
	a.r.GET("/census/:censusid/merkleproof/:pubkey", a.getMerkleProofHandler)

	nKeys := 100
	// generate the publicKeys
	log.Debugf("Generating %d PublicKeys", nKeys)
	keys := test.GenUserKeys(nKeys)

	// create a new census with the first 100 keys
	censusID := doPostNewCensus(c, a, keys.PublicKeys, keys.Weights)

	time.Sleep(1 * time.Second)

	censusRoot := doPostCloseCensus(c, a, censusID)

	for i := 0; i < nKeys; i++ {
		cp := doGetProof(c, a, censusID, keys.PublicKeys[i])
		// fmt.Printf("Index: %d, MerkleProof: %x\n", cp.Index, cp.MerkleProof)

		v, err := census.CheckProof(censusRoot, cp.MerkleProof, cp.Index,
			&keys.PublicKeys[i], keys.Weights[i])
		c.Assert(err, qt.IsNil)
		c.Assert(v, qt.IsTrue)
	}
}

func TestGetProcessInfo(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, sqlite := newTestAPI(c, chainID)
	a.r.GET("/process/:processid", a.getProcess)

	// simulate SmartContract Process creation, by adding the CensusRoot in
	// the votesaggregator db
	processID := uint64(123)
	censusRoot := []byte("testroot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	ethEndBlockNum := uint64(20)
	resultsPublishingWindow := uint64(20)
	minParticipation := uint8(20)
	minPositiveVotes := uint8(60)
	typ := uint8(1)
	err := sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, ethEndBlockNum, resultsPublishingWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	process := doGetProcess(c, a, processID)
	c.Assert(process.Status, qt.Equals, types.ProcessStatusOn)

	err = sqlite.UpdateProcessStatus(processID, types.ProcessStatusFrozen)
	c.Assert(err, qt.IsNil)

	process = doGetProcess(c, a, processID)
	c.Assert(process.Status, qt.Equals, types.ProcessStatusFrozen)
}

func TestBuildCensusAndPostVoteHandler(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, sqlite := newTestAPI(c, chainID)
	a.r.POST("/census", a.postNewCensus)
	a.r.POST("/census/:censusid/close", a.postCloseCensus)
	a.r.GET("/census/:censusid/merkleproof/:pubkey", a.getMerkleProofHandler)
	a.r.POST("/process/:processid", a.postVote)

	nKeys := 10
	// generate the publicKeys
	log.Debugf("Generating %d PublicKeys", nKeys)
	keys := test.GenUserKeys(nKeys)

	// create a new census with the first 100 keys
	censusID := doPostNewCensus(c, a, keys.PublicKeys, keys.Weights)

	time.Sleep(1 * time.Second)

	censusRoot := doPostCloseCensus(c, a, censusID)
	censusSize := uint64(len(keys.PublicKeys))

	var proofs []types.CensusProof
	for i := 0; i < nKeys; i++ {
		cp := doGetProof(c, a, censusID, keys.PublicKeys[i])
		v, err := census.CheckProof(censusRoot, cp.MerkleProof, cp.Index,
			&keys.PublicKeys[i], keys.Weights[i])
		c.Assert(err, qt.IsNil)
		c.Assert(v, qt.IsTrue)
		proofs = append(proofs, cp)
	}

	// simulate SmartContract Process creation, by adding the CensusRoot in
	// the votesaggregator db
	processID := uint64(123)
	ethBlockNum := uint64(10)
	ethEndBlockNum := uint64(20)
	resultsPublishingWindow := uint64(20)
	minParticipation := uint8(20)
	minPositiveVotes := uint8(60)
	typ := uint8(1)
	err := sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, ethEndBlockNum, resultsPublishingWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	// prepare the votes
	votes := prepareVotes(c, chainID, processID, keys, proofs)
	// send the votes
	for i := 0; i < len(votes); i++ {
		doPostVote(c, a, processID, votes[i])
	}
}

func TestPostVoteHandler(t *testing.T) {
	c := qt.New(t)

	chainID := uint64(3)
	a, sqlite := newTestAPI(c, chainID)
	a.r.POST("/process/:processid", a.postVote)
	a.r.GET("/process/:processid", a.getProcess)

	// generate the census without the API endpoints
	nKeys := 20
	log.Debugf("Generating %d PublicKeys", nKeys)
	keys := test.GenUserKeys(nKeys)
	cens := test.GenCensus(c, keys)
	// close the census & get root
	err := cens.Census.Close()
	c.Assert(err, qt.IsNil)
	censusRoot, err := cens.Census.Root()
	c.Assert(err, qt.IsNil)
	censusSize := uint64(nKeys)

	// prepare the votes
	processID := uint64(123)
	votes := test.GenVotes(c, cens, chainID, processID, 60)

	// simulate SmartContract Process creation, by adding the CensusRoot in
	// the votesaggregator db
	ethBlockNum := uint64(10)
	ethEndBlockNum := uint64(20)
	resultsPublishingWindow := uint64(20)
	minParticipation := uint8(20)
	minPositiveVotes := uint8(60)
	typ := uint8(1)
	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, ethEndBlockNum, resultsPublishingWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	// check that getting the process status by the API returns status=On
	process := doGetProcess(c, a, processID)
	c.Assert(process.Status, qt.Equals, types.ProcessStatusOn)

	// cast the votes except one
	for i := 0; i < nKeys-1; i++ {
		doPostVote(c, a, processID, votes[i])
	}

	// simulate that the ResPubStartBlock is reached and that the process
	// has ended
	err = sqlite.UpdateProcessStatus(processID, types.ProcessStatusFrozen)
	c.Assert(err, qt.IsNil)

	// check that getting the process status by the API returns status=Closed
	process = doGetProcess(c, a, processID)
	c.Assert(process.Status, qt.Equals, types.ProcessStatusFrozen)

	// try to cast the last vote, expecting error because the process is closed
	// doPostVote(c, a, processID, votes[nKeys-1])
	processIDStr := strconv.Itoa(int(processID))
	jsonReqData, err := json.Marshal(votes[nKeys-1])
	c.Assert(err, qt.IsNil)
	req, err := http.NewRequest("POST", "/process/"+processIDStr, bytes.NewBuffer(jsonReqData))
	c.Assert(err, qt.IsNil)
	w := httptest.NewRecorder()
	a.r.ServeHTTP(w, req)
	fmt.Println(w.Body)
	body, err := ioutil.ReadAll(w.Body)
	c.Assert(err, qt.IsNil)
	var msg errorMsg
	err = json.Unmarshal(body, &msg)
	c.Assert(err, qt.IsNil)
	c.Assert(msg.Message, qt.Equals,
		"process ResPubStartBlock (20) reached, votes can not be added")
}
