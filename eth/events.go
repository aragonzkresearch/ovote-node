package eth

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/vocdoni/arbo"
)

// eventNewProcess contains the data received from an event log of newProcess
type eventNewProcess struct {
	Creator          common.Address
	ProcessID        uint64
	TxHash           [32]byte
	CensusRoot       [32]byte
	CensusSize       uint64
	ResPubStartBlock uint64
	ResPubWindow     uint64
	MinParticipation uint8
	Type             uint8
}

// String implements the String interface for eventNewProcess
func (e *eventNewProcess) String() string {
	return fmt.Sprintf("[eventNewProcess]: Creator %s, ProcessID: %d, TxHash: %s,"+
		" CensusRoot: %s, Type: %d, CensusSize: %d, ResPubStartBlock: %d,"+
		" ResPubWindow: %d, MinParticipation: %d",
		e.Creator, e.ProcessID, hex.EncodeToString(e.TxHash[:]),
		arbo.BytesToBigInt(e.CensusRoot[:]), e.Type, e.CensusSize,
		e.ResPubStartBlock, e.ResPubWindow, e.MinParticipation)
}

func parseEventNewProcess(d []byte) (*eventNewProcess, error) {
	if len(d) != eventNewProcessLen {
		return nil, fmt.Errorf("newProcess event log should be of length %d, current: %d",
			eventNewProcessLen, len(d))
	}
	var e eventNewProcess

	// contract event:
	// event EventProcessCreated(address creator, uint256 id,uint256
	// transactionHash,  uint256 censusRoot, uint8 typ, uint64 censusSize,
	// uint64 resPubStartBlock, uint64 resPubWindow, uint8
	// minParticipation);

	creatorBytes := d[:32]
	e.Creator = common.BytesToAddress(creatorBytes[12:32])

	// WARNING for the moment is uint256 but probably will change to uint64
	// idBytes := new(big.Int).SetBytes(idBytes)
	idBytes := d[64-8 : 64] // uint64
	e.ProcessID = binary.BigEndian.Uint64(idBytes)

	copy(e.TxHash[:], d[64:96])

	// note that here Ethereum returns the CensusRoot in big endian
	copy(e.CensusRoot[:], arbo.SwapEndianness(d[96:128]))

	censusSizeBytes := d[160-8 : 160] // uint64
	e.CensusSize = binary.BigEndian.Uint64(censusSizeBytes)

	resPubStartBlockBytes := d[192-8 : 192] // uint64
	e.ResPubStartBlock = binary.BigEndian.Uint64(resPubStartBlockBytes)

	resPubWindowBytes := d[224-8 : 224] // uint64
	e.ResPubWindow = binary.BigEndian.Uint64(resPubWindowBytes)

	e.MinParticipation = uint8(d[255])
	e.Type = uint8(d[287])

	return &e, nil
}

type eventResultPublished struct {
	Publisher    common.Address
	ProcessID    uint64
	ReceiptsRoot [32]byte
	Result       uint64
	NVotes       uint64
}

// String implements the String interface for eventResultPublished
func (e *eventResultPublished) String() string {
	return fmt.Sprintf("[eventResultPublished]: Publisher %s, ProcessID: %d,"+
		" ReceiptsRoot: %s, Result: %d, NVotes: %d",
		e.Publisher, e.ProcessID,
		arbo.BytesToBigInt(e.ReceiptsRoot[:]), e.Result, e.NVotes)
}

func parseEventResultPublished(d []byte) (*eventResultPublished, error) {
	if len(d) != eventResultPublishedLen {
		return nil, fmt.Errorf("resultPublished event log should be of"+
			" length %d, current: %d", eventResultPublishedLen, len(d))
	}

	// event EventResultPublished(address publisher, uint256 id, uint256
	// receiptsRoot, uint64 result, uint64 nVotes);

	var e eventResultPublished

	publisherBytes := d[:32]
	e.Publisher = common.BytesToAddress(publisherBytes[12:32])

	idBytes := d[64-8 : 64] // uint64
	e.ProcessID = binary.BigEndian.Uint64(idBytes)

	// note that here Ethereum returns the CensusRoot in big endian
	copy(e.ReceiptsRoot[:], arbo.SwapEndianness(d[64:96]))

	result := d[128-8 : 128] // uint64
	e.Result = binary.BigEndian.Uint64(result)

	nVotes := d[160-8 : 160] // uint64
	e.NVotes = binary.BigEndian.Uint64(nVotes)

	return &e, nil
}

type eventProcessClosed struct {
	Caller    common.Address
	ProcessID uint64
	Success   bool
}

// String implements the String interface for eventProcessClosed
func (e *eventProcessClosed) String() string {
	return fmt.Sprintf("[eventProcessClosed]: Caller %s, ProcessID: %d, Success: %t",
		e.Caller, e.ProcessID, e.Success)
}

// event EventProcessClosed(address caller, uint256 id, bool success);
func parseEventProcessClosed(d []byte) (*eventProcessClosed, error) {
	if len(d) != eventProcessClosedLen {
		return nil, fmt.Errorf("processClosed event log should be of length %d, current: %d",
			eventProcessClosedLen, len(d))
	}

	var e eventProcessClosed

	creatorBytes := d[:32]
	e.Caller = common.BytesToAddress(creatorBytes[12:32])

	idBytes := d[64-8 : 64] // uint64
	e.ProcessID = binary.BigEndian.Uint64(idBytes)

	success := d[96-1 : 96] // uint64
	if success[0] == byte(1) {
		e.Success = true
	}

	return &e, nil
}
