package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/groupoidlabs/ovote-node/types"
	_ "github.com/mattn/go-sqlite3"
)

func TestStoreProcess(t *testing.T) {
	c := qt.New(t)

	db, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(db)

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
	minPositiveVotes := uint8(60)
	typ := uint8(1)

	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	// try to store the same processID, expecting error
	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "UNIQUE constraint failed: processes.id")

	// try to store the a different processID, but the same censusRoot,
	// expecting no error
	err = sqlite.StoreProcess(processID+1, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	process, err := sqlite.ReadProcessByID(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(process.ID, qt.Equals, processID)
	c.Assert(process.CensusRoot, qt.DeepEquals, censusRoot)
	c.Assert(process.EthBlockNum, qt.Equals, ethBlockNum)

	// read the stored votes
	processes, err := sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 2)
	c.Assert(processes[0].ID, qt.Equals, processID)
	c.Assert(processes[0].CensusRoot, qt.DeepEquals, censusRoot)
	c.Assert(processes[0].EthBlockNum, qt.Equals, ethBlockNum)
}

func TestProcessStatus(t *testing.T) {
	c := qt.New(t)

	db, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(db)

	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	// prepare the process
	processID := uint64(123)
	censusRoot := []byte("censusRoot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	resPubStartBlock := uint64(20)
	resPubWindow := uint64(20)
	minParticipation := uint8(60)
	minPositiveVotes := uint8(20)
	typ := uint8(1)

	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	status, err := sqlite.GetProcessStatus(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(status, qt.Equals, types.ProcessStatusOn)

	// update status to ProofGen
	err = sqlite.UpdateProcessStatus(processID, types.ProcessStatusProofGenerating)
	c.Assert(err, qt.IsNil)

	status, err = sqlite.GetProcessStatus(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(status, qt.Equals, types.ProcessStatusProofGenerating)

	// update status to Finished
	err = sqlite.UpdateProcessStatus(processID, types.ProcessStatusProofGenerated)
	c.Assert(err, qt.IsNil)

	status, err = sqlite.GetProcessStatus(processID)
	c.Assert(err, qt.IsNil)
	c.Assert(status, qt.Equals, types.ProcessStatusProofGenerated)
}

func TestProcessesByStatus(t *testing.T) {
	c := qt.New(t)

	db, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(db)

	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	censusRoot := []byte("censusRoot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	resPubStartBlock := uint64(20)
	resPubWindow := uint64(20)
	minParticipation := uint8(60)
	minPositiveVotes := uint8(20)
	typ := uint8(1)

	for i := 0; i < 10; i++ {
		err = sqlite.StoreProcess(uint64(i), censusRoot, censusSize,
			ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
			minPositiveVotes, typ)
		c.Assert(err, qt.IsNil)
	}

	processes, err := sqlite.ReadProcessesByStatus(types.ProcessStatusOn)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 10)

	// update status to Closed for the first 4 processes
	for i := 0; i < 4; i++ {
		err = sqlite.UpdateProcessStatus(uint64(i), types.ProcessStatusFrozen)
		c.Assert(err, qt.IsNil)
	}

	processes, err = sqlite.ReadProcessesByStatus(types.ProcessStatusOn)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 6)

	processes, err = sqlite.ReadProcessesByStatus(types.ProcessStatusFrozen)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 4)
}

func TestProcessByResPubStartBlock(t *testing.T) {
	c := qt.New(t)

	db, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(db)

	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	processID := uint64(123)
	censusRoot := []byte("censusRoot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	resPubStartBlock := uint64(20)
	resPubWindow := uint64(20)
	minParticipation := uint8(60)
	minPositiveVotes := uint8(20)
	typ := uint8(1)

	err = sqlite.StoreProcess(processID, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)
	err = sqlite.StoreProcess(processID+1, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)
	err = sqlite.StoreProcess(processID+2, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)
	err = sqlite.StoreProcess(processID+3, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)
	err = sqlite.StoreProcess(processID+4, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock+1, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)
	err = sqlite.StoreProcess(processID+5, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock+1, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	c.Assert(err, qt.IsNil)

	processes, err := sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 6)

	processes, err = sqlite.ReadProcessesByResPubStartBlock(resPubStartBlock)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 4)

	processes, err = sqlite.ReadProcessesByResPubStartBlock(resPubStartBlock + 1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 2)
}

func TestFrozeProcessesByCurrentBlockNum(t *testing.T) {
	c := qt.New(t)

	db, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := NewSQLite(db)

	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	processID := uint64(123)
	censusRoot := []byte("censusRoot")
	censusSize := uint64(100)
	ethBlockNum := uint64(10)
	resPubStartBlock := uint64(20)
	resPubWindow := uint64(20)
	minParticipation := uint8(60)
	minPositiveVotes := uint8(20)
	typ := uint8(1)

	// first, store few processes
	for i := 0; i < 10; i++ {
		err = sqlite.StoreProcess(processID+uint64(i), censusRoot,
			censusSize, ethBlockNum, resPubStartBlock+uint64(i), resPubWindow,
			minParticipation, minPositiveVotes, typ)
		c.Assert(err, qt.IsNil)
	}

	// expect 10 total processes
	processes, err := sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 10)

	// expect 10 processes with status = types.ProcessStatusOn
	processes, err = sqlite.ReadProcessesByStatus(types.ProcessStatusOn)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 10)

	// simulate that the blockchain advances up to block i0, i1, ..., i5 (6 in total)
	err = sqlite.FrozeProcessesByCurrentBlockNum(resPubStartBlock + 5)
	c.Assert(err, qt.IsNil)

	// expect 4 processes with status = types.ProcessStatusOn
	processes, err = sqlite.ReadProcessesByStatus(types.ProcessStatusOn)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 4)

	// expect 6 processes with status = types.ProcessStatusFrozen
	processes, err = sqlite.ReadProcessesByStatus(types.ProcessStatusFrozen)
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 6)
}
