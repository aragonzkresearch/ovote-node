package eth

import (
	"context"
	"database/sql"
	"flag"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	qt "github.com/frankban/quicktest"
	"github.com/groupoidlabs/ovote-node/db"
	"go.vocdoni.io/dvote/log"
)

var ethURL string
var contractAddr string
var startBlock uint64

func init() {
	// block: 6945416
	// contract addr: 0x79ea1cc5B8BFF0F46E1B98068727Fd02D8EB1aF3
	flag.StringVar(&ethURL, "ethurl", "", "eth provider url")
	flag.StringVar(&contractAddr, "addr", "", "OVOTE contract address")
	flag.Uint64Var(&startBlock, "block", 0, "eth block from which to start to sync")
}

func TestSyncHistory(t *testing.T) {
	if ethURL == "" || contractAddr == "" || startBlock == 0 {
		t.Skip()
	}

	c := qt.New(t)
	log.Init("debug", "stdout")

	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	addr := common.HexToAddress(contractAddr)
	client, err := New(Options{
		EthURL: ethURL,
		SQLite: sqlite, ContractAddr: addr})
	c.Assert(err, qt.IsNil)

	err = client.syncHistory(startBlock)
	c.Assert(err, qt.IsNil)
}

func TestSyncLive(t *testing.T) {
	if ethURL == "" || contractAddr == "" || startBlock == 0 {
		t.Skip()
	}

	c := qt.New(t)
	log.Init("debug", "stdout")

	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	addr := common.HexToAddress(contractAddr)
	client, err := New(Options{
		EthURL: ethURL,
		SQLite: sqlite, ContractAddr: addr})
	c.Assert(err, qt.IsNil)

	go client.syncBlocksLive() // nolint:errcheck
	err = client.syncEventsLive()
	c.Assert(err, qt.IsNil)
}

func TestSync(t *testing.T) {
	if ethURL == "" || contractAddr == "" || startBlock == 0 {
		t.Skip()
	}

	c := qt.New(t)
	log.Init("debug", "stdout")

	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	addr := common.HexToAddress(contractAddr)
	client, err := New(Options{
		EthURL: ethURL,
		SQLite: sqlite, ContractAddr: addr})
	c.Assert(err, qt.IsNil)

	// store meta into db
	chainID, err := client.client.ChainID(context.Background())
	c.Assert(err, qt.IsNil)
	err = client.db.InitMeta(chainID.Uint64(), startBlock)
	c.Assert(err, qt.IsNil)

	err = client.Sync()
	c.Assert(err, qt.IsNil)
}
