package log

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for log ingestion, querying, and full-text search.
type Handler struct {
	svc *Service
}

// NewHandler creates a new log Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// IngestLogs handles batch log ingestion for a project.
func (h *Handler) IngestLogs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	var entries []LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.svc.Ingest(projectID, entries); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "count": strconv.Itoa(len(entries))})
}

// QueryLogs returns log entries for a project, optionally filtered by log level.
func (h *Handler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	level := r.URL.Query().Get("level")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, err := h.svc.Query(projectID, level, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// SearchLogs performs a full-text search over log messages.
func (h *Handler) SearchLogs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, err := h.svc.Search(projectID, query, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// Routes returns a chi.Router with the log endpoints registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{projectID}/logs", h.IngestLogs)
	r.Get("/{projectID}/logs", h.QueryLogs)
	r.Get("/{projectID}/logs/search", h.SearchLogs)
	return r
}
