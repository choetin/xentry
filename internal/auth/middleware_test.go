package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJWTMiddleware_ValidToken(t *testing.T) {
	svc := NewService("test-secret")
	token, _ := svc.GenerateToken("user-123")

	mw := JWTMiddleware(svc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(UserIDKey).(string)
		if userID != "user-123" {
			t.Errorf("expected user-123, got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestJWTMiddleware_MissingToken(t *testing.T) {
	svc := NewService("test-secret")
	mw := JWTMiddleware(svc)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestDSNTokenMiddleware_Valid(t *testing.T) {
	mw := DSNTokenMiddleware("valid-dsn-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Xentry-DSN", "valid-dsn-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDSNTokenMiddleware_Invalid(t *testing.T) {
	mw := DSNTokenMiddleware("valid-dsn-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Xentry-DSN", "wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestGetUserID(t *testing.T) {
	svc := NewService("test-secret")
	token, _ := svc.GenerateToken("user-abc")
	mw := JWTMiddleware(svc)

	var got string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetUserID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got != "user-abc" {
		t.Errorf("expected user-abc, got %s", got)
	}
}
