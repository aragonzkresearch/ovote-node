package votesaggregator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aragonzkresearch/ovote-node/db"
	"github.com/aragonzkresearch/ovote-node/test"
	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	_ "github.com/mattn/go-sqlite3"
)

func baseTestVotesAggregator(c *qt.C, chainID, processID uint64, nVotes, ratio int) (
	*VotesAggregator, []types.VotePackage) {
	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	va, err := New(sqlite, chainID, nil)
	c.Assert(err, qt.IsNil)

	// prepare the census
	keys := test.GenUserKeys(nVotes)
	testCensus := test.GenCensus(c, keys)
	err = testCensus.Census.Close()
	c.Assert(err, qt.IsNil)

	censusRoot, err := testCensus.Census.Root()
	c.Assert(err, qt.IsNil)
	censusSize := uint64(len(keys.PublicKeys))
	votes := test.GenVotes(c, testCensus, chainID, processID, ratio)

	// store a process for the test
	ethBlockNum := uint64(10)
	ethEndBlockNum := uint64(20)
	resultsPublishingWindow := uint64(20)
	minParticipation := uint8(20)
	typ := uint8(1)
	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, ethEndBlockNum, resultsPublishingWindow, minParticipation,
		typ)
	c.Assert(err, qt.IsNil)

	return va, votes
}

func TestStoreAndReadVotes(t *testing.T) {
	c := qt.New(t)

	nVotes := 10
	chainID := uint64(3)
	processID := uint64(123)
	va, votes := baseTestVotesAggregator(c, chainID, processID, nVotes, 60)

	var err error
	for i := 0; i < len(votes); i++ {
		err = va.AddVote(processID, votes[i])
		c.Assert(err, qt.IsNil)
	}

	// try to store a vote with already stored index
	err = va.AddVote(processID, votes[0])
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "UNIQUE constraint failed: votepackages.indx")

	// try to store invalid merkleproofs
	votes[0].CensusProof.Index = 11
	err = va.AddVote(processID, votes[0])
	c.Assert(err.Error(), qt.Equals, "merkleproof verification failed")

	// try to store invalid merkleproofs
	votes[0].Vote = []byte("invalidvotecontent")
	err = va.AddVote(processID, votes[0])
	c.Assert(err.Error(), qt.Equals, "signature verification failed")
}

func TestGenerateZKInputs(t *testing.T) {
	c := qt.New(t)
	testGenerateZKInputs(c, 3, 3, 1, 60)
	testGenerateZKInputs(c, 3, 3, 3, 60)
	testGenerateZKInputs(c, 8, 4, 3, 60)
	testGenerateZKInputs(c, 8, 4, 5, 60)
	testGenerateZKInputs(c, 8, 4, 7, 60)
	testGenerateZKInputs(c, 8, 4, 8, 60)
	testGenerateZKInputs(c, 16, 4, 10, 60)
}

func testGenerateZKInputs(c *qt.C, nMaxVotes, nLevels, nVotes, ratio int) {
	chainID := uint64(3)
	processID := uint64(123)
	va, votes := baseTestVotesAggregator(c, chainID, processID, nVotes, ratio)

	var err error
	for i := 0; i < len(votes); i++ {
		err = va.AddVote(processID, votes[i])
		c.Assert(err, qt.IsNil)
	}

	zki, err := va.generateZKInputs(processID, nMaxVotes, nLevels)
	c.Assert(err, qt.IsNil)
	s, err := json.Marshal(zki)
	c.Assert(err, qt.IsNil)

	// fmt.Println(string(s))
	filename := fmt.Sprintf("compat-tests/zkinputs_%d_%d_%d_%d.json",
		nMaxVotes, nLevels, nVotes, ratio)
	err = ioutil.WriteFile(filename, s, 0600)
	c.Assert(err, qt.IsNil)
}
