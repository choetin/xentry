package web

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xentry/xentry/internal/auth"
	"github.com/xentry/xentry/internal/web/i18n"
)

//go:embed all:templates
var templateFS embed.FS

// requestLang is set per-request before template execution.
// Since Go HTTP handlers run sequentially per connection, this is safe.
var requestLang string

func templateT(key string) string {
	return i18n.T(requestLang, key)
}

func templateTf(key string, args ...interface{}) string {
	return fmt.Sprintf(i18n.T(requestLang, key), args...)
}

// --- Pagination ---

const pageSize = 20

// Pagination holds pagination state for list pages.
type Pagination struct {
	CurrentPage int
	TotalPages  int
}

// pageParam extracts the page number from the query string (default 1).
func pageParam(r *http.Request) int {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if p < 1 {
		p = 1
	}
	return p
}

// offset returns the SQL OFFSET for the current page.
func (p Pagination) offset() int {
	return (p.CurrentPage - 1) * pageSize
}

// newPagination creates a Pagination from the current page and total item count.
func newPagination(page, total int) Pagination {
	tp := total / pageSize
	if total%pageSize > 0 {
		tp++
	}
	if tp < 1 {
		tp = 1
	}
	return Pagination{CurrentPage: page, TotalPages: tp}
}

// buildPageURL builds a URL with the given page number, preserving other query params.
func buildPageURL(r *http.Request, page int) string {
	q := r.URL.Query()
	q.Set("page", strconv.Itoa(page))
	return r.URL.Path + "?" + q.Encode()
}

// renderPagination generates pagination HTML. Called from handlers and passed to templates.
func (h *Handler) renderPagination(r *http.Request, p Pagination, lang string) template.HTML {
	if p.TotalPages <= 1 {
		return ""
	}
	requestLang = lang

	var b strings.Builder
	label := fmt.Sprintf(i18n.T(lang, "pagination.page_of"), p.CurrentPage, p.TotalPages)

	fmt.Fprintf(&b, `<div class="flex items-center justify-end gap-2 mt-4 mb-2">`)

	if p.TotalPages <= 7 {
		for i := 1; i <= p.TotalPages; i++ {
			if i == p.CurrentPage {
				fmt.Fprintf(&b, `<span class="px-2.5 py-1 text-sm rounded bg-blue-600 text-white font-medium">%d</span>`, i)
			} else {
				fmt.Fprintf(&b, `<a href="%s" class="px-2.5 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">%d</a>`, buildPageURL(r, i), i)
			}
		}
	} else {
		fmt.Fprintf(&b, `<a href="%s" class="px-2.5 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">1</a>`, buildPageURL(r, 1))
		if p.CurrentPage > 3 {
			fmt.Fprintf(&b, `<span class="px-1 text-gray-400">...</span>`)
		}
		start := p.CurrentPage - 1
		if start < 2 {
			start = 2
		}
		end := p.CurrentPage + 1
		if end > p.TotalPages-1 {
			end = p.TotalPages - 1
		}
		for i := start; i <= end; i++ {
			if i == p.CurrentPage {
				fmt.Fprintf(&b, `<span class="px-2.5 py-1 text-sm rounded bg-blue-600 text-white font-medium">%d</span>`, i)
			} else {
				fmt.Fprintf(&b, `<a href="%s" class="px-2.5 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">%d</a>`, buildPageURL(r, i), i)
			}
		}
		if p.CurrentPage < p.TotalPages-2 {
			fmt.Fprintf(&b, `<span class="px-1 text-gray-400">...</span>`)
		}
		fmt.Fprintf(&b, `<a href="%s" class="px-2.5 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">%d</a>`, buildPageURL(r, p.TotalPages), p.TotalPages)
	}

	if p.CurrentPage > 1 {
		fmt.Fprintf(&b, `<a href="%s" class="px-2.5 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">%s</a>`, buildPageURL(r, p.CurrentPage-1), i18n.T(lang, "pagination.previous"))
	}
	fmt.Fprintf(&b, `<span class="text-sm text-gray-500 dark:text-gray-400 ml-1">%s</span>`, label)
	if p.CurrentPage < p.TotalPages {
		fmt.Fprintf(&b, `<a href="%s" class="px-2.5 py-1 text-sm rounded border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800">%s</a>`, buildPageURL(r, p.CurrentPage+1), i18n.T(lang, "pagination.next"))
	}

	fmt.Fprintf(&b, `</div>`)
	return template.HTML(b.String())
}

// Handler serves HTML pages using Go's html/template engine.
type Handler struct {
	appTemplates  map[string]*template.Template
	authTemplates map[string]*template.Template
	db            *sql.DB
	authSvc       *auth.Service
}

// pageConfig maps a logical page name to its template file path and layout.
type pageConfig struct {
	file   string
	layout string // "app" or "auth"
}

// pageConfigs defines all pages. Auth pages use auth_layout.html,
// app pages use layout.html (with sidebar).
var pageConfigs = map[string]pageConfig{
	"login":               {"templates/auth/login.html", "auth"},
	"register":            {"templates/auth/register.html", "auth"},
	"dashboard":           {"templates/dashboard.html", "app"},
	"issues/list":         {"templates/issues/list.html", "app"},
	"issues/detail":       {"templates/issues/detail.html", "app"},
	"transactions/list":   {"templates/transactions/list.html", "app"},
	"transactions/detail": {"templates/transactions/detail.html", "app"},
	"logs/list":           {"templates/logs/list.html", "app"},
	"releases/list":       {"templates/releases/list.html", "app"},
	"orgs/list":           {"templates/orgs/list.html", "app"},
	"orgs/projects":       {"templates/orgs/projects.html", "app"},
	"projects/settings":   {"templates/projects/settings.html", "app"},
}

// pageData is the envelope passed to all app templates.
// Page-specific data is accessed via .Data.
type pageData struct {
	User          *auth.User
	ActiveNav     string
	OrgID         string
	OrgName       string
	ProjectID     string
	ProjectName   string
	Lang          string
	LangSwitchURL string
	Data          interface{}
}

// NewHandler creates a new web handler, building a separate template set
// per page (layout + page file) to avoid define-name collisions.
func NewHandler(db *sql.DB, authSvc *auth.Service) *Handler {
	funcMap := template.FuncMap{
		"safeHTML":   func(s string) template.HTML { return template.HTML(s) },
		"formatTime": func(ts int64) string { return time.Unix(ts, 0).Format("2006-01-02 15:04:05") },
		"T":          templateT,
		"Tf":         templateTf,
	}
	appTemplates := make(map[string]*template.Template)
	authTemplates := make(map[string]*template.Template)

	for name, pc := range pageConfigs {
		if pc.layout == "auth" {
			// Parse auth page with auth layout
			at, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/auth_layout.html", pc.file)
			if err != nil {
				log.Fatalf("parsing auth template %s: %v", name, err)
			}
			authTemplates[name] = at
		} else {
			// Parse app page with app layout
			at, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", pc.file)
			if err != nil {
				log.Fatalf("parsing app template %s: %v", name, err)
			}
			appTemplates[name] = at
		}
	}
	return &Handler{appTemplates: appTemplates, authTemplates: authTemplates, db: db, authSvc: authSvc}
}

// renderAuth looks up the per-page auth template set and executes "auth-base".
func (h *Handler) renderAuth(w http.ResponseWriter, name string, data map[string]interface{}) {
	if lang, ok := data["Lang"].(string); ok {
		requestLang = lang
	}
	data["Version"] = Version
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := h.authTemplates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "auth-base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderApp looks up the per-page app template set and executes "base".
// It converts pageData to a map so that T and Tf (function values) are
// callable as template functions (Go templates cannot call function-valued
// struct fields, only top-level map entries).
func (h *Handler) renderApp(w http.ResponseWriter, name string, pd pageData) {
	if pd.LangSwitchURL == "" {
		if pd.Lang == "zh" {
			pd.LangSwitchURL = "/lang/en"
		} else {
			pd.LangSwitchURL = "/lang/zh"
		}
	}
	requestLang = pd.Lang
	data := map[string]interface{}{
		"User":          pd.User,
		"ActiveNav":     pd.ActiveNav,
		"OrgID":         pd.OrgID,
		"OrgName":       pd.OrgName,
		"ProjectID":     pd.ProjectID,
		"ProjectName":   pd.ProjectName,
		"Lang":          pd.Lang,
		"LangSwitchURL": pd.LangSwitchURL,
		"Data":          pd.Data,
			"Version":       Version,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, ok := h.appTemplates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// --- Locale helpers ---

type localeKey string

func getLang(r *http.Request) string {
	lang, _ := r.Context().Value(localeKey("lang")).(string)
	if lang == "" {
		return "en"
	}
	return lang
}

// LocaleMiddleware reads the xentry_lang cookie and stores the language
// in the request context so all downstream handlers can access it.
func (h *Handler) LocaleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := "en"
		if c, err := r.Cookie("xentry_lang"); err == nil {
			for _, l := range i18n.Supported() {
				if c.Value == l {
					lang = l
					break
				}
			}
		}
		ctx := context.WithValue(r.Context(), localeKey("lang"), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetLang sets the user's language preference cookie and redirects back
// to the referring page (or / if no referer).
func (h *Handler) SetLang(w http.ResponseWriter, r *http.Request) {
	lang := chi.URLParam(r, "lang")
	valid := false
	for _, l := range i18n.Supported() {
		if l == lang {
			valid = true
			break
		}
	}
	if !valid {
		lang = "en"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "xentry_lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/"
	}
	http.Redirect(w, r, referer, http.StatusFound)
}

// --- Auth pages ---

// LoginPage serves the sign-in page.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	lang := getLang(r)
	h.renderAuth(w, "login", map[string]interface{}{
		"Error": r.URL.Query().Get("error"),
		"Lang":  lang,
	})
}

// RegisterPage serves the registration page.
func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	lang := getLang(r)
	h.renderAuth(w, "register", map[string]interface{}{
		"Error": r.URL.Query().Get("error"),
		"Lang":  lang,
	})
}

// --- App page helpers ---

// getUserFromCookie extracts the user from the JWT cookie set by CookieJWTMiddleware.
func (h *Handler) getUser(r *http.Request) *auth.User {
	userID, ok := r.Context().Value(auth.UserIDKey).(string)
	if !ok {
		return nil
	}
	user, err := h.authSvc.GetUserByID(userID)
	if err != nil {
		return nil
	}
	return user
}

// --- Dashboard ---

type dashboardData struct {
	Orgs              []dashOrg
	Projects          []dashProject
	UnresolvedIssues  int
	TotalEvents       int
	TotalTransactions int
	TotalReleases     int
	RecentIssues      []dashIssue
}

type dashOrg struct {
	ID, Name, Slug string
}

type dashProject struct {
	ID, OrgID, OrgName, Name, Slug, Platform string
}

type dashIssue struct {
	ID, ProjectID, OrgName, Title, Level, Status string
	Count                                        int
}

// Dashboard serves the main dashboard page with server-rendered data.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	page := pageParam(r)
	lang := getLang(r)

	// Organizations (scoped to current user via org_members)
	var orgs []dashOrg
	rows, err := h.db.Query(
		"SELECT o.id, o.name, o.slug FROM organizations o JOIN org_members m ON m.org_id = o.id WHERE m.user_id = ? AND o.deleted_at = 0 ORDER BY o.created_at",
		userID,
	)
	if err == nil {
		for rows.Next() {
			var o dashOrg
			rows.Scan(&o.ID, &o.Name, &o.Slug)
			orgs = append(orgs, o)
		}
		rows.Close()
	}

	// Projects (scoped to current user's orgs, paginated)
	var projectTotal int
	h.db.QueryRow("SELECT COUNT(*) FROM projects p JOIN org_members m ON m.org_id = p.org_id JOIN organizations o ON o.id = p.org_id WHERE m.user_id = ? AND p.deleted_at = 0 AND o.deleted_at = 0", userID).Scan(&projectTotal)
	projPg := newPagination(page, projectTotal)

	var projects []dashProject
	rows, err = h.db.Query(
		"SELECT p.id, p.org_id, o.name, p.name, p.slug, p.platform FROM projects p JOIN organizations o ON o.id = p.org_id JOIN org_members m ON m.org_id = p.org_id WHERE m.user_id = ? AND p.deleted_at = 0 AND o.deleted_at = 0 ORDER BY p.created_at LIMIT ? OFFSET ?",
		userID, pageSize, projPg.offset(),
	)
	if err == nil {
		for rows.Next() {
			var p dashProject
			rows.Scan(&p.ID, &p.OrgID, &p.OrgName, &p.Name, &p.Slug, &p.Platform)
			projects = append(projects, p)
		}
		rows.Close()
	}

	// Stats (scoped to current user's projects in non-deleted orgs)
	var stats dashboardData
	h.db.QueryRow("SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id JOIN organizations o ON o.id = p.org_id JOIN org_members m ON m.org_id = p.org_id WHERE i.status = 'unresolved' AND i.deleted_at = 0 AND p.deleted_at = 0 AND o.deleted_at = 0 AND m.user_id = ?", userID).Scan(&stats.UnresolvedIssues)
	h.db.QueryRow("SELECT COUNT(*) FROM events e JOIN projects p ON p.id = e.project_id JOIN organizations o ON o.id = p.org_id JOIN org_members m ON m.org_id = p.org_id WHERE e.deleted_at = 0 AND p.deleted_at = 0 AND o.deleted_at = 0 AND m.user_id = ?", userID).Scan(&stats.TotalEvents)
	h.db.QueryRow("SELECT COUNT(*) FROM transactions t JOIN projects p ON p.id = t.project_id JOIN organizations o ON o.id = p.org_id JOIN org_members m ON m.org_id = p.org_id WHERE t.deleted_at = 0 AND p.deleted_at = 0 AND o.deleted_at = 0 AND m.user_id = ?", userID).Scan(&stats.TotalTransactions)
	h.db.QueryRow("SELECT COUNT(*) FROM releases r JOIN projects p ON p.id = r.project_id JOIN organizations o ON o.id = p.org_id JOIN org_members m ON m.org_id = p.org_id WHERE r.deleted_at = 0 AND p.deleted_at = 0 AND o.deleted_at = 0 AND m.user_id = ?", userID).Scan(&stats.TotalReleases)

	// Recent issues (scoped to current user's projects)
	rows, err = h.db.Query(
		"SELECT i.id, i.project_id, o.name, i.title, i.level, i.status, i.count FROM issues i JOIN projects p ON p.id = i.project_id JOIN organizations o ON o.id = p.org_id JOIN org_members m ON m.org_id = p.org_id WHERE i.deleted_at = 0 AND p.deleted_at = 0 AND o.deleted_at = 0 AND m.user_id = ? ORDER BY i.last_seen DESC LIMIT 10",
		userID,
	)
	if err == nil {
		for rows.Next() {
			var ri dashIssue
			rows.Scan(&ri.ID, &ri.ProjectID, &ri.OrgName, &ri.Title, &ri.Level, &ri.Status, &ri.Count)
			stats.RecentIssues = append(stats.RecentIssues, ri)
		}
		rows.Close()
	}

	h.renderApp(w, "dashboard", pageData{
		User:      h.getUser(r),
		ActiveNav: "dashboard",
		Lang:      lang,
		Data: map[string]interface{}{
			"Orgs":       orgs,
			"Projects":   projects,
			"Stats":      stats,
			"Pagination": h.renderPagination(r, projPg, lang),
		},
	})
}

// --- Issues ---

// IssuesPage serves the issue list page for a project.
func (h *Handler) IssuesPage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	page := pageParam(r)
	lang := getLang(r)

	// Get total count
	var total int
	h.db.QueryRow("SELECT COUNT(*) FROM issues WHERE project_id = ? AND deleted_at = 0", projectID).Scan(&total)
	pg := newPagination(page, total)

	type issueRow struct {
		ID, ProjectID, Title, Level, Status string
		Count                               int
		FirstSeen, LastSeen                int64
	}
	rows, err := h.db.Query(
		"SELECT id, project_id, title, level, status, count, first_seen, last_seen FROM issues WHERE project_id = ? AND deleted_at = 0 ORDER BY last_seen DESC LIMIT ? OFFSET ?",
		projectID, pageSize, pg.offset(),
	)
	var issues []issueRow
	if err == nil {
		for rows.Next() {
			var i issueRow
			rows.Scan(&i.ID, &i.ProjectID, &i.Title, &i.Level, &i.Status, &i.Count, &i.FirstSeen, &i.LastSeen)
			issues = append(issues, i)
		}
		rows.Close()
	}

	h.renderApp(w, "issues/list", pageData{
		User:      h.getUser(r),
		ActiveNav: "issues",
		ProjectID: projectID,
		Lang:      lang,
		Data: map[string]interface{}{
			"Issues":     issues,
			"Pagination": h.renderPagination(r, pg, lang),
		},
	})
}

// IssueDetailPage serves the detail page for a single issue.
func (h *Handler) IssueDetailPage(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "issueID")
	projectID := chi.URLParam(r, "projectID")

	type issueDetail struct {
		ID, ProjectID, Title, Level, Status, Type string
		Count                                    int
		FirstSeen, LastSeen                      int64
	}
	var issue issueDetail
	err := h.db.QueryRow(
		"SELECT id, project_id, title, level, status, type, count, first_seen, last_seen FROM issues WHERE id = ? AND deleted_at = 0", issueID,
	).Scan(&issue.ID, &issue.ProjectID, &issue.Title, &issue.Level, &issue.Status, &issue.Type, &issue.Count, &issue.FirstSeen, &issue.LastSeen)

	if err != nil {
		http.Error(w, "issue not found", http.StatusNotFound)
		return
	}

	type eventRow struct {
		ID, Release, Environment, Platform, Message string
		Timestamp                                   int64
		Annotations                                 map[string]string
		StackwalkOutput                             string
	}
	var events []eventRow
	erows, eErr := h.db.Query(
		"SELECT id, timestamp, release, environment, platform, message, payload, stackwalk_output FROM events WHERE issue_id = ? AND deleted_at = 0 ORDER BY timestamp DESC LIMIT 20",
		issueID,
	)
	if eErr == nil {
		for erows.Next() {
			var e eventRow
			var payload []byte
			erows.Scan(&e.ID, &e.Timestamp, &e.Release, &e.Environment, &e.Platform, &e.Message, &payload, &e.StackwalkOutput)
			// Extract annotations from payload JSON
			var p struct {
				Annotations map[string]string `json:"annotations,omitempty"`
			}
			if json.Valid(payload) {
				json.Unmarshal(payload, &p)
			}
			e.Annotations = p.Annotations
			events = append(events, e)
		}
		erows.Close()
	}

	// Get stackwalk output from the latest event that has it.
	var stackwalkOutput string
	for _, e := range events {
		if e.StackwalkOutput != "" {
			stackwalkOutput = e.StackwalkOutput
			break
		}
	}

	h.renderApp(w, "issues/detail", pageData{
		User:      h.getUser(r),
		ActiveNav: "issues",
		ProjectID: projectID,
		Lang:      getLang(r),
		Data: map[string]interface{}{
			"Issue":          issue,
			"StackwalkOutput": stackwalkOutput,
			"Events":         events,
		},
	})
}

// --- Transactions ---

// TransactionsPage serves the transaction list page for a project.
func (h *Handler) TransactionsPage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	page := pageParam(r)
	lang := getLang(r)

	var total int
	h.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE project_id = ? AND deleted_at = 0", projectID).Scan(&total)
	pg := newPagination(page, total)

	type txRow struct {
		ID, ProjectID, Name, TraceID, Status string
		Op                    string
		StartTime            float64
		Duration             float64
	}
	rows, err := h.db.Query(
		"SELECT id, project_id, name, trace_id, status, op, start_time, duration FROM transactions WHERE project_id = ? AND deleted_at = 0 ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		projectID, pageSize, pg.offset(),
	)
	var txs []txRow
	if err == nil {
		for rows.Next() {
			var t txRow
			rows.Scan(&t.ID, &t.ProjectID, &t.Name, &t.TraceID, &t.Status, &t.Op, &t.StartTime, &t.Duration)
			txs = append(txs, t)
		}
		rows.Close()
	}

	h.renderApp(w, "transactions/list", pageData{
		User:      h.getUser(r),
		ActiveNav: "apm",
		ProjectID: projectID,
		Lang:      lang,
		Data: map[string]interface{}{
			"Transactions": txs,
			"Pagination":   h.renderPagination(r, pg, lang),
		},
	})
}

// TransactionDetailPage serves the detail page for a single transaction.
func (h *Handler) TransactionDetailPage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	txID := chi.URLParam(r, "txID")

	type txDetail struct {
		ID, ProjectID, Name, TraceID, Status, Op, Environment string
		StartTime, Duration                                      float64
	}
	var tx txDetail
	err := h.db.QueryRow(
		"SELECT id, project_id, name, trace_id, status, op, environment, start_time, duration FROM transactions WHERE id = ? AND deleted_at = 0", txID,
	).Scan(&tx.ID, &tx.ProjectID, &tx.Name, &tx.TraceID, &tx.Status, &tx.Op, &tx.Environment, &tx.StartTime, &tx.Duration)
	if err != nil {
		http.Error(w, "transaction not found", http.StatusNotFound)
		return
	}

	type spanRow struct {
		ID, TxID, ParentID, SpanID, Op, Description string
		Status                                       string
		StartTime, Duration                           float64
		LeftPct, WidthPct                             float64
	}
	srows, err := h.db.Query(
		"SELECT id, tx_id, parent_id, span_id, op, description, status, start_time, duration FROM spans WHERE tx_id = ? ORDER BY start_time", txID,
	)
	var spans []spanRow
	if err == nil {
		for srows.Next() {
			var s spanRow
			srows.Scan(&s.ID, &s.TxID, &s.ParentID, &s.SpanID, &s.Op, &s.Description, &s.Status, &s.StartTime, &s.Duration)
			spans = append(spans, s)
		}
		srows.Close()
	}

	maxEnd := float64(0)
	for _, s := range spans {
		end := s.StartTime + s.Duration
		if end > maxEnd {
			maxEnd = end
		}
	}
	if maxEnd > 0 {
		for i := range spans {
			spans[i].LeftPct = (spans[i].StartTime / maxEnd) * 100
			w := (spans[i].Duration / maxEnd) * 100
			if w < 1 {
				w = 1
			}
			spans[i].WidthPct = w
		}
	}

	h.renderApp(w, "transactions/detail", pageData{
		User:      h.getUser(r),
		ActiveNav: "apm",
		ProjectID: projectID,
		Lang:      getLang(r),
		Data: map[string]interface{}{
			"TX":    tx,
			"Spans": spans,
		},
	})
}

// --- Logs ---

// LogsPage serves the log browser page for a project.
func (h *Handler) LogsPage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	page := pageParam(r)
	lang := getLang(r)
	level := r.URL.Query().Get("level")
	q := r.URL.Query().Get("q")

	where := "WHERE project_id = ? AND deleted_at = 0"
	args := []interface{}{projectID}
	if level != "" {
		where += " AND level = ?"
		args = append(args, level)
	}
	if q != "" {
		where += " AND message LIKE ?"
		args = append(args, "%"+q+"%")
	}

	// Count total
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	h.db.QueryRow("SELECT COUNT(*) FROM logs "+where, countArgs...).Scan(&total)
	pg := newPagination(page, total)

	query := "SELECT id, timestamp, level, message, logger, environment, release FROM logs " + where + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, pg.offset())

	rows, err := h.db.Query(query, args...)
	type logRow struct {
		ID, Level, Message, Logger, Environment, Release string
		Timestamp                                                int64
	}
	var logs []logRow
	if err == nil {
		for rows.Next() {
			var l logRow
			rows.Scan(&l.ID, &l.Level, &l.Message, &l.Logger, &l.Environment, &l.Release, &l.Timestamp)
			logs = append(logs, l)
		}
		rows.Close()
	}

	h.renderApp(w, "logs/list", pageData{
		User:      h.getUser(r),
		ActiveNav: "logs",
		ProjectID: projectID,
		Lang:      lang,
		Data: map[string]interface{}{
			"Logs":       logs,
			"Level":      level,
			"Q":          q,
			"Pagination": h.renderPagination(r, pg, lang),
		},
	})
}

// --- Releases ---

// ReleasesPage serves the releases list page for a project.
func (h *Handler) ReleasesPage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	page := pageParam(r)
	lang := getLang(r)

	var total int
	h.db.QueryRow("SELECT COUNT(*) FROM releases WHERE project_id = ? AND deleted_at = 0", projectID).Scan(&total)
	pg := newPagination(page, total)

	type releaseRow struct {
		ID, ProjectID, Version, Environment string
		CreatedAt                            int64
	}
	rows, err := h.db.Query(
		"SELECT id, project_id, version, environment, created_at FROM releases WHERE project_id = ? AND deleted_at = 0 ORDER BY created_at DESC LIMIT ? OFFSET ?",
		projectID, pageSize, pg.offset(),
	)
	var releases []releaseRow
	if err == nil {
		for rows.Next() {
			var rel releaseRow
			rows.Scan(&rel.ID, &rel.ProjectID, &rel.Version, &rel.Environment, &rel.CreatedAt)
			releases = append(releases, rel)
		}
		rows.Close()
	}

	h.renderApp(w, "releases/list", pageData{
		User:      h.getUser(r),
		ActiveNav: "releases",
		ProjectID: projectID,
		Lang:      lang,
		Data: map[string]interface{}{
			"Releases":   releases,
			"Pagination": h.renderPagination(r, pg, lang),
		},
	})
}

// --- Organizations ---

// OrgListPage serves the organizations list page.
func (h *Handler) OrgListPage(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r)
	page := pageParam(r)
	lang := getLang(r)

	var total int
	h.db.QueryRow("SELECT COUNT(*) FROM organizations o JOIN org_members m ON m.org_id = o.id WHERE m.user_id = ? AND o.deleted_at = 0", userID).Scan(&total)
	pg := newPagination(page, total)

	type orgRow struct {
		ID, Name, Slug string
		ProjectCount   int
	}
	rows, err := h.db.Query(
		"SELECT o.id, o.name, o.slug, COUNT(p.id) FROM organizations o JOIN org_members m ON m.org_id = o.id LEFT JOIN projects p ON p.org_id = o.id AND p.deleted_at = 0 WHERE m.user_id = ? AND o.deleted_at = 0 GROUP BY o.id ORDER BY o.created_at LIMIT ? OFFSET ?",
		userID, pageSize, pg.offset(),
	)
	var orgs []orgRow
	if err == nil {
		for rows.Next() {
			var o orgRow
			rows.Scan(&o.ID, &o.Name, &o.Slug, &o.ProjectCount)
			orgs = append(orgs, o)
		}
		rows.Close()
	}

	h.renderApp(w, "orgs/list", pageData{
		User:      h.getUser(r),
		ActiveNav: "orgs",
		Lang:      lang,
		Data: map[string]interface{}{
			"Orgs":       orgs,
			"Pagination": h.renderPagination(r, pg, lang),
		},
	})
}

// OrgProjectsPage serves the projects list for an organization.
func (h *Handler) OrgProjectsPage(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	page := pageParam(r)
	lang := getLang(r)

	var orgName string
	h.db.QueryRow("SELECT name FROM organizations WHERE id = ? AND deleted_at = 0", orgID).Scan(&orgName)

	var total int
	h.db.QueryRow("SELECT COUNT(*) FROM projects WHERE org_id = ? AND deleted_at = 0", orgID).Scan(&total)
	pg := newPagination(page, total)

	type projRow struct {
		ID, Name, Slug, Platform string
	}
	rows, err := h.db.Query(
		"SELECT id, name, slug, platform FROM projects WHERE org_id = ? AND deleted_at = 0 ORDER BY created_at LIMIT ? OFFSET ?",
		orgID, pageSize, pg.offset(),
	)
	var projects []projRow
	if err == nil {
		for rows.Next() {
			var p projRow
			rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Platform)
			projects = append(projects, p)
		}
		rows.Close()
	}

	h.renderApp(w, "orgs/projects", pageData{
		User:      h.getUser(r),
		ActiveNav: "orgs",
		OrgID:     orgID,
		OrgName:   orgName,
		Lang:      lang,
		Data: map[string]interface{}{
			"Projects":   projects,
			"Pagination": h.renderPagination(r, pg, lang),
		},
	})
}

// --- Project Settings ---

// ProjectSettingsPage serves the project settings page (DSN, API tokens).
func (h *Handler) ProjectSettingsPage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")

	var projName, dsnToken string
	h.db.QueryRow("SELECT name, dsn_token FROM projects WHERE id = ? AND deleted_at = 0", projectID).Scan(&projName, &dsnToken)

	type tokenRow struct {
		ID           string
		Name         string
		CreatedAt    string
		CreatedAtRaw int64
		ProjectID    string
	}
	rows, err := h.db.Query(
		"SELECT id, name, created_at FROM api_tokens WHERE project_id = ? AND deleted_at = 0 ORDER BY created_at DESC", projectID,
	)
	var tokens []tokenRow
	if err == nil {
		for rows.Next() {
			var t tokenRow
			rows.Scan(&t.ID, &t.Name, &t.CreatedAtRaw)
			t.CreatedAt = time.Unix(t.CreatedAtRaw, 0).Format("2006-01-02 15:04:05")
			t.ProjectID = projectID
			tokens = append(tokens, t)
		}
		rows.Close()
	}

	type symbolRow struct {
		DebugID    string
		Type       string
		Release    string
		UploadedAt int64
	}
	symRows, symErr := h.db.Query(
		"SELECT debug_id, type, release, uploaded_at FROM symbol_files WHERE project_id = ? ORDER BY uploaded_at DESC", projectID,
	)
	var symbols []symbolRow
	if symErr == nil {
		for symRows.Next() {
			var s symbolRow
			symRows.Scan(&s.DebugID, &s.Type, &s.Release, &s.UploadedAt)
			symbols = append(symbols, s)
		}
		symRows.Close()
	}

	crashpadURL := fmt.Sprintf("http://%s/crashpad/%s/crash", r.Host, dsnToken)
	h.renderApp(w, "projects/settings", pageData{
		User:         h.getUser(r),
		ActiveNav:    "settings",
		ProjectID:    projectID,
		ProjectName:  projName,
		Lang:         getLang(r),
		Data: map[string]interface{}{
			"DSN":          dsnToken,
			"CrashpadURL": crashpadURL,
			"Tokens":       tokens,
			"Symbols":      symbols,
		},
	})
}

// --- Logout ---

// Logout clears the auth cookie and redirects to /login.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "xentry_auth",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

// --- Helpers ---

// isHTMX returns true if the request was sent by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
