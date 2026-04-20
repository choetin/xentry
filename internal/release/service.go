// Package release provides release (version) tracking and per-release statistics.
package release

import (
	"database/sql"

	"github.com/xentry/xentry/pkg/util"
)

// Release represents a version release of a project.
type Release struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	Version     string `json:"version"`
	Environment string `json:"environment"`
	CreatedAt   int64  `json:"created_at"`
}

// Service handles release storage and retrieval.
type Service struct {
	db *sql.DB
}

// NewService creates a new release Service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create inserts a new release record for a project.
func (s *Service) Create(projectID, version, environment string) (*Release, error) {
	id := util.UUID()
	if environment == "" {
		environment = "production"
	}
	_, err := s.db.Exec(
		"INSERT INTO releases (id, project_id, version, environment) VALUES (?,?,?,?)",
		id, projectID, version, environment,
	)
	if err != nil {
		return nil, err
	}
	return &Release{ID: id, ProjectID: projectID, Version: version, Environment: environment}, nil
}

// List returns all non-deleted releases for a project, ordered by creation time descending.
func (s *Service) List(projectID string) ([]Release, error) {
	rows, err := s.db.Query("SELECT id, project_id, version, environment, created_at FROM releases WHERE project_id = ? AND deleted_at = 0 ORDER BY created_at DESC", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var releases []Release
	for rows.Next() {
		var r Release
		rows.Scan(&r.ID, &r.ProjectID, &r.Version, &r.Environment, &r.CreatedAt)
		releases = append(releases, r)
	}
	return releases, nil
}

// GetStats returns the issue and event counts for a specific release version.
func (s *Service) GetStats(projectID, version string) (map[string]interface{}, error) {
	stats := map[string]interface{}{}
	var issueCount, eventCount int
	s.db.QueryRow("SELECT COUNT(*) FROM issues WHERE project_id = ? AND deleted_at = 0 AND id IN (SELECT DISTINCT issue_id FROM events WHERE release = ? AND deleted_at = 0)", projectID, version).Scan(&issueCount)
	s.db.QueryRow("SELECT COUNT(*) FROM events WHERE project_id = ? AND deleted_at = 0 AND release = ?", projectID, version).Scan(&eventCount)
	stats["issues"] = issueCount
	stats["events"] = eventCount
	return stats, nil
}
