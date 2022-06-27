package main

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/aragonzkresearch/ovote-node/api"
	"github.com/aragonzkresearch/ovote-node/censusbuilder"
	"github.com/aragonzkresearch/ovote-node/db"
	"github.com/aragonzkresearch/ovote-node/eth"
	"github.com/aragonzkresearch/ovote-node/prover"
	"github.com/aragonzkresearch/ovote-node/votesaggregator"
	"github.com/ethereum/go-ethereum/common"
	_ "github.com/mattn/go-sqlite3"
	flag "github.com/spf13/pflag"
	kvdb "go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
	"go.vocdoni.io/dvote/log"
)

// Config contains the main configuration parameters of the node
type Config struct {
	dir, logLevel, port             string
	startScanBlock                  uint64
	censusBuilder, votesAggregator  bool
	contractAddr, ethURL, proverURL string
}

func main() {
	config := Config{}

	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	flag.StringVarP(&config.dir, "dir", "d", filepath.Join(home, ".ovote-node"),
		"storage data directory")
	flag.StringVarP(&config.logLevel, "logLevel", "l", "info", "log level (info, debug, warn, error)")
	flag.StringVarP(&config.port, "port", "p", "8080", "network port for the HTTP API")
	flag.BoolVarP(&config.censusBuilder, "censusbuilder", "c", false, "CensusBuilder active")
	flag.BoolVarP(&config.votesAggregator, "votesaggregator", "v", false, "VotesAggregator active")
	flag.StringVar(&config.ethURL, "eth", "", "web3 provider url")
	flag.StringVar(&config.contractAddr, "addr", "", "OVOTE contract address")
	flag.Uint64Var(&config.startScanBlock, "block", 0,
		"Start scanning block (usually the block where the OVOTE contract was deployed)")
	flag.StringVar(&config.proverURL, "prover", "127.0.0.1:9000", "prover url")
	// TODO add flag for configurable threshold of minimum census size (to prevent small censuses)

	flag.CommandLine.SortFlags = false
	flag.Parse()

	log.Init(config.logLevel, "stdout")

	log.Debugf("Config: %#v\n", config)

	var censusBuilder *censusbuilder.CensusBuilder
	var votesAggregator *votesaggregator.VotesAggregator
	if config.censusBuilder {
		opts := kvdb.Options{Path: filepath.Join(config.dir, "censusbuilder")}
		database, err := pebbledb.New(opts)
		if err != nil {
			log.Fatal(err)
		}

		censusBuilder, err = censusbuilder.New(database, filepath.Join(config.dir, "subsdb"))
		if err != nil {
			log.Fatal(err)
		}
	}

	if config.votesAggregator {
		// prepare DB
		sqlDB, err := sql.Open("sqlite3", filepath.Join(config.dir, "testdb.sqlite3"))
		if err != nil {
			log.Fatal(err)
		}
		sqlite := db.NewSQLite(sqlDB)
		err = sqlite.Migrate()
		if err != nil {
			log.Fatal(err)
		}

		// TODO give error if config.contractAddr is incorrect
		contractAddr := common.HexToAddress(config.contractAddr)

		// prepare ethereum client
		ethC, err := eth.New(eth.Options{
			EthURL:       config.ethURL,
			SQLite:       sqlite,
			ContractAddr: contractAddr,
		})
		if err != nil {
			log.Fatal(err)
		}

		// TODO check that ethC has access to OVOTE contract address

		// set (if not set already) the 'lastSyncBlockNum'
		// check if lastSyncBlockNum exists in the db
		lastSyncBlockNum, err := sqlite.GetLastSyncBlockNum()
		if err != nil && err != db.ErrMetaNotInDB {
			log.Fatal(err)
		}
		if err == db.ErrMetaNotInDB {
			// if not in db, check that the flag is not 0, and store it
			if config.startScanBlock == 0 {
				log.Fatal("startblock flag can not be 0 to initialize db" +
					" (to prevent scanning since the genesis)")
			}
			err = sqlite.InitMeta(ethC.ChainID, config.startScanBlock)
			if err != nil {
				log.Fatal(err)
			}
			lastSyncBlockNum = config.startScanBlock
		}
		log.Infof("Eth scanning from block: %d", lastSyncBlockNum)

		proverClient := prover.NewClient(config.proverURL)

		// prepare VotesAggregator
		votesAggregator, err = votesaggregator.New(sqlite, ethC.ChainID, proverClient)
		if err != nil {
			log.Fatal(err)
		}

		err = ethC.Sync()
		if err != nil {
			log.Fatal(err)
		}
	}

	a, err := api.New(censusBuilder, votesAggregator)
	if err != nil {
		log.Fatal(err)
	}
	err = a.Serve(config.port)
	if err != nil {
		log.Fatal(err)
	}
}
