package symbol

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xentry/xentry/internal/auth"
)

// Handler provides HTTP handlers for symbol file upload.
type Handler struct {
	svc *Service
}

// NewHandler creates a new symbol Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Upload handles symbol file upload for a project. The file should be a
// Breakpad .sym file (optionally gzip-compressed).
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := auth.GetUserID(r)
	if userID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	symType := r.FormValue("type")
	release := r.FormValue("release")
	if symType == "" {
		symType = "breakpad"
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	sf, err := h.svc.Upload(projectID, userID, symType, release, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sf)
}

// Routes returns a chi.Router with the symbol upload endpoint registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{projectID}/symbols", h.Upload)
	return r
}
