package repo

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

const (
	pragmaWalMode     = "PRAGMA journal_mode=WAL;"
	pragmaNormalSync  = "PRAGMA synchronous=NORMAL;"
	pragmaForeignKeys = "PRAGMA foreign_keys=ON;"
	pragmaCacheSize   = "PRAGMA cache_size=-64000;" // 64MB cache
)

var schemaSQL = `
CREATE TABLE IF NOT EXISTS games (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id   TEXT    NOT NULL UNIQUE,
    name_en     TEXT    NOT NULL,
    name_local  TEXT    DEFAULT '',
    cover_url   TEXT    DEFAULT '',
    source_url  TEXT    DEFAULT '',
    options_num INTEGER DEFAULT 0,
    updated_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_games_name_en    ON games(name_en);
CREATE INDEX IF NOT EXISTS idx_games_name_local ON games(name_local);
CREATE INDEX IF NOT EXISTS idx_games_updated    ON games(updated_at DESC);

CREATE TABLE IF NOT EXISTS trainers (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id        INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    version        TEXT    DEFAULT '',
    game_version   TEXT    DEFAULT '',
    download_url   TEXT    DEFAULT '',
    file_size      INTEGER DEFAULT 0,
    file_name      TEXT    DEFAULT '',
    download_count INTEGER DEFAULT 0,
    source_hash    TEXT    NOT NULL UNIQUE,
    updated_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_trainers_game_id ON trainers(game_id);

CREATE TABLE IF NOT EXISTS trainer_states (
    trainer_id   INTEGER PRIMARY KEY REFERENCES trainers(id) ON DELETE CASCADE,
    status       INTEGER NOT NULL DEFAULT 0,
    local_path   TEXT    DEFAULT '',
    installed_at INTEGER DEFAULT 0,
    launched_at  INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS name_mapping (
    name_en  TEXT PRIMARY KEY,
    name_zh  TEXT NOT NULL,
    aliases  TEXT DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS kv_store (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
`

type DB struct {
	*sql.DB
}

// Open creates/opens the SQLite database at the given path
func Open(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Set connection pool
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	// Run pragmas
	for _, p := range []string{pragmaForeignKeys, pragmaNormalSync, pragmaCacheSize} {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	// Create schema
	if _, err := db.Exec(schemaSQL); err != nil {
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &DB{db}, nil
}

// Close closes the database
func (db *DB) Close() error {
	return db.DB.Close()
}
