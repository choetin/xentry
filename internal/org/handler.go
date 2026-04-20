package org

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/xentry/xentry/internal/auth"
)

// Handler provides HTTP handlers for organization CRUD and membership endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new org Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create handles organization creation. Supports both JSON and form-encoded bodies.
// On HTMX requests it redirects to /orgs; otherwise it returns JSON.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var name, slug string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var req struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if isHTMXRequest(r) {
				htmxError(w, "Invalid request body", http.StatusBadRequest)
			} else {
				http.Error(w, "invalid request body", http.StatusBadRequest)
			}
			return
		}
		name, slug = req.Name, req.Slug
	} else {
		r.ParseForm()
		name = r.FormValue("name")
		slug = r.FormValue("slug")
	}

	if name == "" || slug == "" {
		if isHTMXRequest(r) {
			htmxError(w, "Name and slug are required.", http.StatusBadRequest)
		} else {
			http.Error(w, "name and slug required", http.StatusBadRequest)
		}
		return
	}
	org, err := h.svc.Create(name, slug, auth.GetUserID(r))
	if err != nil {
		if isHTMXRequest(r) {
			htmxError(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusConflict)
		}
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/orgs")
		w.WriteHeader(http.StatusCreated)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(org)
}

// List returns all organizations the authenticated user belongs to.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.svc.ListByUser(auth.GetUserID(r))
	if err != nil {
		http.Error(w, "failed to list organizations", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orgs)
}

// Get returns a single organization by ID.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	org, err := h.svc.GetByID(id)
	if err != nil {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

// Members returns all members of the specified organization.
func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	members, err := h.svc.GetMembers(id)
	if err != nil {
		http.Error(w, "failed to list members", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// Delete soft-deletes an organization and all its projects.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.SoftDelete(id); err != nil {
		if isHTMXRequest(r) {
			htmxError(w, "Failed to delete organization.", http.StatusInternalServerError)
		} else {
			http.Error(w, "failed to delete organization", http.StatusInternalServerError)
		}
		return
	}
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/orgs")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Routes returns a chi.Router with the organization endpoints registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/members", h.Members)
	r.Delete("/{id}", h.Delete)
	return r
}

// isHTMXRequest returns true if the request was sent by HTMX.
func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// htmxError writes a styled HTML error that HTMX can swap into a target div.
func htmxError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm mt-2">%s</div>`, msg)
}
