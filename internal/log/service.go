// Package log provides structured log ingestion, querying, and full-text search.
package log

import (
	"database/sql"
	"encoding/json"

	"github.com/xentry/xentry/pkg/util"
)

// LogEntry represents a single structured log line.
type LogEntry struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"project_id"`
	Timestamp   int64             `json:"timestamp"`
	Level       string            `json:"level"`
	Message     string            `json:"message"`
	Logger      string            `json:"logger"`
	TraceID     string            `json:"trace_id"`
	SpanID      string            `json:"span_id"`
	Environment string            `json:"environment"`
	Release     string            `json:"release"`
	Attributes  map[string]string `json:"attributes"`
}

// Service handles log storage and retrieval with FTS support.
type Service struct {
	db *sql.DB
}

// NewService creates a new log Service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Ingest inserts a batch of log entries into the database in a single transaction.
func (s *Service) Ingest(projectID string, entries []LogEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := range entries {
		e := &entries[i]
		if e.ID == "" {
			e.ID = util.UUID()
		}
		e.ProjectID = projectID
		if e.Environment == "" {
			e.Environment = "production"
		}
		attrs, _ := json.Marshal(e.Attributes)

		_, err := tx.Exec(
			"INSERT INTO logs (id, project_id, timestamp, level, message, logger, trace_id, span_id, environment, release, attributes) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
			e.ID, e.ProjectID, e.Timestamp, e.Level, e.Message, e.Logger, e.TraceID, e.SpanID, e.Environment, e.Release, string(attrs),
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Query returns log entries for a project, optionally filtered by log level.
func (s *Service) Query(projectID, level string, limit, offset int) ([]LogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := "SELECT id, project_id, timestamp, level, message, logger, trace_id, span_id, environment, release, attributes FROM logs WHERE project_id = ? AND deleted_at = 0"
	args := []interface{}{projectID}
	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}
	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanLogs(rows)
}

// Search performs a full-text search over log messages using the FTS index.
// Falls back to a normal query if the search string is empty.
func (s *Service) Search(projectID, query string, limit, offset int) ([]LogEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if query == "" {
		// Fallback to normal query
		return s.Query(projectID, "", limit, offset)
	}
	sqlQuery := `SELECT l.id, l.project_id, l.timestamp, l.level, l.message, l.logger, l.trace_id, l.span_id, l.environment, l.release, l.attributes
			FROM logs l JOIN logs_fts f ON l.rowid = f.rowid
			WHERE l.project_id = ? AND l.deleted_at = 0 AND logs_fts MATCH ?`
	sqlQuery += " ORDER BY l.timestamp DESC LIMIT ? OFFSET ?"
	rows, err := s.db.Query(sqlQuery, projectID, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanLogs(rows)
}

// scanLogs scans database rows into LogEntry slices.
func (s *Service) scanLogs(rows *sql.Rows) ([]LogEntry, error) {
	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		var attrs string
		rows.Scan(&e.ID, &e.ProjectID, &e.Timestamp, &e.Level, &e.Message, &e.Logger, &e.TraceID, &e.SpanID, &e.Environment, &e.Release, &attrs)
		json.Unmarshal([]byte(attrs), &e.Attributes)
		entries = append(entries, e)
	}
	return entries, nil
}
