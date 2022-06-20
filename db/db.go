package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// TODO unify naming of methods (Store/Set/Add, Get/Read/etc)

var (
	// ErrMetaNotInDB is used to indicate when metadata (which includes
	// lastSyncBlockNum) is not stored in the db
	ErrMetaNotInDB = fmt.Errorf("Meta does not exist in the db")
)

// SQLite represents the SQLite database
type SQLite struct {
	db *sql.DB
}

// NewSQLite returns a new *SQLite database
func NewSQLite(db *sql.DB) *SQLite {
	return &SQLite{
		db: db,
	}
}

// Migrate creates the tables needed for the database
func (r *SQLite) Migrate() error {
	query := `
	PRAGMA foreign_keys = ON;
	`
	_, err := r.db.Exec(query)
	if err != nil {
		return err
	}

	query = `
	CREATE TABLE IF NOT EXISTS processes(
		id INTEGER NOT NULL PRIMARY KEY UNIQUE,
		status INTEGER NOT NULL,
		censusRoot BLOB NOT NULL,
		censusSize INTEGER NOT NULL,
		ethBlockNum INTEGER NOT NULL,
		resPubStartBlock INTEGER NOT NULL,
		resPubWindow INTEGER NOT NULL,
		minParticipation INTEGER NOT NULL,
		type INTEGER NOT NULL,
		insertedDatetime DATETIME
	);
	`
	_, err = r.db.Exec(query)
	if err != nil {
		return err
	}

	query = `
	CREATE TABLE IF NOT EXISTS votepackages(
		indx INTEGER NOT NULL PRIMARY KEY UNIQUE,
		publicKey BLOB NOT NULL UNIQUE,
		weight BLOB NOT NULL,
		merkleproof BLOB NOT NULL UNIQUE,
		signature BLOB NOT NULL,
		vote BLOB NOT NULL,
		insertedDatetime DATETIME,
		processID INTEGER NOT NULL,
		FOREIGN KEY(processID) REFERENCES processes(id)
	);
	`
	_, err = r.db.Exec(query)
	if err != nil {
		return err
	}

	query = `
	CREATE TABLE IF NOT EXISTS proofs(
		proofid INTEGER NOT NULL PRIMARY KEY UNIQUE,
		proof BLOB NOT NULL,
		publicInputs BLOB NOT NULL,
		insertedDatetime DATETIME,
		proofAddedDatetime DATETIME,
		processID INTEGER NOT NULL,
		FOREIGN KEY(processID) REFERENCES processes(id)
	);
	`
	_, err = r.db.Exec(query)
	if err != nil {
		return err
	}

	query = `
	CREATE TABLE IF NOT EXISTS meta(
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		chainID INTEGER NOT NULL,
		lastSyncBlockNum INTEGER NOT NULL,
		lastUpdate DATETIME
	);
	`
	_, err = r.db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

// InitMeta initializes the meta table with the given chainID
func (r *SQLite) InitMeta(chainID, lastSyncBlockNum uint64) error {
	sqlQuery := `
	INSERT INTO meta(
		chainID,
		lastSyncBlockNum,
		lastUpdate
	) values(?, ?, CURRENT_TIMESTAMP)
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return err
	}
	defer stmt.Close() //nolint:errcheck

	_, err = stmt.Exec(chainID, lastSyncBlockNum)
	if err != nil {
		return fmt.Errorf("InitMeta error: %s", err)
	}
	return nil
}

// UpdateLastSyncBlockNum stores the given lastSyncBlockNum into the meta
// unique row
func (r *SQLite) UpdateLastSyncBlockNum(lastSyncBlockNum uint64) error {
	sqlQuery := `
	UPDATE meta SET lastSyncBlockNum=? WHERE id=?
	`

	stmt, err := r.db.Prepare(sqlQuery)
	if err != nil {
		return fmt.Errorf("UpdateLastSyncBlockNum error: %s", err)
	}
	defer stmt.Close() //nolint:errcheck

	_, err = stmt.Exec(int(lastSyncBlockNum), 1)
	if err != nil {
		return fmt.Errorf("UpdateLastSyncBlockNum error: %s", err)
	}
	return nil
}

// GetLastSyncBlockNum gets the lastSyncBlockNum from the meta unique row
func (r *SQLite) GetLastSyncBlockNum() (uint64, error) {
	row := r.db.QueryRow("SELECT lastSyncBlockNum FROM meta WHERE id = 1")

	var lastSyncBlockNum int
	err := row.Scan(&lastSyncBlockNum)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrMetaNotInDB
		}
		return 0, err
	}
	return uint64(lastSyncBlockNum), nil
}

// func (r *SQLite) ReadVotePackagesByCensusRoot(processID uint64) ([]types.VotePackage, error) {
// func (r *SQLite) ReadVoteByPublicKeyAndCensusRoot(censusRoot []byte) (
// 	[]types.VotePackage, error) {
// func (r *SQLite) ReadVotesByPublicKey(censusRoot []byte) ([]types.VotePackage, error) {
