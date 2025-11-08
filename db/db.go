package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// Executor defines the minimal interface for executing database operations.
type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// DB represents a database connection that implements the Executor interface.
// It can be passed to repository constructors to create repository instances.
type DB struct {
	conn *sql.DB
}

// Exec executes a query without returning any rows.
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

// Query executes a query that returns rows.
func (db *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
func (db *DB) QueryRow(query string, args ...any) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

func NewSqliteDB(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tracks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		spotify_id TEXT UNIQUE NOT NULL,
		track_name TEXT NOT NULL,
		artist TEXT NOT NULL,
		album TEXT NOT NULL,
		added_at DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		search_id INTEGER,
		search_time DATETIME,
		download_username TEXT,
		download_filename TEXT,
		download_path TEXT,
		error_msg TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_tracks_spotify_id ON tracks(spotify_id);
	CREATE INDEX IF NOT EXISTS idx_tracks_status ON tracks(status);
	CREATE INDEX IF NOT EXISTS idx_tracks_added_at ON tracks(added_at DESC);

	CREATE TABLE IF NOT EXISTS downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		filename TEXT NOT NULL,
		size INTEGER NOT NULL,
		status TEXT NOT NULL,
		bytes_received INTEGER NOT NULL DEFAULT 0,
		queue_position INTEGER,
		error TEXT,
		token INTEGER,
		created_at DATETIME NOT NULL,
		completed_at DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(username, filename)
	);

	CREATE INDEX IF NOT EXISTS idx_downloads_username ON downloads(username);
	CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
	CREATE INDEX IF NOT EXISTS idx_downloads_token ON downloads(token);
	CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at DESC);

	CREATE TABLE IF NOT EXISTS uploads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		filename TEXT NOT NULL,
		real_path TEXT,
		size INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL,
		bytes_sent INTEGER NOT NULL DEFAULT 0,
		token INTEGER,
		error TEXT,
		created_at DATETIME NOT NULL,
		completed_at DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(username, filename)
	);

	CREATE INDEX IF NOT EXISTS idx_uploads_username ON uploads(username);
	CREATE INDEX IF NOT EXISTS idx_uploads_status ON uploads(status);
	CREATE INDEX IF NOT EXISTS idx_uploads_token ON uploads(token);
	CREATE INDEX IF NOT EXISTS idx_uploads_created_at ON uploads(created_at DESC);
	`

	_, err := db.conn.Exec(schema)
	if err != nil {
		return err
	}

	// Add download columns if they don't exist (migration)
	migrations := []string{
		`ALTER TABLE tracks ADD COLUMN download_username TEXT`,
		`ALTER TABLE tracks ADD COLUMN download_filename TEXT`,
	}

	for _, migration := range migrations {
		_, err := db.conn.Exec(migration)
		// Ignore errors for columns that already exist
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			log.Printf("Migration warning: %v", err)
		}
	}

	return nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Failed to get user home directory: %v", err)
		home = "."
	}
	spotseekDir := filepath.Join(home, ".spotseek")
	return filepath.Join(spotseekDir, "tracks.db")
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
