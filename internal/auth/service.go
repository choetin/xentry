// Package auth provides user authentication, registration, and JWT token management.
package auth

import (
	"database/sql"
	"errors"

	"github.com/xentry/xentry/pkg/util"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrUserExists is returned when a registration attempt uses an already-taken email.
	ErrUserExists   = errors.New("user already exists")
	// ErrInvalidLogin is returned when the provided email or password is incorrect.
	ErrInvalidLogin = errors.New("invalid email or password")
)

// Service handles user authentication and token operations.
type Service struct {
	db        *sql.DB
	jwtSecret string
}

// NewService creates a new auth Service with the given HMAC secret for JWT signing.
func NewService(jwtSecret string) *Service {
	return &Service{jwtSecret: jwtSecret}
}

// SetDB injects the database connection. Must be called before any DB-dependent methods.
func (s *Service) SetDB(db *sql.DB) { s.db = db }

// HashPassword returns the bcrypt hash of the given plaintext password.
func (s *Service) HashPassword(password string) (string, error) {
	return hashPassword(password)
}

// CheckPassword reports whether the plaintext password matches the bcrypt hash.
func (s *Service) CheckPassword(password, hash string) bool {
	return checkPassword(password, hash)
}

// CreateUser registers a new user with the given email, password, and display name.
// It returns the new user's ID or an error (ErrUserExists on duplicate email).
func (s *Service) CreateUser(email, password, name string) (string, error) {
	if s.db == nil {
		return "", errors.New("database not initialized")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	id := util.UUID()
	_, err = s.db.Exec("INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)",
		id, email, hash, name)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Authenticate verifies the email/password combination and returns the user ID
// and a signed JWT on success. Returns ErrInvalidLogin on failure.
func (s *Service) Authenticate(email, password string) (string, string, error) {
	if s.db == nil {
		return "", "", errors.New("database not initialized")
	}
	var id, passwordHash string
	err := s.db.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", email).Scan(&id, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrInvalidLogin
		}
		return "", "", err
	}
	if !checkPassword(password, passwordHash) {
		return "", "", ErrInvalidLogin
	}
	token, err := s.GenerateToken(id)
	if err != nil {
		return "", "", err
	}
	return id, token, nil
}

// GetUserByID returns the user with the given ID, or an error if not found.
func (s *Service) GetUserByID(id string) (*User, error) {
	var u User
	err := s.db.QueryRow("SELECT id, email, name, created_at FROM users WHERE id = ?", id).Scan(
		&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// User represents a registered user account.
type User struct {
	ID        string
	Email     string
	Name      string
	CreatedAt int64
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
