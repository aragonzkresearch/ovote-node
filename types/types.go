package types

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/iden3/go-iden3-crypto/babyjub"
	"github.com/iden3/go-iden3-crypto/poseidon"
	"github.com/vocdoni/arbo"
)

// ProcessStatus type is used to define the status of the Process
type ProcessStatus int

var (
	// HashLen indicates the length of the bytes representation of the
	// output of the Hash function used, Poseidon Hash in OVOTE's case.
	HashLen int = arbo.HashFunctionPoseidon.Len()
)

const (
	// ProcessStatusOn indicates that the process is accepting vote (Voting
	// phase)
	ProcessStatusOn ProcessStatus = iota
	// ProcessStatusFrozen indicates that the process is in
	// ResultsPublishingPhase and is no longer accepting new votes, the
	// proof can now be generated
	ProcessStatusFrozen
	// ProcessStatusProofGenerating indicates that the process is no longer
	// accepting new votes and the zkProof is being generated
	ProcessStatusProofGenerating
	// ProcessStatusProofGenerated indicates that the process is finished,
	// and the zkProof is already generated
	ProcessStatusProofGenerated
	// ProcessStatusContractClosed indicates that the process has been
	// closed in the contract.
	ProcessStatusContractClosed
)

// ByteArray is a type alias over []byte to implement custom json marshalers in
// hex
type ByteArray []byte

// MarshalJSON implements the Marshaler interface for ByteArray types
func (b ByteArray) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(b))
}

// UnmarshalJSON implements the Unmarshaler interface for ByteArray types
func (b *ByteArray) UnmarshalJSON(j []byte) error {
	var s string
	err := json.Unmarshal(j, &s)
	if err != nil {
		return err
	}
	b2, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	*b = ByteArray(b2)
	return nil
}

// CensusProof contains the proof of a PublicKey in the Census Tree
type CensusProof struct {
	Index       uint64             `json:"index"`
	PublicKey   *babyjub.PublicKey `json:"publicKey"`
	Weight      *big.Int           `json:"weight"`
	MerkleProof ByteArray          `json:"merkleProof"`
}

// VotePackage represents the vote sent by the User
type VotePackage struct {
	Signature   babyjub.SignatureComp `json:"signature"`
	CensusProof CensusProof           `json:"censusProof"`
	Vote        ByteArray             `json:"vote"`
}

// Process represents a voting process
type Process struct {
	// ID is determined by the SmartContract, is unique for each Process
	ID uint64
	// ProofID determines the ID assigned by the proof-server, is used to
	// retrieve the proof
	ProofID uint64
	// CensusRoot indicates the CensusRoot of the Process, the same
	// CensusRoot can be reused by different Processes
	CensusRoot []byte
	// CensusSize determines the number of public keys placed in the
	// CensusTree leaves under the CensusRoot
	CensusSize uint64
	// EthBlockNum indicates at which Ethereum block number the process has
	// been created
	EthBlockNum uint64
	// ResPubStartBlock (Results Publishing Start Block) indicates the
	// EthBlockNum where the process ends, in which the results can be
	// published
	ResPubStartBlock uint64
	// ResPubWindow (Results Publishing Window) indicates the window of
	// time (blocks), which starts at the ResPubStartBlock and ends at
	// ResPubStartBlock+ResPubWindow.  During this window of time, the
	// results + zkProofs can be sent to the SmartContract.
	ResPubWindow uint64
	// MinParticipation sets a threshold of minimum number of votes over
	// the total users in the census (% over CensusSize)
	MinParticipation uint8
	// Type of process, where 0: multisig, 1: referendum
	Type uint8
	// InsertedDatetime contains the datetime of when the process was
	// inserted in the db
	InsertedDatetime time.Time
	// Status determines the current status of the process
	Status ProcessStatus
}

// HashVote computes the vote hash following the circuit approach
func HashVote(chainID, processID uint64, vote []byte) (*big.Int, error) {
	voteBI := arbo.BytesToBigInt(vote)
	o, err := poseidon.Hash([]*big.Int{
		big.NewInt(int64(chainID)),
		big.NewInt(int64(processID)),
		voteBI,
	})
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (vp *VotePackage) verifySignature(chainID, processID uint64) error {
	msgToSign, err := HashVote(chainID, processID, vp.Vote)
	if err != nil {
		return err
	}

	sigUncompressed, err := vp.Signature.Decompress()
	if err != nil {
		return err
	}
	v := vp.CensusProof.PublicKey.VerifyPoseidon(
		msgToSign, sigUncompressed)
	if !v {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
func (vp *VotePackage) verifyMerkleProof(root []byte) error {
	indexBytes := Uint64ToIndex(vp.CensusProof.Index)
	pubKHashBytes, err := HashPubKBytes(vp.CensusProof.PublicKey,
		vp.CensusProof.Weight)
	if err != nil {
		return err
	}
	v, err := arbo.CheckProof(arbo.HashFunctionPoseidon, indexBytes,
		pubKHashBytes, root, vp.CensusProof.MerkleProof)
	if err != nil {
		return err
	}
	if !v {
		return fmt.Errorf("merkleproof verification failed")
	}
	return nil
}

// Verify checks the signature and merkleproof of the VotePackage
func (vp *VotePackage) Verify(chainID, processID uint64, root []byte) error {
	if err := vp.verifySignature(chainID, processID); err != nil {
		return err
	}
	if err := vp.verifyMerkleProof(root); err != nil {
		return err
	}
	// TODO ensure that vote value is 0 or 1
	return nil
}

// Uint64ToIndex returns the bytes representation of the given uint64 that will
// be used as a leaf index in the MerkleTree
func Uint64ToIndex(u uint64) []byte {
	return arbo.BigIntToBytes(MaxKeyLen, big.NewInt(int64(int(u)))) //nolint:gomnd
}

// HashPubKBytes returns the bytes representation of the Poseidon hash of the
// given PublicKey together with its weight, that will be used as a leaf value
// in the MerkleTree. If no weight is provided (eg. nil), a weight of 1 is
// assigned.
func HashPubKBytes(pubK *babyjub.PublicKey, weight *big.Int) ([]byte, error) {
	if weight == nil {
		weight = big.NewInt(1)
	}
	pubKHash, err := poseidon.Hash([]*big.Int{pubK.X, pubK.Y, weight})
	if err != nil {
		return nil, err
	}
	return arbo.BigIntToBytes(HashLen, pubKHash), nil
}

//
// // SignatureCompressedSize sets the size of the compressed Signature byte array
// const SignatureCompressedSize = 64
//
// // Signature contains a babyjubjub compressed Signature
// type Signature [SignatureCompressedSize]byte

// HexToPublicKey converts the given hex representation of a
// babyjub.PublicKeyComp, and returns the babyjub.PublicKey
func HexToPublicKey(h string) (*babyjub.PublicKey, error) {
	// pubKStr := c.Param("pubkey")
	// var pubK babyjub.PublicKey
	// err = json.Unmarshal([]byte(pubKStr), &pubK)
	// if err != nil {
	//         returnErr(c, err)
	//         return
	// }

	pubKCompBytes, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	var pubKComp babyjub.PublicKeyComp
	if len(pubKComp[:]) != len(pubKCompBytes) {
		return nil, fmt.Errorf("unexpected pubK length: %d", len(pubKCompBytes))
	}
	copy(pubKComp[:], pubKCompBytes)
	pubK, err := pubKComp.Decompress()
	if err != nil {
		return nil, err
	}

	return pubK, nil
}

// IndexAndWeightToBytes returns a byte array containing the given index and
// weight, encoded as:
// [   8   |   32   ]
// [ index | weight ]
func IndexAndWeightToBytes(index uint64, weight *big.Int) []byte {
	indexBytes := Uint64ToIndex(index)
	weightBytes := arbo.BigIntToBytes(32, weight) //nolint:gomnd
	var b [8 + 32]byte
	copy(b[:8], indexBytes)
	copy(b[8:], weightBytes)
	return b[:]
}

// BytesToIndexAndWeight returns the index and weight from the given byte array
func BytesToIndexAndWeight(b []byte) (uint64, *big.Int, error) {
	if len(b) != 40 { //nolint:gomnd // 40=8+32
		return 0, nil, fmt.Errorf("len(b) should be 40")
	}
	index := binary.LittleEndian.Uint64(b[:8])
	weight := arbo.BytesToBigInt(b[8:])
	return index, weight, nil
}
