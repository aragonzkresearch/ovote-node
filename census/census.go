package census

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/aragonzkresearch/ovote-node/types"
	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/vocdoni/arbo"
	"go.vocdoni.io/dvote/db"
)

var (
	dbKeyNextIndex    = []byte("nextIndex")
	dbKeyCensusClosed = []byte("censusClosed")
)

var (
	// ErrCensusNotClosed is used when trying to do some action with the Census
	// that needs the Census to be closed
	ErrCensusNotClosed = errors.New("Census not closed yet")
	// ErrCensusClosed is used when trying to add keys to a census and the census
	// is already closed
	ErrCensusClosed = errors.New("Census closed, can not add more keys")
	// ErrMaxNLeafsReached is used when trying to add a number of new publicKeys
	// which would exceed the maximum number of keys in the census.
	ErrMaxNLeafsReached = fmt.Errorf("MaxNLeafs (%d) reached", types.MaxNLeafs)
)

// Info contains metadata about a Census
type Info struct {
	// ErrMsg contains the stored error message stored for the last
	// operation that gave error
	ErrMsg string `json:"errMsg,omitempty"`
	Size   uint64 `json:"size"`
	Closed bool   `json:"closed"`
	Root   []byte `json:"root,omitempty"`
}

// Census contains the MerkleTree with the PublicKeys
type Census struct {
	tree *arbo.Tree
	db   db.Database
}

// Options is used to pass the parameters to load a new Census
type Options struct {
	// DB defines the database that will be used for the census
	DB db.Database
}

// New loads the census
func New(opts Options) (*Census, error) {
	arboConfig := arbo.Config{
		Database:     opts.DB,
		MaxLevels:    types.MaxLevels,
		HashFunction: arbo.HashFunctionPoseidon,
		// ThresholdNLeafs: not specified, use the default
	}

	// TODO benchmark concurrent usage to determine wether to do the
	// approach of creating a new db dir for each Census, or to use the
	// same db for all the Censuses using a different db prefix for each
	// Census.
	wTx := opts.DB.WriteTx()
	defer wTx.Discard()

	tree, err := arbo.NewTreeWithTx(wTx, arboConfig)
	if err != nil {
		return nil, err
	}

	c := &Census{
		tree: tree,
		db:   opts.DB,
	}

	// if nextIndex is not set in the db, initialize it to 0
	_, err = c.getNextIndex(wTx)
	if err != nil {
		err = c.setNextIndex(wTx, 0)
		if err != nil {
			return nil, err
		}
	}

	// store editable=true if
	if err := wTx.Set(dbKeyCensusClosed, []byte{0}); err != nil {
		return nil, err
	}

	// commit the db.WriteTx
	if err := wTx.Commit(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Census) setNextIndex(wTx db.WriteTx, nextIndex uint64) error {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(nextIndex))
	if err := wTx.Set(dbKeyNextIndex, b); err != nil {
		return err
	}
	return nil
}

func (c *Census) getNextIndex(rTx db.ReadTx) (uint64, error) {
	b, err := rTx.Get(dbKeyNextIndex)
	if err != nil {
		return 0, err
	}
	nextIndex := binary.LittleEndian.Uint64(b)
	return nextIndex, nil
}

// Size returns the number of PublicKeys added to the Census.
func (c *Census) Size() (uint64, error) {
	rTx := c.db.ReadTx()
	defer rTx.Discard()
	return c.getNextIndex(rTx)
}

var dbKeyErrMsg = []byte("errmsg")

// SetErrMsg stores the given error message into the Census db
func (c *Census) SetErrMsg(status string) error {
	wTx := c.db.WriteTx()
	defer wTx.Discard()

	if err := wTx.Set(dbKeyErrMsg, []byte(status)); err != nil {
		return err
	}
	// commit the db.WriteTx
	if err := wTx.Commit(); err != nil {
		return err
	}
	return nil
}

// GetErrMsg returns the error message of the Census
func (c *Census) GetErrMsg() (string, error) {
	rTx := c.db.ReadTx()
	defer rTx.Discard()

	b, err := rTx.Get(dbKeyErrMsg)
	if err == db.ErrKeyNotFound {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return string(b), nil
}

// Close closes the census
func (c *Census) Close() error {
	isClosed, err := c.IsClosed()
	if err != nil {
		return err
	}
	if isClosed {
		return fmt.Errorf("Census already closed")
	}
	wTx := c.db.WriteTx()
	defer wTx.Discard()
	if err := wTx.Set(dbKeyCensusClosed, []byte{1}); err != nil {
		return err
	}
	// commit the db.WriteTx
	if err := wTx.Commit(); err != nil {
		return err
	}

	return nil
}

// IsClosed returns true if the census is closed, and false if the census is
// still open
func (c *Census) IsClosed() (bool, error) {
	rTx := c.db.ReadTx()
	defer rTx.Discard()

	b, err := rTx.Get(dbKeyCensusClosed)
	if err != nil {
		return false, err
	}

	return bytes.Equal(b, []byte{1}), nil
}

// Root returns the CensusRoot if the Census is closed.
func (c *Census) Root() ([]byte, error) {
	isClosed, err := c.IsClosed()
	if err != nil {
		return nil, err
	}

	if !isClosed {
		return nil, ErrCensusNotClosed
	}
	return c.tree.Root()
}

// IntermediateRoot returns the CensusRoot even if the Census is not closed.
// WARNING: It should be used only for testing purposes.
func (c *Census) IntermediateRoot() ([]byte, error) {
	return c.tree.Root()
}

// Info returns metadata about the Census
func (c *Census) Info() (*Info, error) {
	size, err := c.Size()
	if err != nil {
		return nil, err
	}

	errMsg, err := c.GetErrMsg()
	if err != nil {
		return nil, err
	}

	isClosed, err := c.IsClosed()
	if err != nil {
		return nil, err
	}

	root := types.EmptyRoot
	if isClosed {
		root, err = c.Root()
		if err != nil {
			return nil, err
		}
	}

	ci := &Info{
		ErrMsg: errMsg,
		Size:   size,
		Closed: isClosed,
		Root:   root,
	}

	return ci, nil
}

// AddPublicKeys adds the batch of given PublicKeys, assigning incremental
// indexes to each one.
func (c *Census) AddPublicKeys(pubKs []babyjub.PublicKey,
	weights []*big.Int) ([]arbo.Invalid, error) {
	isClosed, err := c.IsClosed()
	if err != nil {
		return nil, err
	}
	if isClosed {
		return nil, ErrCensusClosed
	}
	wTx := c.db.WriteTx()
	defer wTx.Discard()

	nextIndex, err := c.getNextIndex(wTx)
	if err != nil {
		return nil, err
	}

	if nextIndex+uint64(len(pubKs)) > types.MaxNLeafs {
		return nil, fmt.Errorf("%s, current index: %d, trying to add %d keys",
			ErrMaxNLeafsReached, nextIndex, len(pubKs))
	}
	var indexes [][]byte
	var pubKHashes [][]byte
	for i := 0; i < len(pubKs); i++ {
		// overflow in index should not be possible, as previously the
		// number of keys being added is already checked

		index := nextIndex + uint64(i)
		// TODO ensure that the weights[i] does not overflow the field
		indexAndWeight := types.IndexAndWeightToBytes(
			nextIndex+uint64(i),
			weights[i],
		)
		indexBytes := types.Uint64ToIndex(index)
		indexes = append(indexes[:], indexBytes)

		// store the mapping between PublicKey->Index,Weight
		pubKComp := pubKs[i].Compress()
		if err := wTx.Set(pubKComp[:], indexAndWeight[:]); err != nil {
			return nil, err
		}

		pubKHashBytes, err := types.HashPubKBytes(&pubKs[i], weights[i])
		if err != nil {
			return nil, err
		}
		pubKHashes = append(pubKHashes, pubKHashBytes)
	}

	invalids, err := c.tree.AddBatchWithTx(wTx, indexes, pubKHashes)
	if err != nil {
		return invalids, err
	}
	if len(invalids) != 0 {
		return invalids, fmt.Errorf("Can not add %d PublicKeys", len(invalids))
	}

	// TODO check overflow
	if err = c.setNextIndex(wTx, (nextIndex)+uint64(len(pubKs))); err != nil {
		return nil, err
	}

	// commit the db.WriteTx
	if err := wTx.Commit(); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetProof returns the leaf Value and the MerkleProof compressed for the given
// PublicKey
func (c *Census) GetProof(pubK *babyjub.PublicKey) (uint64, []byte, error) {
	isClosed, err := c.IsClosed()
	if err != nil {
		return 0, nil, err
	}
	if !isClosed {
		// if the Census is not closed, means that the Census is still
		// being updated. MerkleProofs will be generated once the
		// Census is closed for the final CensusRoot
		return 0, nil, ErrCensusNotClosed
	}

	rTx := c.db.ReadTx()
	defer rTx.Discard()

	// get index of pubK
	pubKComp := pubK.Compress()
	indexAndWeight, err := rTx.Get(pubKComp[:])
	if err != nil {
		return 0, nil, err
	}
	index, weight, err := types.BytesToIndexAndWeight(indexAndWeight)
	if err != nil {
		return 0, nil, err
	}
	index32Bytes := types.Uint64ToIndex(index)
	_, leafV, s, existence, err := c.tree.GenProof(index32Bytes)
	if err != nil {
		return 0, nil, err
	}
	if !existence {
		// proof of non-existence currently not needed in the current use case
		return 0, nil,
			fmt.Errorf("publicKey does not exist in the census (%x)", pubKComp[:])
	}
	hashPubKBytes, err := types.HashPubKBytes(pubK, weight)
	if err != nil {
		return 0, nil, err
	}
	if !bytes.Equal(leafV, hashPubKBytes) {
		return 0, nil,
			fmt.Errorf("leafV!=pubK: %x!=%x", leafV, pubK)
	}
	return index, s, nil
}

// CheckProof checks a given MerkleProof of the given PublicKey (& index)
// for the given CensusRoot
func CheckProof(root, proof []byte, index uint64, pubK *babyjub.PublicKey,
	weight *big.Int) (bool, error) {
	// indexBytes := arbo.BigIntToBytes(maxKeyLen, big.NewInt(int64(index))) //nolint:gomnd
	indexBytes := types.Uint64ToIndex(index)
	hashPubK, err := types.HashPubKBytes(pubK, weight)
	if err != nil {
		return false, err
	}

	return arbo.CheckProof(arbo.HashFunctionPoseidon, indexBytes, hashPubK, root, proof)
}
