package project

import (
	"testing"

	"github.com/xentry/xentry/internal/db"
	"github.com/xentry/xentry/internal/org"
)

func setupTestDBWithOrg(t *testing.T) (*db.SQLite, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := db.NewSQLite(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	store.DB().Exec("INSERT INTO users (id, email, password_hash, name) VALUES ('user-1', 'test@test.com', 'hash', 'Test')")
	orgSvc := org.NewService(store.DB())
	o, _ := orgSvc.Create("Test Org", "test-org", "user-1")
	return store, o.ID
}

func TestCreateProject(t *testing.T) {
	store, orgID := setupTestDBWithOrg(t)
	svc := NewService(store.DB())
	p, err := svc.Create(orgID, "My App", "my-app", "windows")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if p.ID == "" {
		t.Error("expected non-empty ID")
	}
	if p.DSNToken == "" {
		t.Error("expected non-empty DSN token")
	}
	if p.Platform != "windows" {
		t.Errorf("expected windows, got %s", p.Platform)
	}
}

func TestListProjects(t *testing.T) {
	store, orgID := setupTestDBWithOrg(t)
	svc := NewService(store.DB())
	svc.Create(orgID, "App A", "app-a", "ios")
	svc.Create(orgID, "App B", "app-b", "android")
	projects, err := svc.ListByOrg(orgID)
	if err != nil {
		t.Fatalf("ListByOrg failed: %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestGetProjectByDSN(t *testing.T) {
	store, orgID := setupTestDBWithOrg(t)
	svc := NewService(store.DB())
	created, _ := svc.Create(orgID, "App", "app", "linux")
	fetched, err := svc.GetByDSNToken(created.DSNToken)
	if err != nil {
		t.Fatalf("GetByDSNToken failed: %v", err)
	}
	if fetched.ID != created.ID {
		t.Error("DSN lookup returned wrong project")
	}
}

func TestCreateAPIToken(t *testing.T) {
	store, orgID := setupTestDBWithOrg(t)
	svc := NewService(store.DB())
	p, _ := svc.Create(orgID, "App", "app", "windows")
	token, err := svc.CreateAPIToken(p.ID, "my-token")
	if err != nil {
		t.Fatalf("CreateAPIToken failed: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if len(token) < 10 {
		t.Error("token seems too short")
	}
}

func TestSoftDeleteProject(t *testing.T) {
	store, orgID := setupTestDBWithOrg(t)
	svc := NewService(store.DB())
	p, _ := svc.Create(orgID, "ToDelete", "to-delete", "windows")

	err := svc.SoftDelete(p.ID)
	if err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}

	// Project should no longer appear in GetByID
	_, err = svc.GetByID(p.ID)
	if err == nil {
		t.Error("expected error after soft delete, got nil")
	}

	// Project should no longer appear in ListByOrg
	projects, _ := svc.ListByOrg(orgID)
	for _, proj := range projects {
		if proj.ID == p.ID {
			t.Error("soft-deleted project should not appear in ListByOrg")
		}
	}

	// DSN lookup should fail
	_, err = svc.GetByDSNToken(p.DSNToken)
	if err == nil {
		t.Error("DSN lookup should fail for soft-deleted project")
	}
}
