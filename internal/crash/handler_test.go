package crash

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/xentry/xentry/internal/db"
	"github.com/xentry/xentry/internal/org"
	"github.com/xentry/xentry/internal/project"
)

func setupCrashTest(t *testing.T) (*db.SQLite, string, string) {
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
	return store, o.ID, p.DSNToken
}

func TestIngestEvent(t *testing.T) {
	store, _, dsn := setupCrashTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc, t.TempDir())

	router := chi.NewRouter()
	router.Post("/api/{projectID}/events", h.IngestEvent)

	event := map[string]interface{}{
		"message":  "Null pointer dereference",
		"level":    "fatal",
		"platform": "windows",
		"release":  "1.0.0",
		"threads": []map[string]interface{}{
			{
				"name":    "main",
				"crashed": true,
				"frames": []map[string]interface{}{
					{"function": "main", "file": "main.cpp", "line": 10},
					{"function": "crash", "file": "crash.cpp", "line": 42},
				},
			},
		},
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest("POST", "/api/proj-123/events", bytes.NewReader(body))
	req.Header.Set("X-Xentry-DSN", dsn)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected event ID in response")
	}
}

func TestIngestEvent_GroupsByFingerprint(t *testing.T) {
	store, _, dsn := setupCrashTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc, t.TempDir())

	router := chi.NewRouter()
	router.Post("/api/{projectID}/events", h.IngestEvent)

	// Send two events with the same stack trace
	event := map[string]interface{}{
		"message": "Same crash",
		"level":   "fatal",
		"threads": []map[string]interface{}{
			{
				"name":    "main",
				"crashed": true,
				"frames": []map[string]interface{}{
					{"function": "main", "file": "main.cpp", "line": 10},
					{"function": "crash", "file": "crash.cpp", "line": 42},
				},
			},
		},
	}

	for i := 0; i < 2; i++ {
		body, _ := json.Marshal(event)
		req := httptest.NewRequest("POST", "/api/proj-123/events", bytes.NewReader(body))
		req.Header.Set("X-Xentry-DSN", dsn)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("event %d: expected 202, got %d", i, rec.Code)
		}
	}

	// Check that only 1 issue was created with count=2
	var count int
	err := store.DB().QueryRow("SELECT COUNT(*) FROM issues WHERE project_id = (SELECT id FROM projects LIMIT 1)").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 issue, got %d", count)
	}

	var issueCount int
	store.DB().QueryRow("SELECT count FROM issues LIMIT 1").Scan(&issueCount)
	if issueCount != 2 {
		t.Errorf("expected issue count=2, got %d", issueCount)
	}
}

func TestListIssues(t *testing.T) {
	store, _, dsn := setupCrashTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc, t.TempDir())

	router := chi.NewRouter()
	router.Post("/api/{projectID}/events", h.IngestEvent)
	router.Get("/api/{projectID}/issues", h.ListIssues)

	// Ingest one event first
	event := map[string]interface{}{
		"message": "Test crash",
		"level":   "error",
		"threads": []map[string]interface{}{
			{"name": "main", "crashed": true, "frames": []map[string]interface{}{
				{"function": "foo", "file": "bar.cpp", "line": 1},
			}},
		},
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest("POST", "/api/proj-123/events", bytes.NewReader(body))
	req.Header.Set("X-Xentry-DSN", dsn)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), req)

	// Now list issues — also needs DSN header since handler resolves it
	req2 := httptest.NewRequest("GET", "/api/proj-123/issues", nil)
	req2.Header.Set("X-Xentry-DSN", dsn)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req2)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var issues []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&issues)
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}

func buildCrashpadMultipartBody(t *testing.T, prod, ver, plat, reason string, includeMinidump bool) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if includeMinidump {
		part, _ := writer.CreateFormFile("upload_file_minidump", "minidump.dmp")
		part.Write([]byte("MDMP" + strings.Repeat("\x00", 100)))
	}
	if prod != "" {
		_ = writer.WriteField("prod", prod)
	}
	if ver != "" {
		_ = writer.WriteField("ver", ver)
	}
	if plat != "" {
		_ = writer.WriteField("plat", plat)
	}
	if reason != "" {
		_ = writer.WriteField("crash_reason", reason)
	}
	contentType := writer.FormDataContentType()
	writer.Close()
	return &buf, contentType
}

func TestIngestCrashpad(t *testing.T) {
	store, _, dsn := setupCrashTest(t)
	svc := NewService(store.DB())
	h := NewHandler(svc, t.TempDir())

	router := chi.NewRouter()
	router.Post("/crashpad/{dsnToken}/crash", h.IngestCrashpad)

	t.Run("successful crash upload", func(t *testing.T) {
		body, ct := buildCrashpadMultipartBody(t, "TestApp", "1.0.0", "windows", "EXCEPTION_ACCESS_VIOLATION", true)

		req := httptest.NewRequest("POST", "/crashpad/"+dsn+"/crash", body)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		respBody := rec.Body.String()
		if !strings.HasPrefix(respBody, "CrashID=bp-") {
			t.Errorf("expected body to start with 'CrashID=bp-', got: %s", respBody)
		}

		// Verify issue was created in DB
		var title string
		err := store.DB().QueryRow("SELECT title FROM issues LIMIT 1").Scan(&title)
		if err != nil {
			t.Fatalf("expected issue to be created, got error: %v", err)
		}
		if !strings.Contains(title, "TestApp") {
			t.Errorf("expected issue title to contain 'TestApp', got: %s", title)
		}
	})

	t.Run("empty DSN token returns 400", func(t *testing.T) {
		body, ct := buildCrashpadMultipartBody(t, "TestApp", "1.0.0", "windows", "EXCEPTION_ACCESS_VIOLATION", true)

		req := httptest.NewRequest("POST", "/crashpad//crash", body)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid DSN token returns 400", func(t *testing.T) {
		body, ct := buildCrashpadMultipartBody(t, "TestApp", "1.0.0", "windows", "EXCEPTION_ACCESS_VIOLATION", true)

		req := httptest.NewRequest("POST", "/crashpad/invalid-dsn-token/crash", body)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing minidump file returns 400", func(t *testing.T) {
		body, ct := buildCrashpadMultipartBody(t, "TestApp", "1.0.0", "windows", "EXCEPTION_ACCESS_VIOLATION", false)

		req := httptest.NewRequest("POST", "/crashpad/"+dsn+"/crash", body)
		req.Header.Set("Content-Type", ct)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
	})
}
