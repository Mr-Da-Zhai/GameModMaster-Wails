package repo

import (
	"database/sql"
	"fmt"

	"GameModMaster/internal/model"
)

type TrainerRepo struct {
	db *DB
}

// NewTrainerRepo creates a new TrainerRepo
func NewTrainerRepo(db *DB) *TrainerRepo {
	return &TrainerRepo{db: db}
}

// GetByID returns a trainer by its primary key
func (r *TrainerRepo) GetByID(id int32) (*model.Trainer, error) {
	const query = `SELECT id, game_id, version, game_version, download_url, file_size, file_name, download_count, source_hash, updated_at
	               FROM trainers WHERE id = ?`

	row := r.db.QueryRow(query, id)
	t, err := scanTrainer(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get trainer by id %d: %w", id, err)
	}
	return t, nil
}

// GetByGameID returns all trainers for a given game, ordered by updated_at DESC
func (r *TrainerRepo) GetByGameID(gameID int32) ([]*model.Trainer, error) {
	const query = `SELECT id, game_id, version, game_version, download_url, file_size, file_name, download_count, source_hash, updated_at
	               FROM trainers WHERE game_id = ? ORDER BY updated_at DESC`

	rows, err := r.db.Query(query, gameID)
	if err != nil {
		return nil, fmt.Errorf("get trainers by game_id %d: %w", gameID, err)
	}
	defer rows.Close()

	var trainers []*model.Trainer
	for rows.Next() {
		t, err := scanTrainerRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan trainer: %w", err)
		}
		trainers = append(trainers, t)
	}
	return trainers, rows.Err()
}

// GetBySourceHash returns a trainer by its source hash
func (r *TrainerRepo) GetBySourceHash(hash string) (*model.Trainer, error) {
	const query = `SELECT id, game_id, version, game_version, download_url, file_size, file_name, download_count, source_hash, updated_at
	               FROM trainers WHERE source_hash = ?`

	row := r.db.QueryRow(query, hash)
	t, err := scanTrainer(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get trainer by source_hash %q: %w", hash, err)
	}
	return t, nil
}

// BatchUpsert inserts or replaces multiple trainers in a transaction.
// It uses source_hash as the conflict key to update existing records.
func (r *TrainerRepo) BatchUpsert(trainers []*model.Trainer) error {
	if len(trainers) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	const upsertSQL = `INSERT OR REPLACE INTO trainers (id, game_id, version, game_version, download_url, file_size, file_name, download_count, source_hash, updated_at)
	                   VALUES (
	                       (SELECT id FROM trainers WHERE source_hash = ?),
	                       ?, ?, ?, ?, ?, ?, ?, ?, ?
	                   )`

	stmt, err := tx.Prepare(upsertSQL)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, t := range trainers {
		_, err := stmt.Exec(
			t.SourceHash,
			t.GameID,
			t.Version,
			t.GameVersion,
			t.DownloadURL,
			t.FileSize,
			t.FileName,
			t.DownloadCount,
			t.SourceHash,
			t.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("upsert trainer %q: %w", t.SourceHash, err)
		}
	}

	return tx.Commit()
}

// scanTrainer scans a single trainer from a QueryRow
func scanTrainer(row *sql.Row) (*model.Trainer, error) {
	var t model.Trainer
	err := row.Scan(
		&t.ID, &t.GameID, &t.Version, &t.GameVersion,
		&t.DownloadURL, &t.FileSize, &t.FileName, &t.DownloadCount,
		&t.SourceHash, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// scanTrainerRow scans a single trainer from a Rows cursor
func scanTrainerRow(rows *sql.Rows) (*model.Trainer, error) {
	var t model.Trainer
	err := rows.Scan(
		&t.ID, &t.GameID, &t.Version, &t.GameVersion,
		&t.DownloadURL, &t.FileSize, &t.FileName, &t.DownloadCount,
		&t.SourceHash, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
