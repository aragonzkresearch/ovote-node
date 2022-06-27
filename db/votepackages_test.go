package db

import (
	"database/sql"
	"math/big"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/aragonzkresearch/ovote-node/test"
	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-iden3-crypto/babyjub"
	_ "github.com/mattn/go-sqlite3"
	"github.com/vocdoni/arbo"
)

func TestStoreAndReadVotes(t *testing.T) {
	c := qt.New(t)

	db, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(db)

	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	// try to store a vote for a processID that does not exist yet
	vote := []byte("test")
	voteBI := arbo.BytesToBigInt(vote)
	keys := test.GenUserKeys(1)
	sig := keys.PrivateKeys[0].SignPoseidon(voteBI)
	votePackage := types.VotePackage{
		Signature: sig.Compress(),
		CensusProof: types.CensusProof{
			Index:       1,
			PublicKey:   &keys.PublicKeys[0],
			Weight:      big.NewInt(1),
			MerkleProof: []byte("test"),
		},
		Vote: []byte("test"),
	}
	// expect error when storing the vote, as processID does not exist yet
	err = sqlite.StoreVotePackage(uint64(123), votePackage)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "Can not store VotePackage, ProcessID=123 does not exist")

	// store a processID in which the votes will be related
	processID := uint64(123)
	censusRoot := []byte("censusRoot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	resPubStartBlock := uint64(20)
	resPubWindow := uint64(20)
	minParticipation := uint8(60)
	typ := uint8(1)

	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation, typ)
	c.Assert(err, qt.IsNil)

	// prepare the votes
	nVotes := 10

	var votesAdded []types.VotePackage
	for i := 0; i < nVotes; i++ {
		voteBytes := []byte("test")
		voteBI := arbo.BytesToBigInt(voteBytes)
		sk := babyjub.NewRandPrivKey()
		pubK := sk.Public()
		sigUncomp := sk.SignPoseidon(voteBI)
		sig := sigUncomp.Compress()
		vote := types.VotePackage{
			Signature: sig,
			CensusProof: types.CensusProof{
				Index:       uint64(i),
				PublicKey:   pubK,
				Weight:      big.NewInt(1),
				MerkleProof: []byte("test" + strconv.Itoa(i)),
			},
			Vote: voteBytes,
		}
		votesAdded = append(votesAdded, vote)

		err = sqlite.StoreVotePackage(processID, vote)
		c.Assert(err, qt.IsNil)
	}

	// try to store a vote with already stored index
	err = sqlite.StoreVotePackage(processID, votesAdded[0])
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "UNIQUE constraint failed: votepackages.indx")

	// read the stored votes
	votes, err := sqlite.ReadVotePackagesByProcessID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(len(votes), qt.Equals, nVotes)
}
