package crash

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/xentry/xentry/internal/symbol"
	"github.com/xentry/xentry/pkg/util"
)

// IngestEvent represents a crash event submitted by a client SDK.
type IngestEvent struct {
	ProjectID       string             `json:"-"`
	Release         string             `json:"release"`
	Environment     string             `json:"environment"`
	Platform        string             `json:"platform"`
	Message         string             `json:"message"`
	Level           string             `json:"level"`
	Threads         []IngestThread     `json:"threads"`
	Annotations     map[string]string  `json:"annotations,omitempty"`
	StackwalkOutput string             `json:"-"` // raw minidump-stackwalk text for the crashed thread
}

// IngestThread represents a single thread within a crash event.
type IngestThread struct {
	Name    string        `json:"name"`
	Crashed bool          `json:"crashed"`
	Frames  []IngestFrame `json:"frames"`
}

// IngestFrame represents a single stack frame within a thread.
type IngestFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Addr     string `json:"addr"`
	Module   string `json:"module"`
}

// Service handles crash event ingestion, issue grouping, and symbolication.
type Service struct {
	db           *sql.DB
	symbolicator *symbol.Symbolicator // can be nil
}

// NewService creates a new crash Service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// SetSymbolicator attaches an optional symbolicator for async post-ingest symbolication.
func (s *Service) SetSymbolicator(sym *symbol.Symbolicator) {
	s.symbolicator = sym
}

// ResolveUserID returns the user ID of the org owner for a given project.
func (s *Service) ResolveUserID(projectID string) (string, error) {
	var userID string
	err := s.db.QueryRow(
		`SELECT m.user_id FROM org_members m
			 JOIN projects p ON p.org_id = m.org_id
			 WHERE p.id = ? AND m.role = 'owner' AND p.deleted_at = 0`,
		projectID,
	).Scan(&userID)
	return userID, err
}

// ResolveDSN looks up a project ID by its DSN token.
func (s *Service) ResolveDSN(dsnToken string) (string, error) {
	var projectID string
	err := s.db.QueryRow(
		"SELECT id FROM projects WHERE dsn_token = ? AND deleted_at = 0", dsnToken,
	).Scan(&projectID)
	if err != nil {
		return "", err
	}
	return projectID, nil
}

// Ingest processes a crash event: computes a fingerprint, groups it into an issue
// (creating or updating), stores the event and its threads/frames, and triggers
// background symbolication if a symbolicator is configured. Returns the new event ID.
func (s *Service) Ingest(projectID string, event *IngestEvent) (string, error) {
	eventID := util.UUID()
	timestamp := time.Now().Unix()

	// Extract frames for fingerprinting from the crashed thread
	var allFrames []Frame
	for _, th := range event.Threads {
		if th.Crashed {
			for _, f := range th.Frames {
				allFrames = append(allFrames, Frame{
					Function: f.Function,
					File:     f.File,
					Line:     f.Line,
					Addr:     f.Addr,
					Module:   f.Module,
				})
			}
			break
		}
	}
	// Fallback: use the first thread if none is marked crashed
	if len(allFrames) == 0 && len(event.Threads) > 0 {
		for _, f := range event.Threads[0].Frames {
			allFrames = append(allFrames, Frame{
				Function: f.Function,
				File:     f.File,
				Line:     f.Line,
				Addr:     f.Addr,
				Module:   f.Module,
			})
		}
	}

	fp := Fingerprint(allFrames)
	title := event.Message
	if title == "" {
		title = TitleFromStack(allFrames)
	}
	level := event.Level
	if level == "" {
		level = "error"
	}

	// Find or create issue
	issueID, err := s.findOrCreateIssue(projectID, fp, title, level)
	if err != nil {
		return "", err
	}

	// Store event
	payload, _ := json.Marshal(event)
	env := event.Environment
	if env == "" {
		env = "production"
	}
	_, err = s.db.Exec(
		"INSERT INTO events (id, project_id, issue_id, release, environment, platform, timestamp, message, payload, stackwalk_output) VALUES (?,?,?,?,?,?,?,?,?,?)",
		eventID, projectID, issueID, event.Release, env, event.Platform, timestamp, event.Message, string(payload), event.StackwalkOutput,
	)
	if err != nil {
		return "", err
	}

	// Store threads and frames
	for _, th := range event.Threads {
		threadID := util.UUID()
		crashed := 0
		if th.Crashed {
			crashed = 1
		}
		s.db.Exec("INSERT INTO threads (id, event_id, name, crashed, frame_count) VALUES (?,?,?,?,?)",
			threadID, eventID, th.Name, crashed, len(th.Frames))
		for i, f := range th.Frames {
			s.db.Exec("INSERT INTO frames (id, thread_id, frame_no, function, file, line, addr, module) VALUES (?,?,?,?,?,?,?,?)",
				util.UUID(), threadID, i, f.Function, f.File, f.Line, f.Addr, f.Module)
		}
	}

	// Trigger async symbolication
	if s.symbolicator != nil {
		go s.symbolicateEvent(eventID, projectID)
	}

	return eventID, nil
}

// symbolicateEvent attempts to symbolicate unsymbolicated frames of an event.
func (s *Service) symbolicateEvent(eventID, projectID string) {
	// Get unsymbolicated frames
	rows, err := s.db.Query(`
			SELECT f.id, f.addr, f.module
			FROM frames f
			JOIN threads t ON t.id = f.thread_id
			JOIN events e ON e.id = t.event_id
			WHERE e.id = ? AND f.symbolicated = 0
	`, eventID)
	if err != nil {
		return
	}
	defer rows.Close()

	type frameRef struct {
		ID     string
		Addr   string
		Module string
	}

	var refs []frameRef
	for rows.Next() {
		var fr frameRef
		rows.Scan(&fr.ID, &fr.Addr, &fr.Module)
		refs = append(refs, fr)
	}

	// For each frame, try to find a debug_id from the modules in the event payload
	// and symbolicate
	for _, fr := range refs {
		// Try to resolve via module name — this is simplified
		// In a full implementation, debug_id comes from the minidump module list
		// For now, we just attempt symbolication if debug_id can be derived
		// This is a placeholder that the full minidump parser will populate
		_ = fr.Module
	}
}

// findOrCreateIssue looks up an existing issue by fingerprint or creates a new one.
// If the issue already exists, it increments the event count and updates last_seen.
func (s *Service) findOrCreateIssue(projectID, fingerprint, title, level string) (string, error) {
	var issueID string
	err := s.db.QueryRow(
		"SELECT id FROM issues WHERE project_id = ? AND fingerprint = ? AND deleted_at = 0",
		projectID, fingerprint,
	).Scan(&issueID)
	if err == nil {
		// Issue already exists — increment count and update last_seen
		s.db.Exec("UPDATE issues SET last_seen = unixepoch(), count = count + 1 WHERE id = ?", issueID)
		return issueID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Create new issue
	issueID = util.UUID()
	_, err = s.db.Exec(
		"INSERT INTO issues (id, project_id, fingerprint, title, level, status, first_seen, last_seen, count) VALUES (?,?,?,?,?,'unresolved',unixepoch(),unixepoch(),1)",
		issueID, projectID, fingerprint, title, level,
	)
	return issueID, err
}
