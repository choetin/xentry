package apm

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Handler provides HTTP handlers for APM trace ingestion and transaction queries.
type Handler struct {
	svc *Service
}

// NewHandler creates a new APM Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// IngestTrace handles batch ingestion of transactions and spans.
func (h *Handler) IngestTrace(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")

	var payload struct {
		Transactions []Transaction `json:"transactions"`
		Spans        []Span        `json:"spans"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	for i := range payload.Transactions {
		payload.Transactions[i].ProjectID = projectID
		err := h.svc.Ingest(projectID, &payload.Transactions[i], payload.Spans)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// ListTransactions returns recent transactions for a project.
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	txs, err := h.svc.ListTransactions(projectID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txs)
}

// GetTransaction returns a single transaction by ID.
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "id")
	tx, err := h.svc.GetTransaction(txID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tx)
}

// GetTransactionSpans returns all spans for a given transaction.
func (h *Handler) GetTransactionSpans(w http.ResponseWriter, r *http.Request) {
	txID := chi.URLParam(r, "id")
	spans, err := h.svc.GetSpans(txID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spans)
}

// GetStats returns aggregate transaction statistics for a project.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	stats, err := h.svc.GetStats(projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Routes returns a chi.Router with the APM endpoints registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{projectID}/traces", h.IngestTrace)
	r.Get("/{projectID}/transactions", h.ListTransactions)
	r.Get("/{projectID}/transactions/{id}", h.GetTransaction)
	r.Get("/{projectID}/transactions/{id}/spans", h.GetTransactionSpans)
	r.Get("/{projectID}/transactions/stats", h.GetStats)
	return r
}
