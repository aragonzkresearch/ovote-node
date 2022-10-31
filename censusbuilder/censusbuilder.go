package censusbuilder

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aragonzkresearch/ovote-node/census"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
	"go.vocdoni.io/dvote/log"
)

// CensusBuilder manages multiple Census MerkleTrees
type CensusBuilder struct {
	subDBsPath string
	db         db.Database

	// censuses contains the loaded census
	censuses map[uint64]*census.Census
}

// New loads the CensusBuilder
func New(database db.Database, subDBsPath string) (*CensusBuilder, error) {
	cb := &CensusBuilder{
		subDBsPath: subDBsPath,
		db:         database,
		censuses:   make(map[uint64]*census.Census),
	}

	wTx := cb.db.WriteTx()
	defer wTx.Discard()

	// if nextIndex is not set in the db, initialize it to 0
	_, err := cb.getNextCensusID(wTx)
	if err != nil {
		err = cb.setNextCensusID(wTx, 0)
		if err != nil {
			return nil, err
		}
	}

	// TODO check that nextCensusID matches the last subdb Census db in
	// disk

	// commit the db.WriteTx
	if err := wTx.Commit(); err != nil {
		return nil, err
	}

	return cb, nil
}

var dbKeyNextCensusID = []byte("nextCensusID")

func (cb *CensusBuilder) setNextCensusID(wTx db.WriteTx, nextCensusID uint64) error {
	b := make([]byte, 8) //nolint:gomnd
	binary.LittleEndian.PutUint64(b, uint64(nextCensusID))
	if err := wTx.Set(dbKeyNextCensusID, b); err != nil {
		return err
	}
	return nil
}

func (cb *CensusBuilder) getNextCensusID(rTx db.ReadTx) (uint64, error) {
	b, err := rTx.Get(dbKeyNextCensusID)
	if err != nil {
		return 0, err
	}
	nextCensusID := binary.LittleEndian.Uint64(b)
	return nextCensusID, nil
}

// createCensus will create the Census sub-db and point to it in memory
func (cb *CensusBuilder) createCensus(censusID uint64) error {
	path := filepath.Join(cb.subDBsPath, strconv.Itoa(int(censusID)))

	// check if sub-db already exists for the Census
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return fmt.Errorf("can not createCensus, err: %s", err)
	}

	optsDB := db.Options{Path: path}
	database, err := pebbledb.New(optsDB)
	if err != nil {
		return err
	}
	optsCensus := census.Options{DB: database}
	c, err := census.New(optsCensus)
	if err != nil {
		return err
	}
	cb.censuses[censusID] = c
	return nil
}

// loadCensusIfNotYet will load the Census in memory if it is not loaded yet
func (cb *CensusBuilder) loadCensusIfNotYet(censusID uint64) error {
	path := filepath.Join(cb.subDBsPath, strconv.Itoa(int(censusID)))

	if _, ok := cb.censuses[censusID]; !ok {
		// check if sub-db exists for the Census
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return fmt.Errorf("CensusID=%d does not exist", censusID)
		}

		// census not loaded, load it
		optsDB := db.Options{Path: path}
		database, err := pebbledb.New(optsDB)
		if err != nil {
			return err
		}
		optsCensus := census.Options{DB: database}
		c, err := census.New(optsCensus)
		if err != nil {
			return err
		}
		cb.censuses[censusID] = c
	}
	return nil
}

// TODO to create a new Census, add keys, and close it, an ethereum signature
// will be required, to ensure that all these actions are performed by the same
// key. Probably the authentication will be at the API level.

// NewCensus will create a new Census, if the Census already exists, will load it
func (cb *CensusBuilder) NewCensus() (uint64, error) {
	rTx := cb.db.ReadTx()
	defer rTx.Discard()
	nextCensusID, err := cb.getNextCensusID(rTx)
	if err != nil {
		return 0, err
	}

	err = cb.createCensus(nextCensusID)
	if err != nil {
		return 0, err
	}

	// store nextCensusID+1 in the CensusBuilder.db
	wTx := cb.db.WriteTx()
	defer wTx.Discard()
	err = cb.setNextCensusID(wTx, nextCensusID+1)
	if err != nil {
		return 0, err
	}
	if err := wTx.Commit(); err != nil {
		return 0, err
	}
	log.Debugf("[CensusID=%d] New census created", nextCensusID)

	return nextCensusID, nil
}

// CloseCensus closes the Census of the given censusID.
func (cb *CensusBuilder) CloseCensus(censusID uint64) error {
	// TODO to close the Census, the sender will need to be authorized to
	// ensure that is the same that created the Census
	err := cb.loadCensusIfNotYet(censusID)
	if err != nil {
		return err
	}
	return cb.censuses[censusID].Close()
}

// CensusRoot returns the Root of the Census if the Census is closed.
func (cb *CensusBuilder) CensusRoot(censusID uint64) ([]byte, error) {
	err := cb.loadCensusIfNotYet(censusID)
	if err != nil {
		return nil, err
	}
	root, err := cb.censuses[censusID].Root()
	if err != nil {
		return nil, fmt.Errorf("Can not get the CensusRoot, %s", err)
	}
	return root, nil
}

// CensusInfo returns metadata about the Census for the given CensusID
func (cb *CensusBuilder) CensusInfo(censusID uint64) (*census.Info, error) {
	err := cb.loadCensusIfNotYet(censusID)
	if err != nil {
		return nil, err
	}

	return cb.censuses[censusID].Info()
}

// AddPublicKeys adds the batch of given PublicKeys to the Census for the given
// censusID.
func (cb *CensusBuilder) AddPublicKeys(censusID uint64, pubKs []babyjub.PublicKey,
	weights []*big.Int) error {
	err := cb.loadCensusIfNotYet(censusID)
	if err != nil {
		return err
	}
	invalids, err := cb.censuses[censusID].AddPublicKeys(pubKs, weights)
	if err != nil {
		return err
	}
	if len(invalids) != 0 {
		return fmt.Errorf("CensusBuilder.AddPublicKeys error: %d invalid"+
			" keys, invalid msg for key %d: %s", len(invalids),
			invalids[0].Index, invalids[0].Error)
	}
	log.Debugf("[CensusID=%d] %d PublicKeys added", censusID, len(pubKs))
	return nil
}

// AddPublicKeysAndStoreError will call the AddPublicKeys and if there is an
// error, it will store it into the DB. This method is designed to be called
// from a goroutine.
func (cb *CensusBuilder) AddPublicKeysAndStoreError(censusID uint64,
	pubKs []babyjub.PublicKey, weights []*big.Int) {
	if err := cb.AddPublicKeys(censusID, pubKs, weights); err != nil {
		log.Debugf("[CensusID=%d] error: %s", err)
		if err2 := cb.SetErrMsg(censusID, err.Error()); err2 != nil {
			log.Errorf("Error while trying to store CensusID:%d status: %s. Error: %s",
				censusID, err, err2)
		}
	}
}

// SetErrMsg stores the given error message into the CensusID db
func (cb *CensusBuilder) SetErrMsg(censusID uint64, status string) error {
	err := cb.loadCensusIfNotYet(censusID)
	if err != nil {
		return err
	}
	err = cb.censuses[censusID].SetErrMsg(status)
	if err != nil {
		return err
	}
	return nil
}

// GetProof returns the leaf Value and the MerkleProof compressed for the given
// PublicKey in the given CensusID
func (cb *CensusBuilder) GetProof(censusID uint64, pubK *babyjub.PublicKey) (
	uint64, *big.Int, []byte, error) {
	// TODO maybe add auth for this method, requiring a signature by the
	// privK of the given PubK

	if err := cb.loadCensusIfNotYet(censusID); err != nil {
		return 0, nil, nil, err
	}
	index, weight, proof, err := cb.censuses[censusID].GetProof(pubK)
	if err != nil {
		return 0, nil, nil, err
	}
	return index, weight, proof, nil
}
