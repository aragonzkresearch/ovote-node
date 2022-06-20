package eth

import (
	"github.com/groupoidlabs/ovote-node/db"
	"github.com/groupoidlabs/ovote-node/types"
)

// ensure that TestEthClient implements the eth.ClientInterf interface
var _ ClientInterf = (*TestEthClient)(nil)

// TestEthClient simulates an EthReader for testing purposes
type TestEthClient struct {
	db              *db.SQLite
	currentBlock    uint64
	blocksWithEvent map[uint64][]TestEvent
}

// TestEvent is used to simulate creation of new processes in the SmartContract
type TestEvent types.Process

// NewTestEthClient returns a new TestEthClient with the given configuration
func NewTestEthClient(sqlite *db.SQLite, startBlock uint64,
	blocks map[uint64][]TestEvent) *TestEthClient {
	return &TestEthClient{db: sqlite, currentBlock: startBlock, blocksWithEvent: blocks}
}

// AdvanceBlock simulates the advance of the EthBlockNum in the TestEthClient
func (e *TestEthClient) AdvanceBlock() error {
	e.currentBlock++
	events, ok := e.blocksWithEvent[e.currentBlock]
	if ok {
		// simulate event from the SmartContract, and store the process
		// into the db
		for i := 0; i < len(events); i++ {
			err := e.db.StoreProcess(events[i].ID,
				events[i].CensusRoot, events[i].CensusSize,
				e.currentBlock, events[i].ResPubStartBlock,
				events[i].ResPubWindow, events[i].MinParticipation,
				events[i].Type)
			if err != nil {
				return err
			}
		}
	}

	// TODO better to do this in a single SQL query (get & update)
	// get processes that end at this block and update their status
	processes, err := e.db.ReadProcessesByResPubStartBlock(e.currentBlock)
	if err != nil {
		return err
	}
	for i := 0; i < len(processes); i++ {
		err = e.db.UpdateProcessStatus(processes[i].ID, types.ProcessStatusFrozen)
		if err != nil {
			return err
		}
	}

	return nil
}

// Start implements the EthReader.Start method of the interface
func (e *TestEthClient) Start(fromBlock uint64) error {
	return nil
}
