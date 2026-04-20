package release

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for release creation and listing.
type Handler struct {
	svc *Service
}

// NewHandler creates a new release Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Create handles release creation for a project.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	var req struct {
		Version     string `json:"version"`
		Environment string `json:"environment"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	rel, err := h.svc.Create(projectID, req.Version, req.Environment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rel)
}

// List returns all releases for a project.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	rels, err := h.svc.List(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rels)
}

// Routes returns a chi.Router with the release endpoints registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{projectID}/releases", h.Create)
	r.Get("/{projectID}/releases", h.List)
	return r
}
