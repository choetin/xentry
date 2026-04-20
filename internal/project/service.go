// Package project provides project management including creation, DSN tokens, and soft deletion.
package project

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"

	"github.com/xentry/xentry/pkg/util"
)

// Project represents a monitored application within an organization.
type Project struct {
	ID        string
	OrgID     string
	Name      string
	Slug      string
	Platform  string
	DSNToken  string
	CreatedAt int64
}

// Service handles project CRUD, API token management, and soft deletion.
type Service struct {
	db *sql.DB
}

// NewService creates a new project Service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create inserts a new project under the specified organization with a generated DSN token.
func (s *Service) Create(orgID, name, slug, platform string) (*Project, error) {
	id := util.UUID()
	dsnToken := util.Token()
	_, err := s.db.Exec(
		"INSERT INTO projects (id, org_id, name, slug, platform, dsn_token) VALUES (?, ?, ?, ?, ?, ?)",
		id, orgID, name, slug, platform, dsnToken,
	)
	if err != nil {
		return nil, err
	}
	return &Project{
		ID:       id,
		OrgID:    orgID,
		Name:     name,
		Slug:     slug,
		Platform: platform,
		DSNToken: dsnToken,
	}, nil
}

// GetByID returns a single non-deleted project by ID.
func (s *Service) GetByID(id string) (*Project, error) {
	var p Project
	err := s.db.QueryRow(
		"SELECT id, org_id, name, slug, platform, dsn_token, created_at FROM projects WHERE id = ? AND deleted_at = 0", id,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Platform, &p.DSNToken, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByDSNToken returns a non-deleted project by its DSN token.
func (s *Service) GetByDSNToken(token string) (*Project, error) {
	var p Project
	err := s.db.QueryRow(
		"SELECT id, org_id, name, slug, platform, dsn_token, created_at FROM projects WHERE dsn_token = ? AND deleted_at = 0", token,
	).Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Platform, &p.DSNToken, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListByOrg returns all non-deleted projects in the specified organization.
func (s *Service) ListByOrg(orgID string) ([]*Project, error) {
	rows, err := s.db.Query(
		"SELECT id, org_id, name, slug, platform, dsn_token, created_at FROM projects WHERE org_id = ? AND deleted_at = 0 ORDER BY created_at",
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Slug, &p.Platform, &p.DSNToken, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	return projects, rows.Err()
}

// CreateAPIToken generates a new API token for the project. The plaintext token
// is returned only once; a SHA-256 hash is stored in the database.
func (s *Service) CreateAPIToken(projectID, name string) (string, error) {
	plainToken := util.Token()
	hash := sha256.Sum256([]byte(plainToken))
	tokenHash := hex.EncodeToString(hash[:])

	id := util.UUID()
	_, err := s.db.Exec(
		"INSERT INTO api_tokens (id, project_id, name, token_hash) VALUES (?, ?, ?, ?)",
		id, projectID, name, tokenHash,
	)
	if err != nil {
		return "", err
	}
	return plainToken, nil
}

// DeleteAPIToken permanently removes an API token by ID.
func (s *Service) DeleteAPIToken(tokenID string) error {
	_, err := s.db.Exec("DELETE FROM api_tokens WHERE id = ?", tokenID)
	return err
}

// SoftDelete marks a project and all its child data as deleted.
func (s *Service) SoftDelete(id string) error {
	_, err := s.db.Exec("UPDATE projects SET deleted_at = unixepoch() WHERE id = ? AND deleted_at = 0", id)
	if err != nil {
		return err
	}
	s.db.Exec("UPDATE issues SET deleted_at = unixepoch() WHERE project_id = ?", id)
	s.db.Exec("UPDATE events SET deleted_at = unixepoch() WHERE project_id = ?", id)
	s.db.Exec("UPDATE transactions SET deleted_at = unixepoch() WHERE project_id = ?", id)
	s.db.Exec("UPDATE logs SET deleted_at = unixepoch() WHERE project_id = ?", id)
	s.db.Exec("UPDATE releases SET deleted_at = unixepoch() WHERE project_id = ?", id)
	s.db.Exec("UPDATE api_tokens SET deleted_at = unixepoch() WHERE project_id = ?", id)
	s.db.Exec("UPDATE symbol_files SET deleted_at = unixepoch() WHERE project_id = ?", id)
	return nil
}
