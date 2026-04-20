package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite failed: %v", err)
	}
	defer store.Close()

	if store.DB() == nil {
		t.Fatal("expected non-nil DB handle")
	}

	// Verify core tables exist
	tables := []string{"users", "organizations", "org_members", "projects", "api_tokens", "issues", "events", "threads", "frames", "symbol_files", "symbols_cache", "transactions", "spans", "logs", "logs_fts", "releases"}
	for _, table := range tables {
		var name string
		err := store.DB().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q should exist: %v", table, err)
		}
	}
}

func TestNewSQLite_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "test.db")

	store, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite failed: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should be created")
	}
}
