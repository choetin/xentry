// Package apm provides application performance monitoring — transaction and span ingestion,
// querying, and aggregation.
package apm

import (
	"database/sql"
	"encoding/json"

	"github.com/xentry/xentry/pkg/util"
)

// Transaction represents a top-level traced request (e.g., an HTTP request).
type Transaction struct {
	ID          string  `json:"id"`
	ProjectID   string  `json:"project_id"`
	Name        string  `json:"name"`
	TraceID     string  `json:"trace_id"`
	SpanID      string  `json:"span_id"`
	ParentID    string  `json:"parent_id"`
	Op          string  `json:"op"`
	Status      string  `json:"status"`
	Environment string  `json:"environment"`
	Release     string  `json:"release"`
	StartTime   float64 `json:"start_time"`
	Duration    float64 `json:"duration"`
	Timestamp   int64   `json:"timestamp"`
}

// Span represents a unit of work within a transaction trace.
type Span struct {
	ID          string            `json:"id"`
	TxID        string            `json:"tx_id"`
	ParentID    string            `json:"parent_id"`
	SpanID      string            `json:"span_id"`
	Op          string            `json:"op"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	StartTime   float64           `json:"start_time"`
	Duration    float64           `json:"duration"`
	Tags        map[string]string `json:"tags"`
}

// Service handles transaction and span storage and retrieval.
type Service struct {
	db *sql.DB
}

// NewService creates a new APM Service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Ingest stores a transaction and its associated spans.
func (s *Service) Ingest(projectID string, tx *Transaction, spans []Span) error {
	if tx.ID == "" {
		tx.ID = util.UUID()
	}
	tx.ProjectID = projectID

	env := tx.Environment
	if env == "" {
		env = "production"
	}

	_, err := s.db.Exec(
		"INSERT INTO transactions (id, project_id, name, trace_id, span_id, parent_id, op, status, environment, release, start_time, duration, timestamp) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		tx.ID, tx.ProjectID, tx.Name, tx.TraceID, tx.SpanID, tx.ParentID, tx.Op, tx.Status, env, tx.Release, tx.StartTime, tx.Duration, tx.Timestamp,
	)
	if err != nil {
		return err
	}

	for _, sp := range spans {
		if sp.TxID == "" {
			sp.TxID = tx.ID
		}
		if sp.ID == "" {
			sp.ID = util.UUID()
		}
		tagsJSON, _ := json.Marshal(sp.Tags)
		_, err := s.db.Exec(
			"INSERT INTO spans (id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags) VALUES (?,?,?,?,?,?,?,?,?,?)",
			sp.ID, sp.TxID, sp.ParentID, sp.SpanID, sp.Op, sp.Description, sp.Status, sp.StartTime, sp.Duration, string(tagsJSON),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// ListTransactions returns recent transactions for a project, paginated by limit/offset.
func (s *Service) ListTransactions(projectID string, limit, offset int) ([]Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(
		"SELECT id, project_id, name, trace_id, span_id, op, status, environment, release, start_time, duration, timestamp FROM transactions WHERE project_id = ? AND deleted_at = 0 ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		projectID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []Transaction
	for rows.Next() {
		var tx Transaction
		rows.Scan(&tx.ID, &tx.ProjectID, &tx.Name, &tx.TraceID, &tx.SpanID, &tx.Op, &tx.Status, &tx.Environment, &tx.Release, &tx.StartTime, &tx.Duration, &tx.Timestamp)
		txs = append(txs, tx)
	}
	return txs, nil
}

// GetTransaction returns a single transaction by ID.
func (s *Service) GetTransaction(id string) (*Transaction, error) {
	var tx Transaction
	err := s.db.QueryRow(
		"SELECT id, project_id, name, trace_id, span_id, op, status, environment, release, start_time, duration, timestamp FROM transactions WHERE id = ? AND deleted_at = 0", id,
	).Scan(&tx.ID, &tx.ProjectID, &tx.Name, &tx.TraceID, &tx.SpanID, &tx.Op, &tx.Status, &tx.Environment, &tx.Release, &tx.StartTime, &tx.Duration, &tx.Timestamp)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

// GetSpans returns all spans belonging to a transaction, ordered by start time.
func (s *Service) GetSpans(txID string) ([]Span, error) {
	rows, err := s.db.Query(
		"SELECT id, tx_id, parent_id, span_id, op, description, status, start_time, duration, tags FROM spans WHERE tx_id = ? ORDER BY start_time",
		txID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var spans []Span
	for rows.Next() {
		var sp Span
		var tagsJSON string
		rows.Scan(&sp.ID, &sp.TxID, &sp.ParentID, &sp.SpanID, &sp.Op, &sp.Description, &sp.Status, &sp.StartTime, &sp.Duration, &tagsJSON)
		json.Unmarshal([]byte(tagsJSON), &sp.Tags)
		spans = append(spans, sp)
	}
	return spans, nil
}

// GetStats returns aggregate statistics (total count, average duration, error count)
// for all transactions in a project.
func (s *Service) GetStats(projectID string) (map[string]interface{}, error) {
	stats := map[string]interface{}{}
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE project_id = ? AND deleted_at = 0", projectID).Scan(&count)
	stats["total_transactions"] = count

	var avgDuration float64
	s.db.QueryRow("SELECT AVG(duration) FROM transactions WHERE project_id = ? AND deleted_at = 0", projectID).Scan(&avgDuration)
	stats["avg_duration_ms"] = avgDuration

	var errorCount int
	s.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE project_id = ? AND deleted_at = 0 AND status != 'ok'", projectID).Scan(&errorCount)
	stats["error_count"] = errorCount
	return stats, nil
}
