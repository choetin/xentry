// Package db provides SQLite database initialization and schema migration.
package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// SQLite wraps a *sql.DB with auto-migration support on construction.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens (or creates) the SQLite database at dbPath, enables WAL mode
// and foreign keys, then runs any pending schema migrations.
func NewSQLite(dbPath string) (*SQLite, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	store := &SQLite{db: db}
	if err := store.runMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return store, nil
}

// DB returns the underlying *sql.DB.
func (s *SQLite) DB() *sql.DB { return s.db }

// Close closes the database connection.
func (s *SQLite) Close() error { return s.db.Close() }

// runMigrations applies all embedded SQL migration files that have not yet been applied,
// tracked in the schema_migrations table.
func (s *SQLite) runMigrations() error {
	// Create migration tracking table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at INTEGER NOT NULL DEFAULT (unixepoch())
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Find applied migrations
	applied := map[string]bool{}
	rows, err := s.db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied[v] = true
	}

	// Read embedded SQL files
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var filenames []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			filenames = append(filenames, e.Name())
		}
	}
	sort.Strings(filenames)

	for _, name := range filenames {
		if applied[name] {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := s.db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", name, err)
		}
		if _, err := s.db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	return nil
}
