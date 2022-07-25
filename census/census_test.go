package census

import (
	"encoding/binary"
	"math"
	"math/big"
	"testing"

	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
)

// NOTE: most of the methods of Census are wrappers over
// https://github.com/vocdoni/arbo.  The proper tests are in arbo's repo, here
// there are tests that check the specific code of the Census package.

func newTestDB(c *qt.C) db.Database {
	var database db.Database
	var err error
	opts := db.Options{Path: c.TempDir()}
	database, err = pebbledb.New(opts)
	c.Assert(err, qt.IsNil)
	return database
}

func newTestCensus(c *qt.C) *Census {
	database := newTestDB(c)
	opts := Options{database}
	census, err := New(opts)
	c.Assert(err, qt.IsNil)
	return census
}

func TestNextIndex(t *testing.T) {
	c := qt.New(t)
	census := newTestCensus(c)

	// expect nextIndex to be 0
	rTx := census.db.ReadTx()
	i, err := census.getNextIndex(rTx)
	c.Assert(err, qt.IsNil)
	c.Assert(i, qt.Equals, uint64(0))
	rTx.Discard()

	// set the nextIndex to 10
	wTx := census.db.WriteTx()
	err = census.setNextIndex(wTx, 10)
	c.Assert(err, qt.IsNil)
	err = wTx.Commit()
	c.Assert(err, qt.IsNil)
	wTx.Discard()

	// expect nextIndex to be 10
	rTx = census.db.ReadTx()
	i, err = census.getNextIndex(rTx)
	c.Assert(err, qt.IsNil)
	c.Assert(i, qt.Equals, uint64(10))

	maxUint64 := uint64(math.MaxUint64)
	// set the nextIndex to maxUint64
	wTx = census.db.WriteTx()
	err = census.setNextIndex(wTx, maxUint64)
	c.Assert(err, qt.IsNil)
	err = wTx.Commit()
	c.Assert(err, qt.IsNil)
	wTx.Discard()

	// expect nextIndex to be maxUint64
	rTx = census.db.ReadTx()
	i, err = census.getNextIndex(rTx)
	c.Assert(err, qt.IsNil)
	c.Assert(i, qt.Equals, maxUint64)
}

func TestAddPublicKeys(t *testing.T) {
	c := qt.New(t)
	census := newTestCensus(c)

	nKeys := 100
	// generate the publicKeys
	var pubKs []babyjub.PublicKey
	var weights []*big.Int
	for i := 0; i < nKeys; i++ {
		sk := babyjub.NewRandPrivKey()
		pubK := sk.Public()
		pubKs = append(pubKs, *pubK)
		weights = append(weights, big.NewInt(1))
	}

	invalids, err := census.AddPublicKeys(pubKs, weights)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// expect nextIndex to be 100
	rTx := census.db.ReadTx()
	nextIndex, err := census.getNextIndex(rTx)
	c.Assert(err, qt.IsNil)
	c.Assert(nextIndex, qt.Equals, uint64(100))
	rTx.Discard()

	// generate more publicKeys
	for i := 0; i < nKeys/2; i++ {
		sk := babyjub.NewRandPrivKey()
		pubK := sk.Public()
		pubKs = append(pubKs, *pubK)
		weights = append(weights, big.NewInt(1))
	}

	// add the new publicKeys to the census
	invalids, err = census.AddPublicKeys(pubKs[nKeys:], weights[nKeys:])
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	// expect nextIndex to be 150
	rTx = census.db.ReadTx()
	nextIndex, err = census.getNextIndex(rTx)
	c.Assert(err, qt.IsNil)
	c.Assert(nextIndex, qt.Equals, uint64(150))
	rTx.Discard()

	// expect that the compressed publicKeys are stored with their
	// corresponding index in the db
	rTx = census.db.ReadTx()
	defer rTx.Discard()
	for i := 0; i < len(pubKs); i++ {
		pubK := pubKs[i].Compress()
		indexBytes, err := rTx.Get(pubK[:])
		c.Assert(err, qt.IsNil)
		index := binary.LittleEndian.Uint64(indexBytes)
		c.Assert(index, qt.Equals, uint64(i))
	}
}

func TestGetProofAndCheckMerkleProof(t *testing.T) {
	c := qt.New(t)
	census := newTestCensus(c)

	nKeys := 100
	// generate the publicKeys
	var pubKs []babyjub.PublicKey
	var weights []*big.Int
	for i := 0; i < nKeys; i++ {
		sk := babyjub.NewRandPrivKey()
		pubK := sk.Public()
		pubKs = append(pubKs, *pubK)
		weights = append(weights, big.NewInt(1))
	}

	invalids, err := census.AddPublicKeys(pubKs, weights)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	err = census.Close()
	c.Assert(err, qt.IsNil)
	root, err := census.tree.Root()
	c.Assert(err, qt.IsNil)

	for i := 0; i < nKeys; i++ {
		index, weight, proof, err := census.GetProof(&pubKs[i])
		c.Assert(err, qt.IsNil)
		c.Assert(weight.Bytes(), qt.DeepEquals, weights[i].Bytes())

		// check the proof using the CheckMerkleProof method
		v, err := CheckProof(root, proof, index, &pubKs[i], weights[i])
		c.Assert(err, qt.IsNil)
		c.Assert(v, qt.IsTrue)

		// check the proof using directly using arbo's method
		// indexBytes := arbo.BigIntToBytes(maxKeyLen, big.NewInt(int64(index))) //nolint:gomnd
		indexBytes := types.Uint64ToIndex(index)
		hashPubK, err := types.HashPubKBytes(&pubKs[i], weights[i])
		c.Assert(err, qt.IsNil)

		v, err = arbo.CheckProof(arbo.HashFunctionPoseidon, indexBytes, hashPubK, root, proof)
		c.Assert(err, qt.IsNil)
		c.Assert(v, qt.IsTrue)
	}
}

func TestInfo(t *testing.T) {
	c := qt.New(t)

	census := newTestCensus(c)

	nKeys := 100
	// generate the publicKeys
	var pubKs []babyjub.PublicKey
	var weights []*big.Int
	for i := 0; i < nKeys; i++ {
		sk := babyjub.NewRandPrivKey()
		pubK := sk.Public()
		pubKs = append(pubKs, *pubK)
		weights = append(weights, big.NewInt(1))
	}

	invalids, err := census.AddPublicKeys(pubKs, weights)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invalids), qt.Equals, 0)

	ci, err := census.Info()
	c.Assert(err, qt.IsNil)

	c.Assert(ci.ErrMsg, qt.Equals, "")
	c.Assert(ci.Size, qt.Equals, uint64(100))
	c.Assert(ci.Closed, qt.IsFalse)
	c.Assert(ci.Root, qt.DeepEquals, types.EmptyRoot)

	err = census.Close()
	c.Assert(err, qt.IsNil)

	root, err := census.Root()
	c.Assert(err, qt.IsNil)

	ci, err = census.Info()
	c.Assert(err, qt.IsNil)

	c.Assert(ci.ErrMsg, qt.Equals, "")
	c.Assert(ci.Size, qt.Equals, uint64(100))
	c.Assert(ci.Closed, qt.IsTrue)
	c.Assert(ci.Root, qt.DeepEquals, root)
}
