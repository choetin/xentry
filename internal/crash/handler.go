package crash

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// gzipReadCloser wraps a gzip.Reader and an underlying io.Closer so both
// are released on Close.
type gzipReadCloser struct {
	r *gzip.Reader
	c io.Closer
}

func (g *gzipReadCloser) Read(p []byte) (int, error) { return g.r.Read(p) }
func (g *gzipReadCloser) Close() error {
	g.r.Close()
	return g.c.Close()
}

const dsnHeader = "X-Xentry-DSN"

// saveMinidump persists a minidump file to {dataDir}/{userID}/minidumps/{eventID}.dmp.
func saveMinidump(dataDir, userID, eventID string, src io.Reader) {
	dir := filepath.Join(dataDir, userID, "minidumps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("crashpad: failed to create minidump dir: %v\n", err)
		return
	}
	dst, err := os.Create(filepath.Join(dir, eventID+".dmp"))
	if err != nil {
		fmt.Printf("crashpad: failed to create minidump file: %v\n", err)
		return
	}
	defer dst.Close()
	io.Copy(dst, src)
}

// Handler provides HTTP handlers for crash event ingestion and issue queries.
type Handler struct {
	svc     *Service
	dataDir string
}

// NewHandler creates a new crash Handler.
func NewHandler(svc *Service, dataDir string) *Handler {
	return &Handler{svc: svc, dataDir: dataDir}
}

// resolveProjectID maps the DSN token from the request header to a real project ID.
func (h *Handler) resolveProjectID(w http.ResponseWriter, r *http.Request) (string, bool) {
	dsnToken := r.Header.Get(dsnHeader)
	if dsnToken == "" {
		http.Error(w, "missing DSN token", http.StatusUnauthorized)
		return "", false
	}
	projectID, err := h.svc.ResolveDSN(dsnToken)
	if err != nil {
		http.Error(w, "invalid DSN token", http.StatusUnauthorized)
		return "", false
	}
	return projectID, true
}

// IngestEvent handles JSON crash event ingestion via the API.
func (h *Handler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.resolveProjectID(w, r)
	if !ok {
		return
	}

	var event IngestEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	id, err := h.svc.Ingest(projectID, &event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// IngestMinidump handles multipart minidump file upload via the API.
func (h *Handler) IngestMinidump(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.resolveProjectID(w, r)
	if !ok {
		return
	}

	err := r.ParseMultipartForm(64 << 20)
	if err != nil {
		http.Error(w, "failed to parse multipart form", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("upload_file_minidump")
	if err != nil {
		http.Error(w, "missing upload_file_minidump", http.StatusBadRequest)
		return
	}
	defer file.Close()
	dmpData, _ := io.ReadAll(file)

	event := &IngestEvent{
		ProjectID:   projectID,
		Environment: r.FormValue("environment"),
		Release:     r.FormValue("release"),
		Platform:    r.FormValue("platform"),
		Message:     "Minidump uploaded",
		Level:       "fatal",
	}

	id, err := h.svc.Ingest(projectID, event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	userID, _ := h.svc.ResolveUserID(projectID)
	go saveMinidump(h.dataDir, userID, id, bytes.NewReader(dmpData))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// ListIssues returns all issues for a project, ordered by last seen.
func (h *Handler) ListIssues(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.resolveProjectID(w, r)
	if !ok {
		return
	}

	rows, err := h.svc.db.Query(
		"SELECT id, title, level, status, count, first_seen, last_seen FROM issues WHERE project_id = ? ORDER BY last_seen DESC",
		projectID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var issues []map[string]interface{}
	for rows.Next() {
		var id, title, level, status string
		var count int
		var firstSeen, lastSeen int64
		if err := rows.Scan(&id, &title, &level, &status, &count, &firstSeen, &lastSeen); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		issues = append(issues, map[string]interface{}{
			"id": id, "title": title, "level": level, "status": status,
			"count": count, "first_seen": firstSeen, "last_seen": lastSeen,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(issues)
}

// IngestCrashpad handles Crashpad-compatible crash uploads. The DSN token is
// embedded in the URL path. It parses the minidump, optionally runs minidump_stackwalk
// for thread extraction, and persists the crash. Returns "CrashID=bp-<id>" on success.
func (h *Handler) IngestCrashpad(w http.ResponseWriter, r *http.Request) {
	dsnToken := chi.URLParam(r, "dsnToken")
	if dsnToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Discarded=malformed_no_annotations\n")
		return
	}
	projectID, err := h.svc.ResolveDSN(dsnToken)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Discarded=malformed_no_annotations\n")
		return
	}

	// Handle gzip encoding
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Discarded=malformed_bad_gzip\n")
			return
		}
		r.Body = &gzipReadCloser{r: gz, c: r.Body}
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Discarded=malformed_invalid_payload_structure\n")
		return
	}

	file, _, err := r.FormFile("upload_file_minidump")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Discarded=malformed_no_minidump\n")
		return
	}
	defer file.Close()
	dmpData, _ := io.ReadAll(file)

	// Collect all Crashpad annotations as form fields
	annotations := make(map[string]string)
	for key, vals := range r.MultipartForm.Value {
		if key == "upload_file_minidump" {
			continue
		}
		if len(vals) > 0 {
			annotations[key] = vals[0]
		}
	}

	userID, _ := h.svc.ResolveUserID(projectID)

	// Save minidump to a temp file so minidump_stackwalk can read it.
	tmpFile, err := os.CreateTemp("", "xentry-minidump-*.dmp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(dmpData); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Parse the minidump to extract threads for fingerprinting.
	symbolsDir := filepath.Join(h.dataDir, userID, "symbols")
	swResult, swErr := ParseMinidump(tmpPath, symbolsDir)
	if swErr != nil {
		log.Printf("crashpad: minidump-stackwalk failed (falling back to empty threads): %v\n", swErr)
	} else if len(swResult.Threads) == 0 {
		log.Printf("crashpad: minidump-stackwalk returned 0 threads for %s\n", tmpPath)
	}

	prod := annotations["prod"]
	ver := annotations["ver"]
	plat := annotations["plat"]
	reason := annotations["crash_reason"]

	// Prefer crash reason from minidump_stackwalk over annotation.
	if swErr == nil && swResult.CrashReason != "" {
		reason = swResult.CrashReason
	}

	message := prod + ": crash"
	if reason != "" {
		message = prod + ": " + reason
	}
	if prod == "" {
		message = "Crash"
	}

	event := &IngestEvent{
		ProjectID:    projectID,
		Release:      ver,
		Platform:     plat,
		Environment:  annotations["environment"],
		Message:      message,
		Level:        "fatal",
		Annotations:  annotations,
	}

	// Use parsed threads if stackwalk succeeded.
	if swErr == nil && len(swResult.Threads) > 0 {
		event.Threads = swResult.Threads
		event.StackwalkOutput = swResult.CrashedThreadRaw
	}

	id, err := h.svc.Ingest(projectID, event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Move temp file to final location.
	finalDir := filepath.Join(h.dataDir, userID, "minidumps")
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		log.Printf("crashpad: failed to create minidump dir: %v\n", err)
	} else {
		finalPath := filepath.Join(finalDir, id+".dmp")
		src, srcErr := os.Open(tmpPath)
		if srcErr == nil {
			dst, dstErr := os.Create(finalPath)
			if dstErr == nil {
				io.Copy(dst, src)
				dst.Close()
			} else {
				log.Printf("crashpad: failed to create minidump file: %v\n", dstErr)
			}
			src.Close()
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "CrashID=bp-%s\n", id)
}

// Routes returns a chi.Router with the crash API endpoints registered.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/{projectID}/events", h.IngestEvent)
	r.Post("/{projectID}/crash", h.IngestMinidump)
	r.Get("/{projectID}/issues", h.ListIssues)
	r.Get("/{projectID}/issues/{issueID}", h.GetIssue)
	return r
}

// GetIssue returns the details of a single issue including its latest event's threads and frames.
func (h *Handler) GetIssue(w http.ResponseWriter, r *http.Request) {
	projectID, ok := h.resolveProjectID(w, r)
	if !ok {
		return
	}
	issueID := chi.URLParam(r, "issueID")

	// Get issue details
	var title, level, status string
	var count int
	err := h.svc.db.QueryRow(
		"SELECT title, level, status, count FROM issues WHERE id = ? AND project_id = ?",
		issueID, projectID,
	).Scan(&title, &level, &status, &count)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	type FrameInfo struct {
		FrameNo      int    `json:"frame_no"`
		Function     string `json:"function"`
		File         string `json:"file"`
		Line         int    `json:"line"`
		Addr         string `json:"addr"`
		Module       string `json:"module"`
		Symbolicated bool   `json:"symbolicated"`
	}
	type ThreadInfo struct {
		Name    string      `json:"name"`
		Crashed bool        `json:"crashed"`
		Frames  []FrameInfo `json:"frames"`
	}
	type EventInfo struct {
		ID          string       `json:"id"`
		Timestamp   int64        `json:"timestamp"`
		Release     string       `json:"release"`
		Environment string       `json:"environment"`
		Platform    string       `json:"platform"`
		Message     string       `json:"message"`
		Threads     []ThreadInfo `json:"threads"`
	}

	// Get the latest event for this issue
	rows, err := h.svc.db.Query(
		"SELECT id, timestamp, release, environment, platform, message FROM events WHERE issue_id = ? ORDER BY timestamp DESC LIMIT 1",
		issueID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []EventInfo
	for rows.Next() {
		var e EventInfo
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Release, &e.Environment, &e.Platform, &e.Message); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Get threads for this event
		threadRows, err := h.svc.db.Query(
			"SELECT id, name, crashed FROM threads WHERE event_id = ?", e.ID,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for threadRows.Next() {
			var th ThreadInfo
			var threadID string
			var crashed int
			if err := threadRows.Scan(&threadID, &th.Name, &crashed); err != nil {
				threadRows.Close()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			th.Crashed = crashed == 1

			frameRows, err := h.svc.db.Query(
				"SELECT frame_no, function, file, line, addr, module, symbolicated FROM frames WHERE thread_id = ? ORDER BY frame_no",
				threadID,
			)
			if err != nil {
				frameRows.Close()
				threadRows.Close()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			for frameRows.Next() {
				var f FrameInfo
				var sym int
				if err := frameRows.Scan(&f.FrameNo, &f.Function, &f.File, &f.Line, &f.Addr, &f.Module, &sym); err != nil {
					frameRows.Close()
					threadRows.Close()
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				f.Symbolicated = sym == 1
				th.Frames = append(th.Frames, f)
			}
			frameRows.Close()
			e.Threads = append(e.Threads, th)
		}
		threadRows.Close()
		events = append(events, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     issueID,
		"title":  title,
		"level":  level,
		"status": status,
		"count":  count,
		"events": events,
	})
}
