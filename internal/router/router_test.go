package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xentry/xentry/internal/auth"
	"github.com/xentry/xentry/internal/db"
	"github.com/xentry/xentry/internal/org"
	"github.com/xentry/xentry/internal/project"
)

func setupTestRouter(t *testing.T) (*Services, *db.SQLite) {
	t.Helper()
	dir := t.TempDir()
	store, err := db.NewSQLite(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	authSvc := auth.NewService("test-secret")
	authSvc.SetDB(store.DB())
	orgSvc := org.NewService(store.DB())
	projSvc := project.NewService(store.DB())

	return &Services{
		DB:      store,
		Auth:    authSvc,
		Org:     orgSvc,
		Project: projSvc,
	}, store
}

func TestRouter_RegisterAndLogin(t *testing.T) {
	svc, _ := setupTestRouter(t)
	r := New(svc)

	// Register
	regBody, _ := json.Marshal(map[string]string{
		"email": "test@example.com", "password": "secret123", "name": "Test User",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Login with same credentials
	loginBody, _ := json.Marshal(map[string]string{
		"email": "test@example.com", "password": "secret123",
	})
	req2 := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var loginResp map[string]string
	json.NewDecoder(rec2.Body).Decode(&loginResp)
	if loginResp["token"] == "" {
		t.Error("expected token in login response")
	}
}

func TestRouter_ProtectedRoute(t *testing.T) {
	svc, _ := setupTestRouter(t)
	r := New(svc)

	// Access protected route without token
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRouter_CreateOrg(t *testing.T) {
	svc, _ := setupTestRouter(t)
	r := New(svc)

	// First register a user and get token
	regBody, _ := json.Marshal(map[string]string{
		"email": "orguser@example.com", "password": "pass", "name": "OrgUser",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var regResp map[string]string
	json.NewDecoder(rec.Body).Decode(&regResp)
	token := regResp["token"]

	// Create org with token
	orgBody, _ := json.Marshal(map[string]string{"name": "My Org", "slug": "my-org"})
	req2 := httptest.NewRequest("POST", "/api/organizations", bytes.NewReader(orgBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK && rec2.Code != http.StatusCreated {
		t.Fatalf("create org: expected 200/201, got %d: %s", rec2.Code, rec2.Body.String())
	}
}
