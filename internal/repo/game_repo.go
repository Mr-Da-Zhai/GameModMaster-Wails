package repo

import (
	"database/sql"
	"fmt"

	"GameModMaster/internal/model"
)

type GameRepo struct {
	db *DB
}

// NewGameRepo creates a new GameRepo
func NewGameRepo(db *DB) *GameRepo {
	return &GameRepo{db: db}
}

// GetByID returns a game by its primary key
func (r *GameRepo) GetByID(id int32) (*model.Game, error) {
	const query = `SELECT id, source_id, name_en, name_local, cover_url, source_url, options_num, updated_at
	               FROM games WHERE id = ?`

	row := r.db.QueryRow(query, id)
	g, err := scanGame(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get game by id %d: %w", id, err)
	}
	return g, nil
}

// GetAll returns all games ordered by updated_at DESC
func (r *GameRepo) GetAll() ([]*model.Game, error) {
	const query = `SELECT id, source_id, name_en, name_local, cover_url, source_url, options_num, updated_at
	               FROM games ORDER BY updated_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get all games: %w", err)
	}
	defer rows.Close()

	var games []*model.Game
	for rows.Next() {
		g, err := scanGameRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// GetBySourceID returns a game by its source identifier
func (r *GameRepo) GetBySourceID(sourceID string) (*model.Game, error) {
	const query = `SELECT id, source_id, name_en, name_local, cover_url, source_url, options_num, updated_at
	               FROM games WHERE source_id = ?`

	row := r.db.QueryRow(query, sourceID)
	g, err := scanGame(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get game by source_id %q: %w", sourceID, err)
	}
	return g, nil
}

// Search searches games by name_en or name_local using LIKE
func (r *GameRepo) Search(query string, limit int) ([]*model.Game, error) {
	const q = `SELECT id, source_id, name_en, name_local, cover_url, source_url, options_num, updated_at
	           FROM games
	           WHERE name_en LIKE ? OR name_local LIKE ?
	           ORDER BY updated_at DESC
	           LIMIT ?`

	pattern := "%" + query + "%"
	rows, err := r.db.Query(q, pattern, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("search games %q: %w", query, err)
	}
	defer rows.Close()

	var games []*model.Game
	for rows.Next() {
		g, err := scanGameRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

// BatchUpsert inserts or replaces multiple games in a transaction.
// It uses source_id as the conflict key to update existing records.
func (r *GameRepo) BatchUpsert(games []*model.Game) error {
	if len(games) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// ON CONFLICT(source_id) DO UPDATE — NOT INSERT OR REPLACE.
	// games is the parent of trainers (FK ON DELETE CASCADE), and trainers
	// is the parent of trainer_states (also CASCADE). INSERT OR REPLACE
	// would delete+reinsert the games row, cascading to wipe every trainer
	// and its download state on every refresh. ON CONFLICT keeps the row
	// (and its id) in place so no CASCADE fires.
	const upsertSQL = `INSERT INTO games (id, source_id, name_en, name_local, cover_url, source_url, options_num, updated_at)
	                   VALUES (
	                       (SELECT id FROM games WHERE source_id = ?),
	                       ?, ?, ?, ?, ?, ?, ?
	                   )
	                   ON CONFLICT(source_id) DO UPDATE SET
	                       name_en = excluded.name_en,
	                       name_local = excluded.name_local,
	                       cover_url = excluded.cover_url,
	                       source_url = excluded.source_url,
	                       options_num = excluded.options_num,
	                       updated_at = excluded.updated_at`

	stmt, err := tx.Prepare(upsertSQL)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, g := range games {
		_, err := stmt.Exec(
			g.SourceID,
			g.SourceID,
			g.NameEN,
			g.NameLocal,
			g.CoverURL,
			g.SourceURL,
			g.OptionsNum,
			g.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("upsert game %q: %w", g.SourceID, err)
		}
	}

	return tx.Commit()
}

// scanGame scans a single game from a QueryRow
func scanGame(row *sql.Row) (*model.Game, error) {
	var g model.Game
	err := row.Scan(
		&g.ID, &g.SourceID, &g.NameEN, &g.NameLocal,
		&g.CoverURL, &g.SourceURL, &g.OptionsNum, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// scanGameRow scans a single game from a Rows cursor
func scanGameRow(rows *sql.Rows) (*model.Game, error) {
	var g model.Game
	err := rows.Scan(
		&g.ID, &g.SourceID, &g.NameEN, &g.NameLocal,
		&g.CoverURL, &g.SourceURL, &g.OptionsNum, &g.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &g, nil
}
