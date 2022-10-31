package test

import (
	"fmt"
	"math"
	"math/big"

	"github.com/aragonzkresearch/ovote-node/census"
	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
)

// Keys contains the test PrivateKeys and test PublicKeys
type Keys struct {
	PrivateKeys []babyjub.PrivateKey
	PublicKeys  []babyjub.PublicKey
	Weights     []*big.Int
}

// Census contains the test Keys and census.Census
type Census struct {
	Keys   Keys
	Census *census.Census
}

// GenUserKeys returns n Keys
func GenUserKeys(nUsers int) Keys {
	var keys Keys
	for i := 0; i < nUsers; i++ {
		sk := babyjub.NewRandPrivKey()
		keys.PrivateKeys = append(keys.PrivateKeys, sk)
		keys.PublicKeys = append(keys.PublicKeys, *sk.Public())
		keys.Weights = append(keys.Weights, big.NewInt(1))
	}
	return keys
}

// GenCensus returns a test Census containing the given Keys
func GenCensus(c *qt.C, keys Keys) *Census {
	optsDB := db.Options{Path: c.TempDir()}
	database, err := pebbledb.New(optsDB)
	c.Assert(err, qt.IsNil)

	optsCensus := census.Options{DB: database}
	cens, err := census.New(optsCensus)
	c.Assert(err, qt.IsNil)

	invalids, err := cens.AddPublicKeys(keys.PublicKeys, keys.Weights)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)
	return &Census{Keys: keys, Census: cens}
}

// GenVotes generate the votes from the given Census
func GenVotes(c *qt.C, cens *Census, chainID, processID uint64, ratio int) []types.VotePackage {
	if ratio >= 100 { //nolint:gomnd
		panic(fmt.Errorf("ratio can not be >=100, ratio: %d", ratio))
	}
	var votes []types.VotePackage
	nPosVotes := int(math.Ceil(float64(len(cens.Keys.PrivateKeys)) *
		(float64(ratio) / 100))) //nolint:gomnd
	l := arbo.HashFunctionPoseidon.Len()
	for i := 0; i < len(cens.Keys.PrivateKeys); i++ {
		voteBytes := make([]byte, l)
		if i < nPosVotes {
			voteBytes = arbo.BigIntToBytes(l, big.NewInt(1))
		}
		msgToSign, err := types.HashVote(chainID, processID, voteBytes)
		c.Assert(err, qt.IsNil)
		sigUncomp := cens.Keys.PrivateKeys[i].SignPoseidon(msgToSign)
		sig := sigUncomp.Compress()

		// get merkleproof
		index, weight, proof, err := cens.Census.GetProof(&cens.Keys.PublicKeys[i])
		c.Assert(err, qt.IsNil)
		c.Assert(weight.Bytes(), qt.DeepEquals, cens.Keys.Weights[i].Bytes())

		vote := types.VotePackage{
			Signature: sig,
			CensusProof: types.CensusProof{
				Index:       index,
				PublicKey:   &cens.Keys.PublicKeys[i],
				Weight:      cens.Keys.Weights[i],
				MerkleProof: proof,
			},
			Vote: voteBytes,
		}
		votes = append(votes, vote)
	}
	return votes
}
