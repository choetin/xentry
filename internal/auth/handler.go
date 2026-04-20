package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for authentication endpoints (register, login, me).
type Handler struct {
	svc *Service
}

// NewHandler creates a new auth Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register handles user registration requests. It supports both JSON and
// URL-encoded form bodies. On HTMX requests it sets an auth cookie and
// redirects; otherwise it returns JSON.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	email, password, name := h.parseCredentials(r)
	if email == "" || password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	id, err := h.svc.CreateUser(email, password, name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	token, _ := h.svc.GenerateToken(id)
	if isHTMX(r) {
		setAuthCookie(w, token)
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusCreated)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "token": token})
}

// Login handles authentication requests. On success it returns a JWT token
// (JSON) or sets an auth cookie and redirects (HTMX).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	email, password, _ := h.parseCredentials(r)
	if email == "" || password == "" {
		http.Error(w, "email and password required", http.StatusBadRequest)
		return
	}
	id, token, err := h.svc.Authenticate(email, password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if isHTMX(r) {
		setAuthCookie(w, token)
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "token": token})
}

// Me returns the currently authenticated user's profile as JSON.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := h.svc.GetUserByID(userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Routes returns a chi.Router with the auth endpoints registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	return r
}

// parseCredentials extracts email, password, and optional name from the
// request. It supports both JSON and URL-encoded form bodies.
func (h *Handler) parseCredentials(r *http.Request) (email, password, name string) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		return req.Email, req.Password, req.Name
	}
	r.ParseForm()
	return r.FormValue("email"), r.FormValue("password"), r.FormValue("name")
}

// isHTMX returns true if the request was sent by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// setAuthCookie writes the JWT to an HttpOnly cookie.
func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "xentry_auth",
		Value:    token,
		Path:     "/",
		MaxAge:   int(24 * time.Hour.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
