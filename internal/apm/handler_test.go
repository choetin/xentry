package apm

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

func setupAPMTest(t *testing.T) (*db.SQLite, string) {
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

func TestIngestTrace(t *testing.T) {
	store, projectID := setupAPMTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc)

	router := chi.NewRouter()
	router.Post("/api/{projectID}/traces", h.IngestTrace)
	router.Get("/api/{projectID}/transactions", h.ListTransactions)
	router.Get("/api/{projectID}/transactions/stats", h.GetStats)

	payload := map[string]interface{}{
		"transactions": []map[string]interface{}{
			{
				"name":     "GET /api/users",
				"trace_id": "trace-123",
				"span_id":  "span-456",
				"op":       "http.server",
				"status":   "ok",
				"duration": 150.5,
			},
		},
		"spans": []map[string]interface{}{
			{"op": "db.query", "description": "SELECT * FROM users", "duration": 50.0, "status": "ok"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/"+projectID+"/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	// List transactions
	req2 := httptest.NewRequest("GET", "/api/"+projectID+"/transactions", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec2.Code)
	}
	var txs []map[string]interface{}
	json.NewDecoder(rec2.Body).Decode(&txs)
	if len(txs) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(txs))
	}

	// Get stats
	req3 := httptest.NewRequest("GET", "/api/"+projectID+"/transactions/stats", nil)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("stats: expected 200, got %d", rec3.Code)
	}
}
