package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/aragonzkresearch/ovote-node/types"
)

// ErrProofNotInDB is used to indicate when the proof does not exist in the db.
var ErrProofNotInDB = "Proof does not exist in db"

// StoreProofID stores the given proofID for the given processID.  This method
// should be called only from a prover-server response.
func (r *SQLite) StoreProofID(processID, proofID uint64) error {
	sqlQuery := `
	INSERT INTO proofs(
		proofid,
		proof,
		publicInputs,
		insertedDatetime,
		proofAddedDatetime,
		processID
	) values(?, ?, ?, CURRENT_TIMESTAMP, ?, ?)
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	emptyBytes := []byte{}
	_, err = stmt.Exec(proofID, emptyBytes, emptyBytes, time.Time{}, processID)
	if err != nil {
		if err.Error() == "FOREIGN KEY constraint failed" {
			return fmt.Errorf("Can not store Proof, ProcessID=%d does not exist",
				processID)
		}
		return err
	}
	return nil
}

// AddProofToProofID stores the proof & publicInputs bytes for the given
// processID and proofID. Important: if the processID & proofID does not exist
// in the db yet, this method will not return any error, but will not store the
// data.
func (r *SQLite) AddProofToProofID(processID, proofID uint64, proof, publicInputs []byte) error {
	sqlQuery := `
	UPDATE proofs
	SET proof = ?, publicInputs = ?, proofAddedDatetime = CURRENT_TIMESTAMP
	WHERE (processID = ? AND proofID = ?)
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	_, err = stmt.Exec(proof, publicInputs, processID, proofID)
	if err != nil {
		return err
	}
	return nil
}

// GetProofByProcessID returns the last stored proof (by proof & publicInputs
// addition time) for a given ProcessID
func (r *SQLite) GetProofByProcessID(processID uint64) (*types.ProofInDB, error) {
	row := r.db.QueryRow(
		"SELECT * FROM proofs WHERE processID = ? ORDER BY proofAddedDatetime DESC LIMIT 1;",
		processID)

	var proof types.ProofInDB
	err := row.Scan(&proof.ProofID, &proof.Proof, &proof.PublicInputs,
		&proof.InsertedDatetime, &proof.ProofAddedDatetime, &proof.ProcessID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil,
				fmt.Errorf("%s, ProcessID: %d", ErrProofNotInDB, processID)
		}
		return nil, err
	}
	return &proof, nil
}

// GetProofsByProcessID returns the stored proofs for a given ProcessID
func (r *SQLite) GetProofsByProcessID(processID uint64) ([]types.ProofInDB, error) {
	rows, err := r.db.Query(
		"SELECT * FROM proofs WHERE processID = ? ORDER BY proofAddedDatetime DESC",
		processID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var proofs []types.ProofInDB
	for rows.Next() {
		proof := types.ProofInDB{}
		err = rows.Scan(&proof.ProofID, &proof.Proof,
			&proof.PublicInputs, &proof.InsertedDatetime,
			&proof.ProofAddedDatetime, &proof.ProcessID)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}
