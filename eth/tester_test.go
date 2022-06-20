package eth

import (
	"database/sql"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/groupoidlabs/ovote-node/db"
	"github.com/groupoidlabs/ovote-node/types"
	_ "github.com/mattn/go-sqlite3"
)

func TestAdvanceBlock(t *testing.T) {
	c := qt.New(t)

	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	// WIP
	events := make(map[uint64][]TestEvent)
	events[1001] = []TestEvent{{
		ID:               1,
		CensusRoot:       []byte("root"),
		CensusSize:       100,
		ResPubStartBlock: 1010,
		ResPubWindow:     300,
		MinParticipation: 20,
	}}
	events[1002] = []TestEvent{{
		ID:               2,
		CensusRoot:       []byte("root"),
		CensusSize:       100,
		ResPubStartBlock: 1011,
		ResPubWindow:     300,
		MinParticipation: 20,
	}}
	eth := NewTestEthClient(sqlite, 1000, events)

	processes, err := sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 0)

	// advance block and expect 1 process in the db
	err = eth.AdvanceBlock()
	c.Assert(err, qt.IsNil)
	processes, err = sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 1)

	// advance block and expect 2 process in the db
	err = eth.AdvanceBlock()
	c.Assert(err, qt.IsNil)
	processes, err = sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 2)

	// check that the obtained processes match the introduced values
	c.Assert(processes[0].ID, qt.Equals, events[1001][0].ID)
	c.Assert(processes[0].CensusRoot, qt.DeepEquals, events[1001][0].CensusRoot)
	c.Assert(processes[0].CensusSize, qt.DeepEquals, events[1001][0].CensusSize)
	c.Assert(processes[0].ResPubStartBlock, qt.Equals, events[1001][0].ResPubStartBlock)
	c.Assert(processes[0].ResPubWindow, qt.Equals, events[1001][0].ResPubWindow)
	c.Assert(processes[0].MinParticipation, qt.Equals, events[1001][0].MinParticipation)
	c.Assert(processes[0].Status, qt.Equals, types.ProcessStatusOn)
	c.Assert(processes[1].ID, qt.Equals, events[1002][0].ID)
	c.Assert(processes[1].CensusRoot, qt.DeepEquals, events[1002][0].CensusRoot)
	c.Assert(processes[1].CensusSize, qt.DeepEquals, events[1002][0].CensusSize)
	c.Assert(processes[1].ResPubStartBlock, qt.Equals, events[1002][0].ResPubStartBlock)
	c.Assert(processes[1].ResPubWindow, qt.Equals, events[1002][0].ResPubWindow)
	c.Assert(processes[1].MinParticipation, qt.Equals, events[1002][0].MinParticipation)
	c.Assert(processes[1].Status, qt.Equals, types.ProcessStatusOn)

	// advance until block 1010, to check that process 0 Status has been updated
	for i := 0; i < 8; i++ {
		err = eth.AdvanceBlock()
		c.Assert(err, qt.IsNil)
	}
	processes, err = sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 2)
	c.Assert(processes[0].Status, qt.Equals, types.ProcessStatusFrozen)
	c.Assert(processes[1].Status, qt.Equals, types.ProcessStatusOn)

	// advance one block more, to check that now both processes has Status==Frozen
	err = eth.AdvanceBlock()
	c.Assert(err, qt.IsNil)
	processes, err = sqlite.ReadProcesses()
	c.Assert(err, qt.IsNil)
	c.Assert(len(processes), qt.Equals, 2)
	c.Assert(processes[0].Status, qt.Equals, types.ProcessStatusFrozen)
	c.Assert(processes[1].Status, qt.Equals, types.ProcessStatusFrozen)
}
