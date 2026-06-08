package repo

import (
	"database/sql"
	"fmt"

	"GameModMaster/internal/model"
)

type StateRepo struct {
	db *DB
}

// NewStateRepo creates a new StateRepo
func NewStateRepo(db *DB) *StateRepo {
	return &StateRepo{db: db}
}

// GetByTrainerID returns the trainer state for the given trainer ID
func (r *StateRepo) GetByTrainerID(trainerID int32) (*model.TrainerState, error) {
	const query = `SELECT trainer_id, status, local_path, installed_at, launched_at
	               FROM trainer_states WHERE trainer_id = ?`

	row := r.db.QueryRow(query, trainerID)
	s, err := scanState(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get state by trainer_id %d: %w", trainerID, err)
	}
	return s, nil
}

// GetByStatus returns all trainer states with the given status
func (r *StateRepo) GetByStatus(status model.TrainerStatus) ([]*model.TrainerState, error) {
	const query = `SELECT trainer_id, status, local_path, installed_at, launched_at
	               FROM trainer_states WHERE status = ?`

	rows, err := r.db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("get states by status %d: %w", status, err)
	}
	defer rows.Close()

	var states []*model.TrainerState
	for rows.Next() {
		s, err := scanStateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan state: %w", err)
		}
		states = append(states, s)
	}
	return states, rows.Err()
}

// ListAll returns all trainer states
func (r *StateRepo) ListAll() ([]*model.TrainerState, error) {
	const query = `SELECT trainer_id, status, local_path, installed_at, launched_at
	               FROM trainer_states`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list all states: %w", err)
	}
	defer rows.Close()

	var states []*model.TrainerState
	for rows.Next() {
		s, err := scanStateRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan state: %w", err)
		}
		states = append(states, s)
	}
	return states, rows.Err()
}

// Upsert inserts or replaces a trainer state
func (r *StateRepo) Upsert(state *model.TrainerState) error {
	const query = `INSERT OR REPLACE INTO trainer_states (trainer_id, status, local_path, installed_at, launched_at)
	               VALUES (?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query,
		state.TrainerID,
		state.Status,
		state.LocalPath,
		state.InstalledAt,
		state.LaunchedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert state for trainer %d: %w", state.TrainerID, err)
	}
	return nil
}

// scanState scans a single trainer state from a QueryRow
func scanState(row *sql.Row) (*model.TrainerState, error) {
	var s model.TrainerState
	err := row.Scan(
		&s.TrainerID, &s.Status, &s.LocalPath,
		&s.InstalledAt, &s.LaunchedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// scanStateRow scans a single trainer state from a Rows cursor
func scanStateRow(rows *sql.Rows) (*model.TrainerState, error) {
	var s model.TrainerState
	err := rows.Scan(
		&s.TrainerID, &s.Status, &s.LocalPath,
		&s.InstalledAt, &s.LaunchedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
