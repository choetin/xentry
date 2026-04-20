package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

// UserIDKey is the context key under which the authenticated user ID is stored.
const UserIDKey contextKey = "userID"

// JWTMiddleware returns middleware that validates an "Authorization: Bearer <token>"
// header and stores the user ID in the request context.
func JWTMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "missing or invalid token", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			userID, err := svc.ValidateToken(tokenStr)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CookieJWTMiddleware returns middleware that reads a JWT from the "xentry_auth" cookie.
// If the cookie is missing or invalid, it redirects to /login.
func CookieJWTMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("xentry_auth")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			userID, err := svc.ValidateToken(cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the authenticated user ID from the request context.
// Returns an empty string if the user is not authenticated.
func GetUserID(r *http.Request) string {
	userID, _ := r.Context().Value(UserIDKey).(string)
	return userID
}

// CookieOrJWTMiddleware authenticates via cookie first, then falls back to
// the Authorization: Bearer header. On failure it returns 401 (no redirect).
func CookieOrJWTMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try cookie first (browser HTMX requests)
			if cookie, err := r.Cookie("xentry_auth"); err == nil {
				if userID, err := svc.ValidateToken(cookie.Value); err == nil {
					ctx := context.WithValue(r.Context(), UserIDKey, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			// Fall back to Bearer token (programmatic API calls)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				if userID, err := svc.ValidateToken(tokenStr); err == nil {
					ctx := context.WithValue(r.Context(), UserIDKey, userID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
	}
}

// DSNTokenMiddleware returns middleware that validates the X-Xentry-DSN header
// or "dsn" query parameter against the expected project DSN token.
func DSNTokenMiddleware(validToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dsn := r.Header.Get("X-Xentry-DSN")
			if dsn == "" {
				dsn = r.URL.Query().Get("dsn")
			}
			if dsn != validToken {
				http.Error(w, "invalid dsn token", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
