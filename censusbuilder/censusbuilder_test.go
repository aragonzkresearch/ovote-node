package censusbuilder

import (
	"testing"

	"github.com/aragonzkresearch/ovote-node/census"
	"github.com/aragonzkresearch/ovote-node/test"
	qt "github.com/frankban/quicktest"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
)

func newTestDB(c *qt.C) db.Database {
	var database db.Database
	var err error
	opts := db.Options{Path: c.TempDir()}
	database, err = pebbledb.New(opts)
	c.Assert(err, qt.IsNil)
	return database
}

func TestNewCensus(t *testing.T) {
	c := qt.New(t)

	// create the CensusBuilder
	database := newTestDB(c)
	cb, err := New(database, c.TempDir())
	c.Assert(err, qt.IsNil)

	censusID1, err := cb.NewCensus()
	c.Assert(err, qt.IsNil)
	err = cb.CloseCensus(censusID1)
	c.Assert(err, qt.IsNil)

	c.Assert(censusID1, qt.Equals, uint64(0))

	_, err = cb.CensusRoot(censusID1)
	c.Assert(err, qt.IsNil)

	censusID2, err := cb.NewCensus()
	c.Assert(err, qt.IsNil)
	c.Assert(censusID1, qt.Equals, uint64(0))

	_, err = cb.CensusRoot(censusID2)
	c.Assert(err.Error(), qt.Equals, "Can not get the CensusRoot, Census not closed yet")
	_, err = cb.censuses[censusID2].IntermediateRoot()
	c.Assert(err, qt.IsNil)

	err = cb.CloseCensus(censusID2)
	c.Assert(err, qt.IsNil)
}

func TestAddPublicKeys(t *testing.T) {
	c := qt.New(t)

	nKeys := 100
	// generate the publicKeys
	keys := test.GenUserKeys(nKeys)

	// create the CensusBuilder
	database := newTestDB(c)
	cb, err := New(database, c.TempDir())
	c.Assert(err, qt.IsNil)

	censusID1, err := cb.NewCensus()
	c.Assert(err, qt.IsNil)
	err = cb.AddPublicKeys(censusID1, keys.PublicKeys, keys.Weights)
	c.Assert(err, qt.IsNil)
	err = cb.CloseCensus(censusID1)
	c.Assert(err, qt.IsNil)

	root1, err := cb.CensusRoot(censusID1)
	c.Assert(err, qt.IsNil)

	// create a 2nd Census, with the same pubKs than the 1st one
	censusID2, err := cb.NewCensus()
	c.Assert(err, qt.IsNil)
	err = cb.AddPublicKeys(censusID2, keys.PublicKeys, keys.Weights)
	c.Assert(err, qt.IsNil)

	_, err = cb.CensusRoot(censusID2)
	c.Assert(err.Error(), qt.Equals, "Can not get the CensusRoot, Census not closed yet")
	root2, err := cb.censuses[censusID2].IntermediateRoot()
	c.Assert(err, qt.IsNil)

	// check that both roots are equal
	c.Assert(root2, qt.DeepEquals, root1)

	// create new pubKs
	keys2 := test.GenUserKeys(nKeys)
	err = cb.AddPublicKeys(censusID2, keys2.PublicKeys, keys2.Weights)
	c.Assert(err, qt.IsNil)

	err = cb.CloseCensus(censusID2)
	c.Assert(err, qt.IsNil)

	root2, err = cb.CensusRoot(censusID2)
	c.Assert(err, qt.IsNil)
	c.Assert(root2, qt.Not(qt.DeepEquals), root1)
}

func TestGetProof(t *testing.T) {
	c := qt.New(t)

	nKeys := 100
	// generate the publicKeys
	keys := test.GenUserKeys(nKeys)

	// create the CensusBuilder
	database := newTestDB(c)
	cb, err := New(database, c.TempDir())
	c.Assert(err, qt.IsNil)

	censusID, err := cb.NewCensus()
	c.Assert(err, qt.IsNil)
	err = cb.AddPublicKeys(censusID, keys.PublicKeys, keys.Weights)
	c.Assert(err, qt.IsNil)
	err = cb.CloseCensus(censusID)
	c.Assert(err, qt.IsNil)

	root, err := cb.CensusRoot(censusID)
	c.Assert(err, qt.IsNil)

	for i := 0; i < nKeys; i++ {
		index, _, proof, err := cb.GetProof(censusID, &keys.PublicKeys[i])
		c.Assert(err, qt.IsNil)
		c.Assert(index, qt.Equals, uint64(i))

		v, err := census.CheckProof(root, proof, index, &keys.PublicKeys[i], keys.Weights[i])
		c.Assert(err, qt.IsNil)
		c.Assert(v, qt.IsTrue)
	}
}

func TestCensusInfo(t *testing.T) {
	c := qt.New(t)

	nKeys := 100
	// generate the publicKeys
	keys := test.GenUserKeys(nKeys)

	// create the CensusBuilder
	database := newTestDB(c)
	cb, err := New(database, c.TempDir())
	c.Assert(err, qt.IsNil)

	censusID, err := cb.NewCensus()
	c.Assert(err, qt.IsNil)
	err = cb.AddPublicKeys(censusID, keys.PublicKeys, keys.Weights)
	c.Assert(err, qt.IsNil)

	ci, err := cb.CensusInfo(censusID)
	c.Assert(err, qt.IsNil)

	emptyRoot := make([]byte, arbo.HashFunctionPoseidon.Len())

	c.Assert(ci.ErrMsg, qt.Equals, "")
	c.Assert(ci.Size, qt.Equals, uint64(100))
	c.Assert(ci.Closed, qt.IsFalse)
	c.Assert(ci.Root, qt.DeepEquals, emptyRoot)

	err = cb.CloseCensus(censusID)
	c.Assert(err, qt.IsNil)

	root, err := cb.CensusRoot(censusID)
	c.Assert(err, qt.IsNil)

	ci, err = cb.CensusInfo(censusID)
	c.Assert(err, qt.IsNil)

	c.Assert(ci.ErrMsg, qt.Equals, "")
	c.Assert(ci.Size, qt.Equals, uint64(100))
	c.Assert(ci.Closed, qt.IsTrue)
	c.Assert(ci.Root, qt.DeepEquals, root)
}
