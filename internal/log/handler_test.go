package log

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/xentry/xentry/internal/db"
	"github.com/xentry/xentry/internal/org"
	"github.com/xentry/xentry/internal/project"
)

func setupLogTest(t *testing.T) (*db.SQLite, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := db.NewSQLite(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	store.DB().Exec("INSERT INTO users (id, email, password_hash, name) VALUES ('user-1', 'test@test.com', 'hash', 'Test')")
	orgSvc := org.NewService(store.DB())
	o, _ := orgSvc.Create("Test", "test", "user-1")
	projSvc := project.NewService(store.DB())
	p, _ := projSvc.Create(o.ID, "App", "app", "windows")
	return store, p.ID
}

func TestIngestAndQueryLogs(t *testing.T) {
	store, projectID := setupLogTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc)

	router := chi.NewRouter()
	router.Post("/api/{projectID}/logs", h.IngestLogs)
	router.Get("/api/{projectID}/logs", h.QueryLogs)

	entries := []LogEntry{
		{Level: "info", Message: "Server started on port 8080", Logger: "main"},
		{Level: "error", Message: "Connection refused", Logger: "db"},
		{Level: "warn", Message: "Deprecated API call", Logger: "api"},
	}
	body, _ := json.Marshal(entries)
	req := httptest.NewRequest("POST", "/api/"+projectID+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("ingest: expected 202, got %d", rec.Code)
	}

	// Query all
	req2 := httptest.NewRequest("GET", "/api/"+projectID+"/logs", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("query: expected 200, got %d", rec2.Code)
	}
	var results []LogEntry
	json.NewDecoder(rec2.Body).Decode(&results)
	if len(results) != 3 {
		t.Errorf("expected 3 logs, got %d", len(results))
	}

	// Filter by level
	req3 := httptest.NewRequest("GET", "/api/"+projectID+"/logs?level=error", nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	var filtered []LogEntry
	json.NewDecoder(rec3.Body).Decode(&filtered)
	if len(filtered) != 1 {
		t.Errorf("expected 1 error log, got %d", len(filtered))
	}
}

func TestSearchLogs(t *testing.T) {
	store, projectID := setupLogTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc)

	router := chi.NewRouter()
	router.Post("/api/{projectID}/logs", h.IngestLogs)
	router.Get("/api/{projectID}/logs/search", h.SearchLogs)

	entries := []LogEntry{
		{Level: "info", Message: "User login successful for admin", Logger: "auth"},
		{Level: "info", Message: "Database connection established", Logger: "db"},
	}
	body, _ := json.Marshal(entries)
	req := httptest.NewRequest("POST", "/api/"+projectID+"/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest("GET", "/api/"+projectID+"/logs/search?q=admin", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("search: expected 200, got %d", rec2.Code)
	}
	var results []LogEntry
	json.NewDecoder(rec2.Body).Decode(&results)
	if len(results) == 0 {
		t.Error("expected search results for 'admin'")
	}
}
