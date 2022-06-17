package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/groupoidlabs/ovote-node/types"
)

// StoreProcess stores a new process with the given id, censusRoot and
// ethBlockNum. When a new process is stored, it's assumed that it comes from
// the SmartContract, and its status is set to types.ProcessStatusOn
func (r *SQLite) StoreProcess(id uint64, censusRoot []byte, censusSize,
	ethBlockNum, resPubStartBlock, resPubWindow uint64, minParticipation,
	minPositiveVotes, typ uint8) error {
	sqlQuery := `
	INSERT INTO processes(
		id,
		status,
		censusRoot,
		censusSize,
		ethBlockNum,
		resPubStartBlock,
		resPubWindow,
		minParticipation,
		minPositiveVotes,
		type,
		insertedDatetime
	) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	_, err = stmt.Exec(id, types.ProcessStatusOn, censusRoot, censusSize,
		ethBlockNum, resPubStartBlock, resPubWindow, minParticipation,
		minPositiveVotes, typ)
	if err != nil {
		return err
	}
	return nil
}

// UpdateProcessStatus sets the given types.ProcessStatus for the given id.
// This method should only be called when updating from SmartContracts.
func (r *SQLite) UpdateProcessStatus(id uint64, status types.ProcessStatus) error {
	sqlQuery := `
	UPDATE processes SET status=? WHERE id=?
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	_, err = stmt.Exec(int(status), id)
	if err != nil {
		return err
	}
	return nil
}

// GetProcessStatus returns the stored types.ProcessStatus for the given id
func (r *SQLite) GetProcessStatus(id uint64) (types.ProcessStatus, error) {
	row := r.db.QueryRow("SELECT status FROM processes WHERE id = ?", id)

	var status int
	err := row.Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("ProcessID: %d, does not exist in the db", id)
		}
		return 0, err
	}
	return types.ProcessStatus(status), nil
}

// ReadProcessByID reads the types.Process by the given id
func (r *SQLite) ReadProcessByID(id uint64) (*types.Process, error) {
	row := r.db.QueryRow("SELECT * FROM processes WHERE id = ?", id)

	var process types.Process
	err := row.Scan(&process.ID, &process.Status, &process.CensusRoot,
		&process.CensusSize, &process.EthBlockNum, &process.ResPubStartBlock,
		&process.ResPubWindow, &process.MinParticipation,
		&process.MinPositiveVotes, &process.Type, &process.InsertedDatetime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("ProcessID: %d, does not exist in the db", id)
		}
		return nil, err
	}
	return &process, nil
}

// ReadProcesses reads all the stored types.Process
func (r *SQLite) ReadProcesses() ([]types.Process, error) {
	sqlQuery := `
	SELECT * FROM processes	ORDER BY datetime(insertedDatetime) DESC
	`
	// TODO maybe, in all affected methods, order by EthBlockNum (creation)
	// instead of insertedDatetime.

	rows, err := r.db.Query(sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var processes []types.Process
	for rows.Next() {
		process := types.Process{}
		err = rows.Scan(&process.ID, &process.Status,
			&process.CensusRoot, &process.CensusSize, &process.EthBlockNum,
			&process.ResPubStartBlock, &process.ResPubWindow,
			&process.MinParticipation, &process.MinPositiveVotes,
			&process.Type, &process.InsertedDatetime)
		if err != nil {
			return nil, err
		}
		processes = append(processes, process)
	}
	return processes, nil
}

// FrozeProcessesByCurrentBlockNum sets the process status to
// ProcessStatusFrozen for all the processes that: have their
// status==ProcessStatusOn and that their ResPubStartBlock <= currentBlockNum.
// This method is intended to be used by the eth.Client when synchronizing
// processes to the last block number.
func (r *SQLite) FrozeProcessesByCurrentBlockNum(currBlockNum uint64) error {
	sqlQuery := `
	UPDATE processes
	SET status = ?
	WHERE (resPubStartBlock <= ? AND status = ?)
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	_, err = stmt.Exec(types.ProcessStatusFrozen,
		int(currBlockNum), types.ProcessStatusOn)
	if err != nil {
		return err
	}
	return nil
}

// ReadProcessesByResPubStartBlock reads all the stored processes which contain
// the given ResPubStartBlock
func (r *SQLite) ReadProcessesByResPubStartBlock(resPubStartBlock uint64) (
	[]types.Process, error) {
	sqlQuery := `
	SELECT * FROM processes WHERE resPubStartBlock = ?
	ORDER BY datetime(resPubStartBlock) DESC
	`

	rows, err := r.db.Query(sqlQuery, resPubStartBlock)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var processes []types.Process
	for rows.Next() {
		process := types.Process{}
		err = rows.Scan(&process.ID, &process.Status,
			&process.CensusRoot, &process.CensusSize, &process.EthBlockNum,
			&process.ResPubStartBlock, &process.ResPubWindow,
			&process.MinParticipation, &process.MinPositiveVotes,
			&process.Type, &process.InsertedDatetime)
		if err != nil {
			return nil, err
		}
		processes = append(processes, process)
	}
	return processes, nil
}

// ReadProcessesByStatus reads all the stored processes which have the given
// status
func (r *SQLite) ReadProcessesByStatus(status types.ProcessStatus) ([]types.Process, error) {
	sqlQuery := `
	SELECT * FROM processes WHERE status = ?
	ORDER BY datetime(insertedDatetime) DESC
	`

	rows, err := r.db.Query(sqlQuery, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var processes []types.Process
	for rows.Next() {
		process := types.Process{}
		err = rows.Scan(&process.ID, &process.Status,
			&process.CensusRoot, &process.CensusSize, &process.EthBlockNum,
			&process.ResPubStartBlock, &process.ResPubWindow,
			&process.MinParticipation, &process.MinPositiveVotes,
			&process.Type, &process.InsertedDatetime)
		if err != nil {
			return nil, err
		}
		processes = append(processes, process)
	}
	return processes, nil
}
