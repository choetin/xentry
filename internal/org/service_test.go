package org

import (
	"database/sql"
	"testing"

	"github.com/xentry/xentry/internal/db"
)

func setupTestDB(t *testing.T) *db.SQLite {
	t.Helper()
	dir := t.TempDir()
	store, err := db.NewSQLite(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func seedUser(t *testing.T, db *sql.DB) {
	t.Helper()
	db.Exec("INSERT INTO users (id, email, password_hash, name) VALUES ('user-1', 'test@test.com', 'hash', 'Test')")
}

func TestCreateOrganization(t *testing.T) {
	s := setupTestDB(t)
	svc := NewService(s.DB())
	seedUser(t, s.DB())
	org, err := svc.Create("My Team", "my-team", "user-1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if org.ID == "" {
		t.Error("expected non-empty ID")
	}
	if org.Slug != "my-team" {
		t.Errorf("expected slug my-team, got %s", org.Slug)
	}
}

func TestCreateOrganization_DuplicateSlug(t *testing.T) {
	s := setupTestDB(t)
	svc := NewService(s.DB())
	seedUser(t, s.DB())
	svc.Create("My Team", "my-team", "user-1")
	_, err := svc.Create("Another Team", "my-team", "user-1")
	if err == nil {
		t.Error("expected error for duplicate slug")
	}
}

func TestAddMember(t *testing.T) {
	s := setupTestDB(t)
	svc := NewService(s.DB())
	seedUser(t, s.DB())
	s.DB().Exec("INSERT INTO users (id, email, password_hash, name) VALUES ('user-2', 'test2@test.com', 'hash', 'Test2')")
	org, _ := svc.Create("Team", "team", "user-1")
	err := svc.AddMember(org.ID, "user-2", "admin")
	if err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}
}

func TestListOrganizations(t *testing.T) {
	s := setupTestDB(t)
	svc := NewService(s.DB())
	seedUser(t, s.DB())
	svc.Create("Team A", "team-a", "user-1")
	svc.Create("Team B", "team-b", "user-1")
	orgs, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(orgs) != 2 {
		t.Errorf("expected 2 orgs, got %d", len(orgs))
	}
}

func TestGetOrganization(t *testing.T) {
	s := setupTestDB(t)
	svc := NewService(s.DB())
	seedUser(t, s.DB())
	created, _ := svc.Create("Test", "test", "user-1")
	fetched, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Name != "Test" {
		t.Errorf("expected Test, got %s", fetched.Name)
	}
}

func TestSoftDeleteOrg(t *testing.T) {
	s := setupTestDB(t)
	svc := NewService(s.DB())
	seedUser(t, s.DB())
	org, _ := svc.Create("ToDelete", "to-delete", "user-1")

	err := svc.SoftDelete(org.ID)
	if err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	// Org should no longer appear in GetByID
	_, err = svc.GetByID(org.ID)
	if err == nil {
		t.Error("expected error after soft delete, got nil")
	}

	// Org should no longer appear in List
	orgs, _ := svc.ListByUser("user-1")
	for _, o := range orgs {
		if o.ID == org.ID {
			t.Error("soft-deleted org should not appear in ListByUser")
		}
	}

	// Org should no longer appear in global List
	allOrgs, _ := svc.List()
	for _, o := range allOrgs {
		if o.ID == org.ID {
			t.Error("soft-deleted org should not appear in List")
		}
	}
}
