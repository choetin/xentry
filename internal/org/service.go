// Package org provides organization management including creation, membership, and soft deletion.
package org

import (
	"database/sql"

	"github.com/xentry/xentry/pkg/util"
)

// Organization represents a named group that owns projects.
type Organization struct {
	ID        string
	Name      string
	Slug      string
	CreatedAt int64
}

// Member represents a user's membership within an organization.
type Member struct {
	OrgID  string
	UserID string
	Role   string
}

// Service handles organization CRUD and membership operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new org Service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Create inserts a new organization and adds the given owner as a member.
func (s *Service) Create(name, slug, ownerID string) (*Organization, error) {
	id := util.UUID()
	_, err := s.db.Exec(
		"INSERT INTO organizations (id, name, slug) VALUES (?, ?, ?)",
		id, name, slug,
	)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(
		"INSERT INTO org_members (org_id, user_id, role) VALUES (?, ?, ?)",
		id, ownerID, "owner",
	)
	if err != nil {
		return nil, err
	}
	return &Organization{ID: id, Name: name, Slug: slug}, nil
}

// GetByID returns a single non-deleted organization by ID.
func (s *Service) GetByID(id string) (*Organization, error) {
	var o Organization
	err := s.db.QueryRow(
		"SELECT id, name, slug, created_at FROM organizations WHERE id = ? AND deleted_at = 0", id,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// ListByUser returns all non-deleted organizations the given user belongs to.
func (s *Service) ListByUser(userID string) ([]*Organization, error) {
	rows, err := s.db.Query(
		"SELECT o.id, o.name, o.slug, o.created_at FROM organizations o JOIN org_members m ON m.org_id = o.id WHERE m.user_id = ? AND o.deleted_at = 0 ORDER BY o.created_at",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, &o)
	}
	return orgs, rows.Err()
}

// List returns all non-deleted organizations.
func (s *Service) List() ([]*Organization, error) {
	rows, err := s.db.Query(
		"SELECT id, name, slug, created_at FROM organizations WHERE deleted_at = 0 ORDER BY created_at",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []*Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, &o)
	}
	return orgs, rows.Err()
}

// SoftDelete marks an organization and all its projects as deleted.
func (s *Service) SoftDelete(id string) error {
	_, err := s.db.Exec("UPDATE organizations SET deleted_at = unixepoch() WHERE id = ? AND deleted_at = 0", id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE projects SET deleted_at = unixepoch() WHERE org_id = ? AND deleted_at = 0", id)
	return err
}

// AddMember inserts a new membership record for the given user in the organization.
func (s *Service) AddMember(orgID, userID, role string) error {
	_, err := s.db.Exec(
		"INSERT INTO org_members (org_id, user_id, role) VALUES (?, ?, ?)",
		orgID, userID, role,
	)
	return err
}

// GetMembers returns all members of the specified organization.
func (s *Service) GetMembers(orgID string) ([]*Member, error) {
	rows, err := s.db.Query(
		"SELECT org_id, user_id, role FROM org_members WHERE org_id = ?",
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.OrgID, &m.UserID, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	return members, rows.Err()
}
