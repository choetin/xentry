// Package router wires all HTTP handlers into a Chi mux with the appropriate
// middleware, authentication, and route groups.
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xentry/xentry/internal/apm"
	"github.com/xentry/xentry/internal/auth"
	"github.com/xentry/xentry/internal/crash"
	"github.com/xentry/xentry/internal/db"
	"github.com/xentry/xentry/internal/log"
	"github.com/xentry/xentry/internal/org"
	"github.com/xentry/xentry/internal/project"
	"github.com/xentry/xentry/internal/release"
	"github.com/xentry/xentry/internal/symbol"
	"github.com/xentry/xentry/internal/web"
)

// Services holds all service dependencies for the HTTP router.
type Services struct {
	DB           *db.SQLite
	Auth         *auth.Service
	Org          *org.Service
	Project      *project.Service
	Crash        *crash.Service
	CrashDataDir string
	Symbol       *symbol.Service
	APM          *apm.Service
	Log          *log.Service
	Release      *release.Service
	Web          *web.Handler
}

// New creates a configured Chi router with all API and page routes wired.
func New(svc *Services) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(svc.Web.LocaleMiddleware)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Auth page routes (no cookie required)
	r.Get("/login", svc.Web.LoginPage)
	r.Get("/register", svc.Web.RegisterPage)
	r.Get("/logout", svc.Web.Logout)
	r.Get("/lang/{lang}", svc.Web.SetLang)

	// App page routes (cookie-protected)
	r.Group(func(r chi.Router) {
		r.Use(auth.CookieJWTMiddleware(svc.Auth))

		r.Get("/", svc.Web.Dashboard)
		r.Get("/orgs", svc.Web.OrgListPage)
		r.Get("/orgs/{orgID}/projects", svc.Web.OrgProjectsPage)
		r.Get("/projects/{projectID}/issues", svc.Web.IssuesPage)
		r.Get("/projects/{projectID}/issues/{issueID}", svc.Web.IssueDetailPage)
		r.Get("/projects/{projectID}/transactions", svc.Web.TransactionsPage)
		r.Get("/projects/{projectID}/transactions/{txID}", svc.Web.TransactionDetailPage)
		r.Get("/projects/{projectID}/logs", svc.Web.LogsPage)
		r.Get("/projects/{projectID}/releases", svc.Web.ReleasesPage)
		r.Get("/projects/{projectID}/settings", svc.Web.ProjectSettingsPage)
	})

	// Auth routes (public)
	authHandler := auth.NewHandler(svc.Auth)
	r.Post("/api/auth/register", authHandler.Register)
	r.Post("/api/auth/login", authHandler.Login)

	// Protected API routes
	r.Group(func(r chi.Router) {
		r.Use(auth.CookieOrJWTMiddleware(svc.Auth))
		r.Get("/api/auth/me", authHandler.Me)

		// Organizations
		orgHandler := org.NewHandler(svc.Org)
		r.Post("/api/organizations", orgHandler.Create)
		r.Get("/api/organizations", orgHandler.List)
		r.Get("/api/organizations/{id}", orgHandler.Get)
		r.Get("/api/organizations/{id}/members", orgHandler.Members)
		r.Delete("/api/organizations/{id}", orgHandler.Delete)

		// Projects
		projHandler := project.NewHandler(svc.Project)
		r.Post("/api/organizations/{orgID}/projects", projHandler.Create)
		r.Get("/api/organizations/{orgID}/projects", projHandler.List)
		r.Get("/api/projects/{id}", projHandler.Get)
		r.Delete("/api/projects/{id}", projHandler.Delete)
		r.Post("/api/projects/{id}/tokens", projHandler.CreateToken)
		r.Delete("/api/projects/{id}/tokens/{tokenID}", projHandler.DeleteToken)

		// Crash read API (supports cookie or Bearer auth for web UI)
		crashHandler := crash.NewHandler(svc.Crash, svc.CrashDataDir)
		r.Get("/api/{projectID}/issues", crashHandler.ListIssues)
		r.Get("/api/{projectID}/issues/{issueID}", crashHandler.GetIssue)

		// APM read API (supports cookie or Bearer auth for web UI)
		apmHandler := apm.NewHandler(svc.APM)
		r.Get("/api/{projectID}/transactions", apmHandler.ListTransactions)
		r.Get("/api/{projectID}/transactions/{id}", apmHandler.GetTransaction)
		r.Get("/api/{projectID}/transactions/{id}/spans", apmHandler.GetTransactionSpans)
		r.Get("/api/{projectID}/transactions/stats", apmHandler.GetStats)

		// Log read/search API (supports cookie or Bearer auth for web UI)
		logHandler := log.NewHandler(svc.Log)
		r.Get("/api/{projectID}/logs", logHandler.QueryLogs)
		r.Get("/api/{projectID}/logs/search", logHandler.SearchLogs)

		// Release read API
		releaseHandler := release.NewHandler(svc.Release)
		r.Get("/api/{projectID}/releases", releaseHandler.List)

		// Release create API
		r.Post("/api/{projectID}/releases", releaseHandler.Create)

		// APM ingest API (DSN token auth handled by handler)
		r.Post("/api/{projectID}/traces", apmHandler.IngestTrace)

		// Symbol upload API (authenticated)
		symbolHandler := symbol.NewHandler(svc.Symbol)
		r.Post("/api/projects/{projectID}/symbols", symbolHandler.Upload)
	})

	// Crash ingest API (DSN token auth handled by handler)
	crashHandler2 := crash.NewHandler(svc.Crash, svc.CrashDataDir)
	r.Post("/api/{projectID}/events", crashHandler2.IngestEvent)
	r.Post("/api/{projectID}/crash", crashHandler2.IngestMinidump)

	// Crashpad-compatible crash upload (DSN token in URL path)
	r.Post("/crashpad/{dsnToken}/crash", crashHandler2.IngestCrashpad)

	// Log ingest API
	logHandler2 := log.NewHandler(svc.Log)
	r.Post("/api/{projectID}/logs", logHandler2.IngestLogs)

	return r
}
