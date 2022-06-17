package eth

import (
	"context"
	"database/sql"
	"encoding/hex"
	"flag"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	qt "github.com/frankban/quicktest"
	"github.com/groupoidlabs/ovote-node/db"
	"github.com/vocdoni/arbo"
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

func TestProcessEventLog(t *testing.T) {
	c := qt.New(t)
	log.Init("debug", "stdout")

	sqlDB, err := sql.Open("sqlite3", filepath.Join(c.TempDir(), "testdb.sqlite3"))
	c.Assert(err, qt.IsNil)

	sqlite := db.NewSQLite(sqlDB)
	err = sqlite.Migrate()
	c.Assert(err, qt.IsNil)

	client := Client{db: sqlite}
	c.Assert(err, qt.IsNil)

	// log bytes generated from the contract event logs.
	d0Hex := "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000006" +
		"0000000000000000000000000000000000000000000000000000000000003039" +
		"08d67ea943c2daebe8b75017e7019efe37891ed6b67dd79b7a245aa634a62845" +
		"00000000000000000000000000000000000000000000000000000000000003e8" +
		"00000000000000000000000000000000000000000000000000000000006646b7" +
		"000000000000000000000000000000000000000000000000000000000000000a" +
		"000000000000000000000000000000000000000000000000000000000000000a" +
		"000000000000000000000000000000000000000000000000000000000000003c" +
		"0000000000000000000000000000000000000000000000000000000000000001"
	d0, err := hex.DecodeString(d0Hex)
	c.Assert(err, qt.IsNil)
	d1Hex := "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000006" +
		"08d67ea943c2daebe8b75017e7019efe37891ed6b67dd79b7a245aa634a62845" +
		"000000000000000000000000000000000000000000000000000000000000012c" +
		"0000000000000000000000000000000000000000000000000000000000000190"
	d1, err := hex.DecodeString(d1Hex)
	c.Assert(err, qt.IsNil)
	d2Hex := "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000006" +
		"0000000000000000000000000000000000000000000000000000000000000001"
	d2, err := hex.DecodeString(d2Hex)
	c.Assert(err, qt.IsNil)

	log0 := types.Log{Data: d0, BlockNumber: 1}
	log1 := types.Log{Data: d1, BlockNumber: 2}
	log2 := types.Log{Data: d2, BlockNumber: 3}

	err = client.processEventLog(log0)
	c.Assert(err, qt.IsNil)
	err = client.processEventLog(log1)
	c.Assert(err, qt.IsNil)
	err = client.processEventLog(log2)
	c.Assert(err, qt.IsNil)

	// check that the process from the event log has been correctly stored
	// in the db
	process, err := sqlite.ReadProcessByID(6)
	c.Assert(err, qt.IsNil)
	c.Assert(process.ID, qt.Equals, uint64(6))
	c.Assert(arbo.BytesToBigInt(process.CensusRoot).String(), qt.DeepEquals,
		"3997482243935470019154908634129466064231369626981967795243271053776626526277")
	c.Assert(process.CensusSize, qt.Equals, uint64(1000))
	c.Assert(process.EthBlockNum, qt.Equals, uint64(1))
	c.Assert(process.ResPubStartBlock, qt.Equals, uint64(6702775))
	c.Assert(process.ResPubWindow, qt.Equals, uint64(10))
	c.Assert(process.MinParticipation, qt.Equals, uint8(10))
	c.Assert(process.MinPositiveVotes, qt.Equals, uint8(60))
	c.Assert(process.Type, qt.Equals, uint8(1))
}

func TestParseEventNewProcess(t *testing.T) {
	c := qt.New(t)
	// log bytes generated from the contract newProcess event.
	// creator, id: 1, txHash: 0, censusRoot: 1111, censusSize: 1000,
	// resPubStartBlock: 6697316, resPubWindow: 1000, minParticipation: 0,
	// minPositiveVotes: 60, type: 1
	dHex := "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000001" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000000000457" +
		"00000000000000000000000000000000000000000000000000000000000003e8" +
		"0000000000000000000000000000000000000000000000000000000000663164" +
		"00000000000000000000000000000000000000000000000000000000000003e8" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"000000000000000000000000000000000000000000000000000000000000003c" +
		"0000000000000000000000000000000000000000000000000000000000000001"
	d, err := hex.DecodeString(dHex)
	c.Assert(err, qt.IsNil)

	e, err := parseEventNewProcess(d)
	c.Assert(err, qt.IsNil)

	c.Assert(e.Creator.String(), qt.Equals,
		"0xa6a2E217aF2f983ee55A6e2195C1763a9420f8ad")
	c.Assert(e.ProcessID, qt.Equals, uint64(1))
	c.Assert(hex.EncodeToString(e.TxHash[:]), qt.Equals,
		"0000000000000000000000000000000000000000000000000000000000000000")
	c.Assert(hex.EncodeToString(e.CensusRoot[:]), qt.Equals,
		"5704000000000000000000000000000000000000000000000000000000000000")
	c.Assert(e.CensusSize, qt.Equals, uint64(1000))
	c.Assert(e.ResPubStartBlock, qt.Equals, uint64(6697316))
	c.Assert(e.ResPubWindow, qt.Equals, uint64(1000))
	c.Assert(e.MinParticipation, qt.Equals, uint8(0))
	c.Assert(e.MinPositiveVotes, qt.Equals, uint8(60))
	c.Assert(e.Type, qt.Equals, uint8(1))

	// log bytes generated from the contract newProcess event.
	// creator, id: 2, txHash:
	// 0x7e1975a6bf513022a8cc382a3cdb1e1dbcd58ebb1cb9abf11e64aadb21262516,
	// censusRoot:
	// 3997482243935470019154908634129466064231369626981967795243271053776626526277,
	// censusSize: 12345, resPubStartBlock: 6697316, resPubWindow: 2500,
	// minParticipation: 30, minPositiveVotes: 50, type: 1
	dHex = "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000002" +
		"7e1975a6bf513022a8cc382a3cdb1e1dbcd58ebb1cb9abf11e64aadb21262516" +
		"08d67ea943c2daebe8b75017e7019efe37891ed6b67dd79b7a245aa634a62845" +
		"0000000000000000000000000000000000000000000000000000000000003039" +
		"0000000000000000000000000000000000000000000000000000000000663164" +
		"00000000000000000000000000000000000000000000000000000000000009c4" +
		"000000000000000000000000000000000000000000000000000000000000001e" +
		"0000000000000000000000000000000000000000000000000000000000000032" +
		"0000000000000000000000000000000000000000000000000000000000000001"
	d, err = hex.DecodeString(dHex)
	c.Assert(err, qt.IsNil)

	e, err = parseEventNewProcess(d)
	c.Assert(err, qt.IsNil)

	c.Assert(e.Creator.String(), qt.Equals,
		"0xa6a2E217aF2f983ee55A6e2195C1763a9420f8ad")
	c.Assert(e.ProcessID, qt.Equals, uint64(2))
	c.Assert(hex.EncodeToString(e.TxHash[:]), qt.Equals,
		"7e1975a6bf513022a8cc382a3cdb1e1dbcd58ebb1cb9abf11e64aadb21262516")
	c.Assert(arbo.BytesToBigInt(e.CensusRoot[:]).String(), qt.Equals,
		"3997482243935470019154908634129466064231369626981967795243271053776626526277")
	c.Assert(e.CensusSize, qt.Equals, uint64(12345))
	c.Assert(e.ResPubStartBlock, qt.Equals, uint64(6697316))
	c.Assert(e.ResPubWindow, qt.Equals, uint64(2500))
	c.Assert(e.MinParticipation, qt.Equals, uint8(30))
	c.Assert(e.MinPositiveVotes, qt.Equals, uint8(50))

	// log bytes generated from the contract newProcess event.
	dHex = "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000006" +
		"0000000000000000000000000000000000000000000000000000000000003039" +
		"08d67ea943c2daebe8b75017e7019efe37891ed6b67dd79b7a245aa634a62845" +
		"00000000000000000000000000000000000000000000000000000000000003e8" +
		"00000000000000000000000000000000000000000000000000000000006646b7" +
		"000000000000000000000000000000000000000000000000000000000000000a" +
		"000000000000000000000000000000000000000000000000000000000000000a" +
		"000000000000000000000000000000000000000000000000000000000000003c" +
		"0000000000000000000000000000000000000000000000000000000000000001"

	d, err = hex.DecodeString(dHex)
	c.Assert(err, qt.IsNil)

	e, err = parseEventNewProcess(d)
	c.Assert(err, qt.IsNil)

	c.Assert(e.Creator.String(), qt.Equals,
		"0xa6a2E217aF2f983ee55A6e2195C1763a9420f8ad")
	c.Assert(e.ProcessID, qt.Equals, uint64(6))
	c.Assert(hex.EncodeToString(e.TxHash[:]), qt.Equals,
		"0000000000000000000000000000000000000000000000000000000000003039")
	c.Assert(arbo.BytesToBigInt(e.CensusRoot[:]).String(), qt.Equals,
		"3997482243935470019154908634129466064231369626981967795243271053776626526277")
	c.Assert(e.CensusSize, qt.Equals, uint64(1000))
	c.Assert(e.ResPubStartBlock, qt.Equals, uint64(6702775))
	c.Assert(e.ResPubWindow, qt.Equals, uint64(10))
	c.Assert(e.MinParticipation, qt.Equals, uint8(10))
	c.Assert(e.MinPositiveVotes, qt.Equals, uint8(60))
}

func TestParseEventResultPublished(t *testing.T) {
	c := qt.New(t)
	// log bytes generated from the contract resultPublish event
	dHex := "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000006" +
		"08d67ea943c2daebe8b75017e7019efe37891ed6b67dd79b7a245aa634a62845" +
		"000000000000000000000000000000000000000000000000000000000000012c" +
		"0000000000000000000000000000000000000000000000000000000000000190"
	d, err := hex.DecodeString(dHex)
	c.Assert(err, qt.IsNil)

	e, err := parseEventResultPublished(d)
	c.Assert(err, qt.IsNil)

	c.Assert(e.Publisher.String(), qt.Equals,
		"0xa6a2E217aF2f983ee55A6e2195C1763a9420f8ad")
	c.Assert(e.ProcessID, qt.Equals, uint64(6))
	c.Assert(arbo.BytesToBigInt(e.ReceiptsRoot[:]).String(), qt.Equals,
		"3997482243935470019154908634129466064231369626981967795243271053776626526277")
	c.Assert(e.Result, qt.Equals, uint64(300))
	c.Assert(e.NVotes, qt.Equals, uint64(400))
}

func TestParseEventProcessClosed(t *testing.T) {
	c := qt.New(t)

	// log bytes generated from the contract processClosed event
	dHex := "000000000000000000000000a6a2e217af2f983ee55a6e2195c1763a9420f8ad" +
		"0000000000000000000000000000000000000000000000000000000000000006" +
		"0000000000000000000000000000000000000000000000000000000000000001"
	d, err := hex.DecodeString(dHex)
	c.Assert(err, qt.IsNil)

	e, err := parseEventProcessClosed(d)
	c.Assert(err, qt.IsNil)

	c.Assert(e.Caller.String(), qt.Equals, "0xa6a2E217aF2f983ee55A6e2195C1763a9420f8ad")
	c.Assert(e.ProcessID, qt.Equals, uint64(6))
	c.Assert(e.Success, qt.IsTrue)
}
