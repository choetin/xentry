package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/xentry/xentry/internal/db"
)

// Deterministic UUIDs for idempotent seeding.
//
// Two accounts with separate data:
//
//	alice@xentry.dev / demo123  ->  Acme Corp (Web Frontend, Mobile App)
//	bob@xentry.dev   / demo123  ->  Beta Labs  (API Service)
var (
	// Users
	aliceID = "a0000000-0000-0000-0000-000000000001"
	bobID   = "a0000000-0000-0000-0000-000000000002"

	// Organizations
	acmeOrgID = "a0000000-0000-0000-0000-000000000010"
	betaOrgID = "a0000000-0000-0000-0000-000000000011"

	// Projects (Acme Corp)
	webProjID    = "a0000000-0000-0000-0000-000000000020"
	mobileProjID = "a0000000-0000-0000-0000-000000000021"
	// Projects (Beta Labs)
	apiProjID = "a0000000-0000-0000-0000-000000000022"

	// API Tokens
	token1ID = "a0000000-0000-0000-0000-000000000030"
	token2ID = "a0000000-0000-0000-0000-000000000031"
	token3ID = "a0000000-0000-0000-0000-000000000032"

	// Issues (Acme Corp – Web Frontend)
	issue1ID = "a0000000-0000-0000-0000-000000000040"
	issue2ID = "a0000000-0000-0000-0000-000000000041"
	// Issues (Acme Corp – Mobile App)
	issue3ID = "a0000000-0000-0000-0000-000000000042"
	issue4ID = "a0000000-0000-0000-0000-000000000043"
	// Issues (Beta Labs – API Service)
	issue5ID = "a0000000-0000-0000-0000-000000000044"

	// Events (for issue 1)
	event1ID = "a0000000-0000-0000-0000-000000000050"
	event2ID = "a0000000-0000-0000-0000-000000000051"

	// Threads (one per event)
	thread1ID = "a0000000-0000-0000-0000-000000000060"
	thread2ID = "a0000000-0000-0000-0000-000000000061"

	// Frames
	frameIDs = []string{
		"a0000000-0000-0000-0000-000000000070",
		"a0000000-0000-0000-0000-000000000071",
		"a0000000-0000-0000-0000-000000000072",
		"a0000000-0000-0000-0000-000000000073",
		"a0000000-0000-0000-0000-000000000074",
		"a0000000-0000-0000-0000-000000000075",
		"a0000000-0000-0000-0000-000000000076",
		"a0000000-0000-0000-0000-000000000077",
	}

	// Transactions (Acme Corp)
	tx1ID = "a0000000-0000-0000-0000-000000000080"
	tx2ID = "a0000000-0000-0000-0000-000000000081"
	// Transactions (Beta Labs)
	tx3ID = "a0000000-0000-0000-0000-000000000082"
	// Transactions (Acme Corp)
	tx4ID = "a0000000-0000-0000-0000-000000000083"

	// Spans
	spanIDs = []string{
		"a0000000-0000-0000-0000-000000000090",
		"a0000000-0000-0000-0000-000000000091",
		"a0000000-0000-0000-0000-000000000092",
		"a0000000-0000-0000-0000-000000000093",
		"a0000000-0000-0000-0000-000000000094",
		"a0000000-0000-0000-0000-000000000095",
		"a0000000-0000-0000-0000-000000000096",
		"a0000000-0000-0000-0000-000000000097",
	}

	// Releases (Acme Corp)
	release1ID = "a0000000-0000-0000-0000-0000000000a0"
	release2ID = "a0000000-0000-0000-0000-0000000000a1"
	release3ID = "a0000000-0000-0000-0000-0000000000a2"
	// Releases (Beta Labs)
	release4ID = "a0000000-0000-0000-0000-0000000000a3"

	// Symbol files
	symbol1ID = "a0000000-0000-0000-0000-0000000000b0"
	symbol2ID = "a0000000-0000-0000-0000-0000000000b1"
)

func main() {
	dbPath := os.Getenv("XENTRY_DB")
	if dbPath == "" {
		dbPath = "./xentry.db"
	}

	store, err := db.NewSQLite(dbPath)
	if err != nil {
		log.Fatalf("init db: %v", err)
	}
	defer store.Close()

	fmt.Println("Seeding demo data into", dbPath)
	seed(store.DB())
	fmt.Println("Done.")
}

func seed(db *sql.DB) {
	hash, err := bcrypt.GenerateFromPassword([]byte("demo123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt: %v", err)
	}
	tokenHash1 := sha256.Sum256([]byte("seed-token-web-frontend"))
	tokenHash2 := sha256.Sum256([]byte("seed-token-mobile-app"))
	tokenHash3 := sha256.Sum256([]byte("seed-token-api-service"))

	// ── Users ────────────────────────────────────────────────
	//   alice@xentry.dev  ->  Acme Corp
	//   bob@xentry.dev    ->  Beta Labs
	exec(db, `INSERT OR IGNORE INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)`,
		aliceID, "alice@xentry.dev", string(hash), "Alice Chen")
	exec(db, `INSERT OR IGNORE INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)`,
		bobID, "bob@xentry.dev", string(hash), "Bob Martinez")

	// ── Organizations ────────────────────────────────────────
	exec(db, `INSERT OR IGNORE INTO organizations (id, name, slug) VALUES (?, ?, ?)`,
		acmeOrgID, "Acme Corp", "acme-corp")
	exec(db, `INSERT OR IGNORE INTO organizations (id, name, slug) VALUES (?, ?, ?)`,
		betaOrgID, "Beta Labs", "beta-labs")

	// ── Org Members ──────────────────────────────────────────
	exec(db, `INSERT OR IGNORE INTO org_members (org_id, user_id, role) VALUES (?, ?, ?)`,
		acmeOrgID, aliceID, "owner")
	exec(db, `INSERT OR IGNORE INTO org_members (org_id, user_id, role) VALUES (?, ?, ?)`,
		betaOrgID, bobID, "owner")

	// ── Projects ─────────────────────────────────────────────
	// Acme Corp
	exec(db, `INSERT OR IGNORE INTO projects (id, org_id, name, slug, platform, dsn_token) VALUES (?, ?, ?, ?, ?, ?)`,
		webProjID, acmeOrgID, "Web Frontend", "web-frontend", "other", "dsn-web-frontend-seed")
	exec(db, `INSERT OR IGNORE INTO projects (id, org_id, name, slug, platform, dsn_token) VALUES (?, ?, ?, ?, ?, ?)`,
		mobileProjID, acmeOrgID, "Mobile App", "mobile-app", "android", "dsn-mobile-app-seed")
	// Beta Labs
	exec(db, `INSERT OR IGNORE INTO projects (id, org_id, name, slug, platform, dsn_token) VALUES (?, ?, ?, ?, ?, ?)`,
		apiProjID, betaOrgID, "API Service", "api-service", "linux", "dsn-api-service-seed")

	// ── API Tokens ───────────────────────────────────────────
	exec(db, `INSERT OR IGNORE INTO api_tokens (id, project_id, name, token_hash, scopes) VALUES (?, ?, ?, ?, ?)`,
		token1ID, webProjID, "Web Frontend Token", fmt.Sprintf("%x", tokenHash1), "event:write")
	exec(db, `INSERT OR IGNORE INTO api_tokens (id, project_id, name, token_hash, scopes) VALUES (?, ?, ?, ?, ?)`,
		token2ID, mobileProjID, "Mobile App Token", fmt.Sprintf("%x", tokenHash2), "event:write")
	exec(db, `INSERT OR IGNORE INTO api_tokens (id, project_id, name, token_hash, scopes) VALUES (?, ?, ?, ?, ?)`,
		token3ID, apiProjID, "API Service Token", fmt.Sprintf("%x", tokenHash3), "event:write")

	// ── Issues ───────────────────────────────────────────────
	// Acme Corp – Web Frontend
	exec(db, `INSERT OR IGNORE INTO issues (id, project_id, fingerprint, title, level, status, type, last_seen, first_seen, count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue1ID, webProjID, "fp-null-pointer-web", "NullPointerException in UserController", "error", "unresolved", "crash",
		1744000000, 1743900000, 24)
	exec(db, `INSERT OR IGNORE INTO issues (id, project_id, fingerprint, title, level, status, type, last_seen, first_seen, count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue2ID, webProjID, "fp-out-of-memory-web", "OutOfMemoryError during image upload", "fatal", "unresolved", "crash",
		1744010000, 1743950000, 5)
	// Acme Corp – Mobile App
	exec(db, `INSERT OR IGNORE INTO issues (id, project_id, fingerprint, title, level, status, type, last_seen, first_seen, count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue3ID, mobileProjID, "fp-deprecated-api-mobile", "Deprecated API call: setUsesCleartextTraffic", "warning", "unresolved", "error",
		1744020000, 1743800000, 152)
	exec(db, `INSERT OR IGNORE INTO issues (id, project_id, fingerprint, title, level, status, type, last_seen, first_seen, count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue4ID, mobileProjID, "fp-network-timeout-mobile", "Network timeout on API call /users/profile", "error", "resolved", "error",
		1743900000, 1743700000, 38)
	// Beta Labs – API Service
	exec(db, `INSERT OR IGNORE INTO issues (id, project_id, fingerprint, title, level, status, type, last_seen, first_seen, count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue5ID, apiProjID, "fp-slow-query-api", "Slow query: SELECT * FROM orders (3200ms)", "warning", "muted", "error",
		1743980000, 1743850000, 67)

	// ── Events (2 for issue 1, Web Frontend) ─────────────────
	stackwalkOutput1 := ` 0  xentry-web.exe!UserController::getUser [user_controller.cpp : 87 + 0x12]
     rsp = 0x0000001c88bab018    rip = 0x00007ff6dec738be
    Found by: given as instruction pointer in context
 1  xentry-web.exe!Router::dispatch [router.cpp : 142 + 0xb]
     rsp = 0x0000001c88bab020    rip = 0x00007ff6dec5231f
    Found by: call frame info
 2  xentry-web.exe!Application::run [application.cpp : 25 + 0x5]
     rsp = 0x0000001c88bab040    rip = 0x00007ff6dec4a100
    Found by: call frame info
 3  xentry-web.exe!main [main.cpp : 10 + 0x3]
     rsp = 0x0000001c88bab060    rip = 0x00007ff6dec49000
    Found by: call frame info`

	stackwalkOutput2 := ` 0  xentry-web.exe!UserController::getUser [user_controller.cpp : 87 + 0x12]
     rsp = 0x0000001c99cbb018    rip = 0x00007ff6ded738be
    Found by: given as instruction pointer in context
 1  xentry-web.exe!Router::dispatch [router.cpp : 142 + 0xb]
     rsp = 0x0000001c99cbb020    rip = 0x00007ff6ded5231f
    Found by: call frame info
 2  xentry-web.exe!Application::run [application.cpp : 25 + 0x5]
     rsp = 0x0000001c99cbb040    rip = 0x00007ff6ded4a100
    Found by: call frame info
 3  xentry-web.exe!main [main.cpp : 10 + 0x3]
     rsp = 0x0000001c99cbb060    rip = 0x00007ff6ded49000
    Found by: call frame info`

	exec(db, `INSERT OR IGNORE INTO events (id, project_id, issue_id, release, environment, platform, timestamp, message, payload, stackwalk_output) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event1ID, webProjID, issue1ID, "v1.1.0", "production", "web", 1744000000,
		"NullPointerException: Cannot invoke \"String.length()\" because \"userName\" is null",
		`{"url":"/api/users/123","method":"GET","user_agent":"Mozilla/5.0"}`,
		stackwalkOutput1)
	exec(db, `INSERT OR IGNORE INTO events (id, project_id, issue_id, release, environment, platform, timestamp, message, payload, stackwalk_output) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event2ID, webProjID, issue1ID, "v1.0.0", "production", "web", 1743950000,
		"NullPointerException: Cannot invoke \"String.length()\" because \"userName\" is null",
		`{"url":"/api/users/456","method":"GET","user_agent":"Mozilla/5.0"}`,
		stackwalkOutput2)

	// ── Threads (one per event) ──────────────────────────────
	exec(db, `INSERT OR IGNORE INTO threads (id, event_id, name, crashed, frame_count) VALUES (?, ?, ?, ?, ?)`,
		thread1ID, event1ID, "main", 1, 4)
	exec(db, `INSERT OR IGNORE INTO threads (id, event_id, name, crashed, frame_count) VALUES (?, ?, ?, ?, ?)`,
		thread2ID, event2ID, "main", 1, 4)

	// ── Frames ───────────────────────────────────────────────
	type frame struct {
		frameNo  int
		function string
		file     string
		line     int
		addr     string
		module   string
	}
	frames1 := []frame{
		{0, "java.lang.NullPointerException.<init>", "NullPointerException.java", 0, "0x7ff8a0010020", "java.base"},
		{1, "com.xentry.web.UserController.getUser", "UserController.java", 87, "0x7ff8a0015000", "xentry-web"},
		{2, "com.xentry.web.Router.dispatch", "Router.java", 142, "0x7ff8a0018000", "xentry-web"},
		{3, "com.xentry.web.Application.main", "Application.java", 25, "0x7ff8a001a000", "xentry-web"},
	}
	for i, f := range frames1 {
		exec(db, `INSERT OR IGNORE INTO frames (id, thread_id, frame_no, function, file, line, addr, module) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			frameIDs[i], thread1ID, f.frameNo, f.function, f.file, f.line, f.addr, f.module)
	}
	frames2 := []frame{
		{0, "java.lang.NullPointerException.<init>", "NullPointerException.java", 0, "0x7ff8a0020020", "java.base"},
		{1, "com.xentry.web.UserController.getUser", "UserController.java", 87, "0x7ff8a0025000", "xentry-web"},
		{2, "com.xentry.web.Router.dispatch", "Router.java", 142, "0x7ff8a0028000", "xentry-web"},
		{3, "com.xentry.web.Application.main", "Application.java", 25, "0x7ff8a002a000", "xentry-web"},
	}
	for i, f := range frames2 {
		exec(db, `INSERT OR IGNORE INTO frames (id, thread_id, frame_no, function, file, line, addr, module) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			frameIDs[4+i], thread2ID, f.frameNo, f.function, f.file, f.line, f.addr, f.module)
	}

	// ── Transactions ─────────────────────────────────────────
	// Acme Corp – Web Frontend
	exec(db, `INSERT OR IGNORE INTO transactions (id, project_id, name, trace_id, span_id, parent_id, op, status, environment, release, start_time, duration, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx1ID, webProjID, "GET /api/users", "trace-001", "span-root-001", "", "http.server", "ok", "production", "v1.1.0", 1744000000.0, 245.5, 1744000002)
	// Acme Corp – Mobile App
	exec(db, `INSERT OR IGNORE INTO transactions (id, project_id, name, trace_id, span_id, parent_id, op, status, environment, release, start_time, duration, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx2ID, mobileProjID, "POST /api/auth/login", "trace-002", "span-root-002", "", "http.server", "ok", "production", "v2.0.0-beta", 1744001000.0, 520.3, 1744001005)
	// Beta Labs – API Service
	exec(db, `INSERT OR IGNORE INTO transactions (id, project_id, name, trace_id, span_id, parent_id, op, status, environment, release, start_time, duration, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx3ID, apiProjID, "GET /api/orders", "trace-003", "span-root-003", "", "http.server", "error", "production", "v1.0.3", 1744002000.0, 3200.0, 1744002003)
	// Acme Corp – Web Frontend
	exec(db, `INSERT OR IGNORE INTO transactions (id, project_id, name, trace_id, span_id, parent_id, op, status, environment, release, start_time, duration, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tx4ID, webProjID, "POST /api/uploads", "trace-004", "span-root-004", "", "http.server", "deadline_exceeded", "production", "v1.1.0", 1744003000.0, 5000.0, 1744003005)

	// ── Spans ────────────────────────────────────────────────
	// TX 1 spans (Web Frontend – alice)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[0], tx1ID, "span-root-001", "span-db-001", "db.query", "SELECT * FROM users WHERE id=?", "ok", 1744000000.05, 45.2, `{"db.system":"postgresql","db.statement":"SELECT"}`)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[1], tx1ID, "span-root-001", "span-cache-001", "cache.get", "GET user:123", "ok", 1744000000.0, 3.1, `{"cache.hit":true}`)
	// TX 2 spans (Mobile App – alice)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[2], tx2ID, "span-root-002", "span-db-002", "db.query", "SELECT * FROM users WHERE email=?", "ok", 1744001000.1, 85.4, `{"db.system":"postgresql"}`)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[3], tx2ID, "span-root-002", "span-hash-002", "function", "bcrypt.CompareHashAndPassword", "ok", 1744001000.2, 120.6, `{}`)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[4], tx2ID, "span-root-002", "span-jwt-002", "function", "jwt.GenerateToken", "ok", 1744001000.35, 2.1, `{}`)
	// TX 3 spans (API Service – bob)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[5], tx3ID, "span-root-003", "span-db-003", "db.query", "SELECT * FROM orders JOIN customers ON...", "error", 1744002000.1, 3100.0, `{"db.system":"postgresql","error":true}`)
	// TX 4 spans (Web Frontend – alice)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[6], tx4ID, "span-root-004", "span-s3-004", "http.client", "PUT s3://uploads/image.png", "ok", 1744003000.1, 4500.0, `{"http.url":"https://s3.amazonaws.com/xentry-uploads/image.png"}`)
	exec(db, `INSERT OR IGNORE INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanIDs[7], tx4ID, "span-root-004", "span-db-004", "db.query", "INSERT INTO uploads (path, user_id) VALUES (?, ?)", "ok", 1744003000.05, 15.3, `{"db.system":"postgresql"}`)

	// ── Logs ─────────────────────────────────────────────────
	type logEntry struct {
		id         string
		projectID  string
		timestamp  int64
		level      string
		message    string
		logger     string
		traceID    string
		spanID     string
		release    string
		attributes string
	}
	logs := []logEntry{
		// Alice – Web Frontend (5 logs)
		{frameIDs[0], webProjID, 1744000000, "info", "Server started on :8080", "http.server", "", "", "v1.1.0", `{"port":8080}`},
		{frameIDs[1], webProjID, 1744000050, "debug", "Incoming request: GET /api/users", "http.server", "trace-001", "span-root-001", "v1.1.0", `{"method":"GET","path":"/api/users"}`},
		{frameIDs[2], webProjID, 1744000100, "error", "Failed to connect to Redis: connection refused", "cache", "", "", "v1.1.0", `{"error":"ECONNREFUSED"}`},
		{frameIDs[3], webProjID, 1744000150, "warn", "Request took longer than expected: 3200ms", "http.server", "trace-003", "span-root-003", "v1.1.0", `{"duration_ms":3200}`},
		{frameIDs[4], webProjID, 1744000200, "fatal", "Out of memory: heap allocation failed (512MB)", "runtime", "", "", "v1.1.0", `{"heap_used":"512MB","heap_max":"512MB"}`},

		// Alice – Mobile App (5 logs)
		{frameIDs[5], mobileProjID, 1744001000, "info", "Application started", "app", "", "", "v2.0.0-beta", `{}`},
		{frameIDs[6], mobileProjID, 1744001050, "debug", "Loading user profile from cache", "profile", "", "", "v2.0.0-beta", `{"cache_key":"user:123"}`},
		{frameIDs[7], mobileProjID, 1744001100, "info", "User authenticated successfully", "auth", "trace-002", "span-root-002", "v2.0.0-beta", `{"user_id":"u-123"}`},
		{"a0000000-0000-0000-0000-000000000078", mobileProjID, 1744001150, "warn", "Battery level low: 15%", "monitor", "", "", "v2.0.0-beta", `{"battery_level":15}`},
		{"a0000000-0000-0000-0000-000000000079", mobileProjID, 1744001200, "error", "Push notification registration failed", "notifications", "", "", "v2.0.0-beta", `{"error":"TOKEN_EXPIRED"}`},

		// Bob – API Service (5 logs)
		{"a0000000-0000-0000-0000-00000000007a", apiProjID, 1744002000, "info", "Health check passed", "health", "", "", "v1.0.3", `{}`},
		{"a0000000-0000-0000-0000-00000000007b", apiProjID, 1744002050, "debug", "Processing order request", "orders", "trace-003", "span-root-003", "v1.0.3", `{"order_id":"ord-456"}`},
		{"a0000000-0000-0000-0000-00000000007c", apiProjID, 1744002100, "error", "Database connection pool exhausted", "db", "", "", "v1.0.3", `{"pool_size":20,"active":20,"waiting":5}`},
		{"a0000000-0000-0000-0000-00000000007d", apiProjID, 1744002150, "warn", "Rate limit approaching for client 192.168.1.100", "middleware", "", "", "v1.0.3", `{"client_ip":"192.168.1.100","requests":95,"limit":100}`},
		{"a0000000-0000-0000-0000-00000000007e", apiProjID, 1744002200, "fatal", "Unhandled exception in worker thread", "worker", "", "", "v1.0.3", `{"thread":"worker-3","error":"segmentation fault"}`},
	}
	for _, l := range logs {
		exec(db, `INSERT OR IGNORE INTO logs (id, project_id, timestamp, level, message, logger, trace_id, span_id, environment, release, attributes) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			l.id, l.projectID, l.timestamp, l.level, l.message, l.logger, l.traceID, l.spanID, "production", l.release, l.attributes)
	}

	// ── Releases ─────────────────────────────────────────────
	// Acme Corp
	exec(db, `INSERT OR IGNORE INTO releases (id, project_id, version, environment, created_at) VALUES (?, ?, ?, ?, ?)`,
		release1ID, webProjID, "v1.0.0", "production", 1743700000)
	exec(db, `INSERT OR IGNORE INTO releases (id, project_id, version, environment, created_at) VALUES (?, ?, ?, ?, ?)`,
		release2ID, webProjID, "v1.1.0", "production", 1743900000)
	exec(db, `INSERT OR IGNORE INTO releases (id, project_id, version, environment, created_at) VALUES (?, ?, ?, ?, ?)`,
		release3ID, mobileProjID, "v2.0.0-beta", "production", 1743950000)
	// Beta Labs
	exec(db, `INSERT OR IGNORE INTO releases (id, project_id, version, environment, created_at) VALUES (?, ?, ?, ?, ?)`,
		release4ID, apiProjID, "v1.0.3", "production", 1743850000)

	// ── Symbol Files ─────────────────────────────────────────
	// Acme Corp – Web Frontend
	exec(db, `INSERT OR IGNORE INTO symbol_files (id, project_id, release, debug_id, type, filepath, size, uploaded_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		symbol1ID, webProjID, "v1.1.0", "EE587629D26A404D9274FFB21ED2AB76a", "breakpad", "data/seed-user/symbols/xentry-web.pdb/EE587629D26A404D9274FFB21ED2AB76a/xentry-web.sym", 204800, 1743900000)
	// Acme Corp – Mobile App
	exec(db, `INSERT OR IGNORE INTO symbol_files (id, project_id, release, debug_id, type, filepath, size, uploaded_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		symbol2ID, mobileProjID, "v2.0.0-beta", "AA11223344556677889900AABBCCDDEEf", "breakpad", "data/seed-user/symbols/libnative.so/AA11223344556677889900AABBCCDDEEf/libnative.sym", 512000, 1743950000)

	// ── Summary ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  alice@xentry.dev / demo123  (Acme Corp)")
	fmt.Println("    Web Frontend:  2 issues, 2 events, 2 tx, 5 logs, 2 releases, 1 symbol")
	fmt.Println("    Mobile App:    2 issues, 1 tx, 5 logs, 1 release, 1 symbol")
	fmt.Println()
	fmt.Println("  bob@xentry.dev   / demo123  (Beta Labs)")
	fmt.Println("    API Service:   1 issue, 1 tx, 5 logs, 1 release")
	fmt.Println()
	fmt.Println("  Total: 2 users, 2 orgs, 3 projects, 3 tokens, 5 issues, 2 events,")
	fmt.Println("         2 threads, 8 frames, 4 tx, 8 spans, 15 logs, 4 releases, 2 symbols")
}

func exec(db *sql.DB, query string, args ...any) {
	_, err := db.Exec(query, args...)
	if err != nil {
		log.Fatalf("query failed: %v\n  sql: %s", err, query)
	}
}
