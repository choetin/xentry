package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	svc := NewService("test-secret-key")
	hash, err := svc.HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "secret123" {
		t.Error("password should not be stored in plain text")
	}
	if !svc.CheckPassword("secret123", hash) {
		t.Error("CheckPassword should return true for correct password")
	}
	if svc.CheckPassword("wrong", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateToken(t *testing.T) {
	svc := NewService("test-secret-key")
	token, err := svc.GenerateToken("user-123")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
	userID, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("expected user-123, got %s", userID)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	svc := NewService("test-secret-key")
	_, err := svc.ValidateToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}
