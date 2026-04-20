package project

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var name, slug, platform string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var req struct {
			Name     string `json:"name"`
			Slug     string `json:"slug"`
			Platform string `json:"platform"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if isHTMXRequest(r) {
				htmxError(w, "Invalid request body", http.StatusBadRequest)
			} else {
				http.Error(w, "invalid request body", http.StatusBadRequest)
			}
			return
		}
		name, slug, platform = req.Name, req.Slug, req.Platform
	} else {
		r.ParseForm()
		name = r.FormValue("name")
		slug = r.FormValue("slug")
		platform = r.FormValue("platform")
	}

	if name == "" || slug == "" {
		if isHTMXRequest(r) {
			htmxError(w, "Name and slug are required.", http.StatusBadRequest)
		} else {
			http.Error(w, "name and slug required", http.StatusBadRequest)
		}
		return
	}
	if platform == "" {
		platform = "other"
	}
	orgID := chi.URLParam(r, "orgID")
	p, err := h.svc.Create(orgID, name, slug, platform)
	if err != nil {
		if isHTMXRequest(r) {
			htmxError(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusConflict)
		}
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/orgs/"+orgID+"/projects")
		w.WriteHeader(http.StatusCreated)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	projects, err := h.svc.ListByOrg(orgID)
	if err != nil {
		http.Error(w, "failed to list projects", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetByID(id)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var name string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if isHTMXRequest(r) {
				htmxError(w, "Invalid request body", http.StatusBadRequest)
			} else {
				http.Error(w, "invalid request body", http.StatusBadRequest)
			}
			return
		}
		name = req.Name
	} else {
		r.ParseForm()
		name = r.FormValue("name")
	}

	if name == "" {
		if isHTMXRequest(r) {
			htmxError(w, "Token name is required.", http.StatusBadRequest)
		} else {
			http.Error(w, "token name required", http.StatusBadRequest)
		}
		return
	}
	token, err := h.svc.CreateAPIToken(id, name)
	if err != nil {
		if isHTMXRequest(r) {
			htmxError(w, err.Error(), http.StatusInternalServerError)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`<div class="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded text-sm mt-2">Token created. Copy it now — it won't be shown again: <code class="font-mono bg-green-100 px-1 rounded">` + token + `</code></div>`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *Handler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	tokenID := chi.URLParam(r, "tokenID")
	if err := h.svc.DeleteAPIToken(tokenID); err != nil {
		if isHTMXRequest(r) {
			htmxError(w, "Failed to delete token.", http.StatusInternalServerError)
		} else {
			http.Error(w, "failed to delete token", http.StatusInternalServerError)
		}
		return
	}
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", r.Header.Get("Referer"))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.SoftDelete(id); err != nil {
		if isHTMXRequest(r) {
			htmxError(w, "Failed to delete project.", http.StatusInternalServerError)
		} else {
			http.Error(w, "failed to delete project", http.StatusInternalServerError)
		}
		return
	}
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", r.Header.Get("Referer"))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{orgID}", h.Create)
	r.Get("/{orgID}", h.List)
	r.Get("/{orgID}/{id}", h.Get)
	r.Post("/{orgID}/{id}/tokens", h.CreateToken)
	r.Delete("/{orgID}/{id}/tokens/{tokenID}", h.DeleteToken)
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
