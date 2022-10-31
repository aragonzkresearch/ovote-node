package integration

import (
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/aragonzkresearch/ovote-node/db"
	"github.com/aragonzkresearch/ovote-node/test"
	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	_ "github.com/mattn/go-sqlite3"
	"github.com/vocdoni/arbo"
)

// Usage:
// 1. Run the node locally
// 2. Exec: go test --integration

var nodeURL = "http://127.0.0.1:8080"
var active bool

func init() {
	flag.BoolVar(&active, "integration", false, "integration test activated")
}

// TestFlowCensusCreation needs a node running,
// eg: cmd/ovote-node> go run cmd.go -c -l=debug
func TestFlowCensusCreation(t *testing.T) {
	if !active {
		t.Skip()
	}
	c := qt.New(t)

	nVotes := 3
	censusID, root, keys, censusProofs := testFlowCensusCreation(c, nodeURL, nVotes)
	fmt.Println("CensusID (in node):", censusID)
	fmt.Println("Root:", hex.EncodeToString(root))
	fmt.Println("Keys:", len(keys.PublicKeys))
	fmt.Printf("CensusProofs: %#v\n", censusProofs)
}

func testFlowCensusCreation(c *qt.C, nodeURL string, nVotes int) (uint64,
	[]byte, test.Keys, []types.CensusProof) {
	// - 1. create keys
	keys := test.GenUserKeys(nVotes)

	// - 2. API: post new census (build census)
	censusID := doPostNewCensus(c, nodeURL, keys.PublicKeys, keys.Weights)

	// - 	2.1. API: Close census
	root := doPostCloseCensus(c, nodeURL, censusID)

	// - 3. API: get merkleproofs for each pubkey
	censusProofs := make([]types.CensusProof, nVotes)
	for i := 0; i < nVotes; i++ {
		censusProofs[i] = doGetCensusProof(c, nodeURL, censusID, keys.PublicKeys[i])
	}
	return censusID, root, keys, censusProofs
}

func TestFlowVoting(t *testing.T) {
	if !active {
		t.Skip()
	}
	c := qt.New(t)

	// instantiate the db
	home, err := os.UserHomeDir()
	c.Assert(err, qt.IsNil)
	path := filepath.Join(home, ".ovote-node")
	sqlDB, err := sql.Open("sqlite3", filepath.Join(path, "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)
	database := db.NewSQLite(sqlDB)
	err = database.Migrate()
	c.Assert(err, qt.IsNil)

	chainID := uint64(5) // goerli
	// use a different processID if reusing the same node db than previous tests
	processID := uint64(101)
	fmt.Println("Ensure to be using the same ChainID than in the node. ChainID:", chainID)
	fmt.Println("Ensure to be using a ProcessID not used yet in the used"+
		"contract. ProcessID:", processID)

	nVotes := 10

	// - 0. reproduce the steps of census creation
	_, root, keys, censusProofs := testFlowCensusCreation(c, nodeURL, nVotes)

	// - 1. Create new process (not through the node, through the contract)
	// simulate that has been created through the contract, by inserting the process in the DB
	lastSyncBlockNum, err := database.GetLastSyncBlockNum()
	fmt.Println("LastSyncBlockNum", lastSyncBlockNum)
	c.Assert(err, qt.IsNil)

	blockNumber := lastSyncBlockNum
	resPubStartBlock := lastSyncBlockNum
	resPubWindow := uint64(20)
	err = database.StoreProcess(processID, root, uint64(nVotes),
		blockNumber, resPubStartBlock, resPubWindow,
		10, 1 /* referendum type */)
	c.Assert(err, qt.IsNil)

	// - 2. get process info (in the real flow would be through the
	// contract, here we get it directly from the db assuming that the eth
	// client already stored it there)

	err = database.UpdateProcessStatus(processID, types.ProcessStatusOn)
	c.Assert(err, qt.IsNil)

	process, err := database.ReadProcessByID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(process.CensusRoot, qt.DeepEquals, root)

	// - 3. Users vote
	// - 3.1. Users get CensusProof from the node (done at step 0)
	// - 3.2. Users sign vote
	votePackages := make([]types.VotePackage, nVotes)
	nAgree := (nVotes / 2) + 1
	for i := 0; i < nVotes; i++ {
		v := big.NewInt(0)
		if i < nAgree {
			v = big.NewInt(1)
		}
		voteBytes := arbo.BigIntToBytes(types.HashLen, v)
		voteHash, err := types.HashVote(chainID, processID, voteBytes)
		c.Assert(err, qt.IsNil)
		sig := keys.PrivateKeys[i].SignPoseidon(voteHash)
		votePackage := types.VotePackage{
			Signature:   sig.Compress(),
			CensusProof: censusProofs[i],
			Vote:        voteBytes,
		}
		votePackages[i] = votePackage
	}

	// - 3.3. Users send {vote + censusProof + sig} to the node
	for i := 0; i < nVotes; i++ {
		doPostVote(c, nodeURL, processID, votePackages[i])
	}

	// - 4. post GenProof
	// - 5. get proof
}
