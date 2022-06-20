package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestProof(t *testing.T) {
	c := qt.New(t)

	database, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(database)

	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	// prepare the votes
	processID := uint64(123)
	censusRoot := []byte("censusRoot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	resPubStartBlock := uint64(20)
	resPubWindow := uint64(20)
	minParticipation := uint8(20)
	typ := uint8(1)

	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation, typ)
	c.Assert(err, qt.IsNil)

	proof, err := sqlite.GetProofByProcessID(processID)
	c.Assert(err.Error(), qt.Equals, ErrProofNotInDB+", ProcessID: 123")
	c.Assert(proof, qt.IsNil)

	// expect no error, despite the ProofID is not stored yet
	err = sqlite.AddProofToProofID(processID, 42, []byte("testproof"), []byte("publicInputs"))
	c.Assert(err, qt.IsNil)

	proof, err = sqlite.GetProofByProcessID(processID)
	c.Assert(err.Error(), qt.Equals, ErrProofNotInDB+", ProcessID: 123")
	c.Assert(proof, qt.IsNil)

	err = sqlite.StoreProofID(processID, 42)
	c.Assert(err, qt.IsNil)

	proof, err = sqlite.GetProofByProcessID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(proof, qt.Not(qt.IsNil))
	c.Assert(proof.Proof, qt.DeepEquals, []byte{})
	c.Assert(proof.PublicInputs, qt.DeepEquals, []byte{})
	// expect proofAddedDatetime to not be set yet
	c.Assert(proof.ProofAddedDatetime, qt.Equals, time.Time{})

	err = sqlite.AddProofToProofID(processID, 42, []byte("testproof"), []byte("publicInputs"))
	c.Assert(err, qt.IsNil)

	proof, err = sqlite.GetProofByProcessID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(proof, qt.Not(qt.IsNil))
	c.Assert(proof.Proof, qt.DeepEquals, []byte("testproof"))
	c.Assert(proof.PublicInputs, qt.DeepEquals, []byte("publicInputs")) // no publicInputs yet
	c.Assert(proof.ProofAddedDatetime, qt.Not(qt.Equals), time.Time{})

	time.Sleep(1 * time.Second)

	// store a different proofID for the same processID
	err = sqlite.StoreProofID(processID, 43)
	c.Assert(err, qt.IsNil)

	// get the Proof for ProcessID, expect the proof to be the proofID=42
	proof, err = sqlite.GetProofByProcessID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(proof.ProofID, qt.Equals, uint64(42))

	// store the proofID=43 proof & inputs, and expect to get it when
	// getting the proof by processID
	err = sqlite.AddProofToProofID(processID, 43, []byte("testproof"), []byte("publicInputs"))
	c.Assert(err, qt.IsNil)
	proof, err = sqlite.GetProofByProcessID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(proof.ProofID, qt.Equals, uint64(43))

	proofs, err := sqlite.GetProofsByProcessID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(proofs[0].ProofID, qt.Equals, uint64(43))
	c.Assert(proofs[1].ProofID, qt.Equals, uint64(42))
}
