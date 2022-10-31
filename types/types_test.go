package types

import (
	"encoding/json"
	"math/big"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
)

func TestVerifyCensusProof(t *testing.T) {
	c := qt.New(t)

	optsDB := db.Options{Path: c.TempDir()}
	database, err := pebbledb.New(optsDB)
	c.Assert(err, qt.IsNil)

	arboConfig := arbo.Config{
		Database:     database,
		MaxLevels:    MaxLevels,
		HashFunction: arbo.HashFunctionPoseidon,
	}
	tree, err := arbo.NewTree(arboConfig)
	c.Assert(err, qt.IsNil)

	// create key
	sk := babyjub.NewRandPrivKey()
	pubK := sk.Public()
	weight := big.NewInt(1)

	index := uint64(1)
	indexBytes := Uint64ToIndex(index)
	value, err := HashPubKBytes(pubK, weight)
	c.Assert(err, qt.IsNil)

	err = tree.Add(indexBytes, value)
	c.Assert(err, qt.IsNil)

	_, _, proof, _, err := tree.GenProof(indexBytes)
	c.Assert(err, qt.IsNil)

	chainID := uint64(3)
	processID := uint64(123)

	vote := []byte("votetest")
	msgToSign, err := HashVote(chainID, processID, vote)
	c.Assert(err, qt.IsNil)
	sig := sk.SignPoseidon(msgToSign)

	vp := VotePackage{
		Signature: sig.Compress(),
		CensusProof: CensusProof{
			Index:       index,
			PublicKey:   pubK,
			Weight:      weight,
			MerkleProof: proof,
		},
		Vote: vote,
	}

	root, err := tree.Root()
	c.Assert(err, qt.IsNil)

	c.Assert(vp.verifySignature(chainID, processID), qt.IsNil)
	c.Assert(vp.verifyMerkleProof(root), qt.IsNil)
	c.Assert(vp.Verify(chainID, processID, root), qt.IsNil)

	vp.CensusProof.Index++
	c.Assert(vp.verifySignature(chainID, processID), qt.IsNil)
	c.Assert(vp.verifyMerkleProof(root), qt.Not(qt.IsNil))
	c.Assert(vp.Verify(chainID, processID, root), qt.Not(qt.IsNil))
}

func TestByteArrayJSON(t *testing.T) {
	c := qt.New(t)
	var b, b2 ByteArray

	// with nil value
	b = nil
	j, err := json.Marshal(b)
	c.Assert(err, qt.IsNil)
	c.Assert(string(j), qt.Equals, `""`)

	err = json.Unmarshal(j, &b2)
	c.Assert(err, qt.IsNil)
	c.Assert(b2, qt.DeepEquals, ByteArray{})

	// with empty array value
	b = []byte{}
	j, err = json.Marshal(b)
	c.Assert(err, qt.IsNil)
	c.Assert(string(j), qt.Equals, `""`)

	err = json.Unmarshal(j, &b2)
	c.Assert(err, qt.IsNil)
	c.Assert(b2, qt.DeepEquals, b)

	// with some value
	b = []byte{1, 2, 3, 253, 254, 255}
	j, err = json.Marshal(b)
	c.Assert(err, qt.IsNil)
	c.Assert(string(j), qt.Equals, `"010203fdfeff"`)

	err = json.Unmarshal(j, &b2)
	c.Assert(err, qt.IsNil)
	c.Assert(b2, qt.DeepEquals, b)
}

func TestVotePackageJSON(t *testing.T) {
	c := qt.New(t)

	optsDB := db.Options{Path: c.TempDir()}
	database, err := pebbledb.New(optsDB)
	c.Assert(err, qt.IsNil)

	arboConfig := arbo.Config{
		Database:     database,
		MaxLevels:    MaxLevels,
		HashFunction: arbo.HashFunctionPoseidon,
	}
	tree, err := arbo.NewTree(arboConfig)
	c.Assert(err, qt.IsNil)

	// create key (deterministic)
	var sk babyjub.PrivateKey
	pubK := sk.Public()
	weight := big.NewInt(1)

	index := uint64(1)
	indexBytes := Uint64ToIndex(index)
	value, err := HashPubKBytes(pubK, weight)
	c.Assert(err, qt.IsNil)

	err = tree.Add(indexBytes, value)
	c.Assert(err, qt.IsNil)

	_, _, proof, _, err := tree.GenProof(indexBytes)
	c.Assert(err, qt.IsNil)

	vote := []byte("votetest")
	voteBI := arbo.BytesToBigInt(vote)
	sig := sk.SignPoseidon(voteBI)

	vp := VotePackage{
		Signature: sig.Compress(),
		CensusProof: CensusProof{
			Index:       index,
			PublicKey:   pubK,
			Weight:      weight,
			MerkleProof: proof,
		},
		Vote: vote,
	}

	j, err := json.Marshal(vp.CensusProof)
	c.Assert(err, qt.IsNil)
	c.Assert(string(j), qt.Equals,
		`{"index":1,"publicKey":"91f1095ac019b50610b5cb56e5db3889177fee`+
			`8b6422fca3dac04ee1932431a9","weight":1,"merkleProof":"04000000"}`)

	var cp2 CensusProof
	err = json.Unmarshal(j, &cp2)
	c.Assert(err, qt.IsNil)
	c.Assert(cp2.Index, qt.Equals, vp.CensusProof.Index)
	c.Assert(cp2.PublicKey.String(), qt.Equals, vp.CensusProof.PublicKey.String())
	c.Assert(cp2.Weight.String(), qt.Equals, vp.CensusProof.Weight.String())
	c.Assert(cp2.MerkleProof, qt.DeepEquals, vp.CensusProof.MerkleProof)

	j, err = json.Marshal(vp)
	c.Assert(err, qt.IsNil)
	c.Assert(string(j), qt.Equals,
		`{"signature":"1d139ecb66c5bc6561b6780fdd6308db2bd836722626f8fd`+
			`99cea5542d9af79d8746b26ad71511fa7eaaac59c759ecabb16032`+
			`7f7ed826747c7566be0dac3402","censusProof":{"index":1,"`+
			`publicKey":"91f1095ac019b50610b5cb56e5db3889177fee8b64`+
			`22fca3dac04ee1932431a9","weight":1,"merkleProof":"0400`+
			`0000"},"vote":"766f746574657374"}`)

	var vp2 VotePackage
	err = json.Unmarshal(j, &vp2)
	c.Assert(err, qt.IsNil)
	c.Assert(vp2.Signature, qt.Equals, vp.Signature)
	c.Assert(vp2.CensusProof.Index, qt.Equals, vp.CensusProof.Index)
	c.Assert(vp2.CensusProof.PublicKey.String(), qt.Equals, vp.CensusProof.PublicKey.String())
	c.Assert(vp2.CensusProof.Weight.String(), qt.Equals, vp.CensusProof.Weight.String())
	c.Assert(vp2.CensusProof.MerkleProof, qt.DeepEquals, vp.CensusProof.MerkleProof)

	c.Assert(vp2.Vote, qt.DeepEquals, vp.Vote)
}

func TestIndexAndWeightParser(t *testing.T) {
	c := qt.New(t)

	index := uint64(1234)
	weight := big.NewInt(987654321)

	b := IndexAndWeightToBytes(index, weight)
	i2, w2, err := BytesToIndexAndWeight(b)
	c.Assert(err, qt.IsNil)
	c.Assert(i2, qt.Equals, index)
	c.Assert(weight.String(), qt.Equals, w2.String())
}
