package db

import (
	"fmt"
	"math/big"

	"github.com/groupoidlabs/ovote-node/types"
)

// StoreVotePackage stores the given types.VotePackage for the given CensusRoot
func (r *SQLite) StoreVotePackage(processID uint64, vote types.VotePackage) error {
	// TODO check that processID exists
	sqlQuery := `
	INSERT INTO votepackages(
		indx,
		publicKey,
		weight,
		merkleproof,
		signature,
		vote,
		insertedDatetime,
		processID
	) values(?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	if vote.CensusProof.Weight == nil {
		// no weight defined, use 0
		vote.CensusProof.Weight = big.NewInt(0)
	}

	_, err = stmt.Exec(vote.CensusProof.Index, vote.CensusProof.PublicKey,
		vote.CensusProof.Weight.Bytes(), vote.CensusProof.MerkleProof,
		vote.Signature[:], vote.Vote, processID)
	if err != nil {
		if err.Error() == "FOREIGN KEY constraint failed" {
			return fmt.Errorf("Can not store VotePackage, ProcessID=%d does not exist", processID)
		}
		return err
	}
	return nil
}

// ReadVotePackagesByProcessID reads all the stored types.VotePackage for the
// given ProcessID. VotePackages returned are sorted by index parameter, from
// smaller to bigger.
func (r *SQLite) ReadVotePackagesByProcessID(processID uint64) ([]types.VotePackage, error) {
	// TODO add pagination
	sqlQuery := `
	SELECT signature, indx, publicKey, weight, merkleproof, vote FROM votepackages
	WHERE processID = ?
	ORDER BY indx ASC
	`

	rows, err := r.db.Query(sqlQuery, processID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var votes []types.VotePackage
	for rows.Next() {
		vote := types.VotePackage{}
		var sigBytes []byte
		var weightBytes []byte
		err = rows.Scan(&sigBytes, &vote.CensusProof.Index,
			&vote.CensusProof.PublicKey, &weightBytes,
			&vote.CensusProof.MerkleProof, &vote.Vote)
		if err != nil {
			return nil, err
		}
		vote.CensusProof.Weight = new(big.Int).SetBytes(weightBytes)
		copy(vote.Signature[:], sigBytes)
		votes = append(votes, vote)
	}
	return votes, nil
}
