package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/aragonzkresearch/ovote-node/db"
	oTypes "github.com/aragonzkresearch/ovote-node/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.vocdoni.io/dvote/log"
)

const (
	// eventNewProcessLen defines the length of an event log of newProcess
	eventNewProcessLen = 288 // = 32*9
	// eventResultPublishedLen defines the length of an event log of
	// resultPublished
	eventResultPublishedLen = 160 // = 32*5
	// eventProcessClosedLen defines the length of an event log of
	// processClosed
	eventProcessClosedLen = 96 // = 32*3
)

// ClientInterf defines the interface that synchronizes with the Ethereum
// blockchain to obtain the processes data
type ClientInterf interface {
	// Sync scans the contract activity since the given fromBlock until the
	// current block, storing in the database all the updates on the
	// Processess
	Start(fromBlock uint64) error
}

// Client implements the ClientInterf that reads data from the Ethereum
// blockchain
type Client struct {
	client       *ethclient.Client
	db           *db.SQLite
	contractAddr common.Address
	ChainID      uint64
}

// Options is used to pass the parameters to load a new Client
type Options struct {
	EthURL       string
	SQLite       *db.SQLite
	ContractAddr common.Address
}

// New loads a new Client
func New(opts Options) (*Client, error) {
	client, err := ethclient.Dial(opts.EthURL)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	// get network ChainID
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	return &Client{
		client:       client,
		db:           opts.SQLite,
		contractAddr: opts.ContractAddr,
		ChainID:      chainID.Uint64(),
	}, nil
}

// Sync synchronizes the blocknums and events since the last synced block to
// the current one, and then live syncs the new ones
func (c *Client) Sync() error {
	// TODO WARNING:
	// Probably the logic will need to be changed to support reorgs of
	// chain. Maybe wait to sync blocks until some new blocks after the
	// block have been created.

	// get lastSyncBlockNum from db
	lastSyncBlockNum, err := c.db.GetLastSyncBlockNum()
	if err != nil {
		return err
	}

	// start live sync events (before synchronizing the history)
	go c.syncEventsLive() // nolint:errcheck

	// sync from lastSyncBlockNum until the current blocknum
	err = c.syncHistory(lastSyncBlockNum)
	if err != nil {
		return err
	}

	// live sync blocks
	err = c.syncBlocksLive()
	if err != nil {
		return err
	}
	return nil
}

// syncBlocksLive synchronizes live the ethereum blocks
func (c *Client) syncBlocksLive() error {
	// sync to new blocks
	headers := make(chan *types.Header)
	sub, err := c.client.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Error(err)
		return err
	}

	for {
		select {
		case err := <-sub.Err():
			log.Error(err)
		case header := <-headers:
			log.Debugf("new eth block received: %d", header.Number.Uint64())
			// store in db lastSyncBlockNum
			err = c.db.UpdateLastSyncBlockNum(header.Number.Uint64())
			if err != nil {
				log.Error(err)
			}
		}
	}
}

// syncEventsLive synchronizes live from the ovote contract events
func (c *Client) syncEventsLive() error {
	query := ethereum.FilterQuery{
		Addresses: []common.Address{c.contractAddr},
	}

	logs := make(chan types.Log)
	sub, err := c.client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Error(err)
		return err
	}

	for {
		select {
		case err := <-sub.Err():
			log.Error(err)
		case vLog := <-logs:
			err = c.processEventLog(vLog)
			if err != nil {
				log.Error(err)
			}
		}
	}
}

// syncHistory synchronizes from the ovote contract the events & blockNums
// from the given block to the current block height.
func (c *Client) syncHistory(startBlock uint64) error {
	header, err := c.client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Error(err)
		return err
	}
	currBlockNum := header.Number
	log.Debugf("[SyncHistory] blocks from: %d, to: %d", startBlock, currBlockNum)
	err = c.syncEventsHistory(big.NewInt(int64(startBlock)), currBlockNum)
	if err != nil {
		log.Error(err)
		return err
	}

	// update the processes which their ResPubStartBlock has been reached
	// (and that they were still in status ProcessStatusOn
	// TODO maybe do not froze process, and allow it to accept votes still
	// in results publishing phase
	err = c.db.FrozeProcessesByCurrentBlockNum(currBlockNum.Uint64())
	if err != nil {
		log.Error(err)
		return err
	}
	// TODO take into account chain reorgs: for currBlockNum, set to
	// ProcessStatusOn the processes with resPubStartBlock>currBlockNum
	return nil
}

// syncEventsHistory synchronizes from the ovote contract log events
// between the given startBlock and endBlock
func (c *Client) syncEventsHistory(startBlock, endBlock *big.Int) error {
	query := ethereum.FilterQuery{
		FromBlock: startBlock,
		ToBlock:   endBlock,
		Addresses: []common.Address{
			c.contractAddr,
		},
	}
	logs, err := c.client.FilterLogs(context.Background(), query)
	if err != nil {
		log.Error(err)
		return err
	}
	for i := 0; i < len(logs); i++ {
		err = c.processEventLog(logs[i])
		if err != nil {
			log.Error(err)
		}
	}

	return nil
}

func (c *Client) processEventLog(eventLog types.Log) error {
	// depending on eventLog.Data length, parse the different types of
	// event logs
	switch l := len(eventLog.Data); l {
	case eventNewProcessLen:
		e, err := parseEventNewProcess(eventLog.Data)
		if err != nil {
			return fmt.Errorf("blocknum: %d, error parsing event log"+
				" (newProcess): %x, err: %s",
				eventLog.BlockNumber, eventLog.Data, err)
		}
		log.Debugf("Event: (blocknum: %d) %s",
			eventLog.BlockNumber, e)
		// store the process in the db
		err = c.db.StoreProcess(e.ProcessID, e.CensusRoot[:], e.CensusSize,
			eventLog.BlockNumber, e.ResPubStartBlock, e.ResPubWindow,
			e.MinParticipation, e.Type)
		if err != nil {
			return fmt.Errorf("error storing new process: %x, err: %s",
				eventLog.Data, err)
		}
	case eventResultPublishedLen:
		e, err := parseEventResultPublished(eventLog.Data)
		if err != nil {
			return fmt.Errorf("blocknum: %d, error parsing event log"+
				" (resultPublished): %x, err: %s",
				eventLog.BlockNumber, eventLog.Data, err)
		}
		log.Debugf("Event: (blocknum: %d) %s",
			eventLog.BlockNumber, e)
	case eventProcessClosedLen:
		e, err := parseEventProcessClosed(eventLog.Data)
		if err != nil {
			return fmt.Errorf("blocknum: %d, error parsing event log"+
				" (processClosed): %x, err: %s",
				eventLog.BlockNumber, eventLog.Data, err)
		}
		log.Debugf("Event: (blocknum: %d) %s",
			eventLog.BlockNumber, e)
		// update process status in DB
		err = c.db.UpdateProcessStatus(e.ProcessID, oTypes.ProcessStatusContractClosed)
		if err != nil {
			return fmt.Errorf("error updating process status: %x, err: %s",
				eventLog.Data, err)
		}
	default:
		fmt.Printf("LOG in block %d:\n %x \n", eventLog.BlockNumber, eventLog.Data)
		return fmt.Errorf("unrecognized event log with length %d", l)
	}

	return nil
}
