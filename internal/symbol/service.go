// Package symbol provides symbol file upload, storage, and lookup for crash symbolication.
package symbol

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/xentry/xentry/pkg/util"
)

// Service handles symbol file uploads, storage on disk, and database tracking.
type Service struct {
	db      *sql.DB
	dataDir string
}

// NewService creates a new symbol Service that stores files under dataDir.
func NewService(db *sql.DB, dataDir string) *Service {
	return &Service{db: db, dataDir: dataDir}
}

// SymbolFile represents a stored symbol file and its metadata.
type SymbolFile struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	Release    string `json:"release"`
	DebugID    string `json:"debug_id"`
	Type       string `json:"type"`
	Filepath   string `json:"filepath"`
	Size       int64  `json:"size"`
	UploadedAt int64  `json:"uploaded_at"`
}

// Upload receives a symbol file (optionally gzip-compressed), parses its MODULE
// header to extract the debug ID and filename, saves it to disk in the Breakpad
// directory layout, and records it in the database.
func (s *Service) Upload(projectID, userID, symType, release string, file io.Reader) (*SymbolFile, error) {
	// Read all content into a buffer so we can both parse the header and save the file.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return nil, fmt.Errorf("reading upload: %w", err)
	}
	raw := buf.Bytes()

	// Client sends gzip-compressed data; decompress on server side.
	var data []byte
	gzReader, gzErr := gzip.NewReader(bytes.NewReader(raw))
	if gzErr == nil {
		var decompressed bytes.Buffer
		if _, err := io.Copy(&decompressed, gzReader); err == nil {
			gzReader.Close()
			data = decompressed.Bytes()
		} else {
			gzReader.Close()
			data = raw
		}
	} else {
		data = raw
	}

	// Parse the MODULE header line to extract debug ID and PDB filename.
	var debugID, pdbFilename string
	scanner := &lineScanner{data: data}
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MODULE ") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				debugID = fields[3]
				pdbFilename = fields[4]
			}
		}
	}
	if debugID == "" || pdbFilename == "" {
		return nil, fmt.Errorf("invalid .sym file: missing MODULE header with debug ID and PDB filename")
	}

	id := util.UUID()
	symDir := filepath.Join(s.dataDir, userID, "symbols", pdbFilename, debugID)
	if err := os.MkdirAll(symDir, 0755); err != nil {
		return nil, err
	}

	symPath := filepath.Join(symDir, pdbFilename)
	// dump_syms strips the .pdb extension from the filename.
	if ext := filepath.Ext(symPath); strings.EqualFold(ext, ".pdb") {
		symPath = symPath[:len(symPath)-len(ext)]
	}
	symPath += ".sym"
	dst, err := os.Create(symPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	size, err := io.Copy(dst, bytes.NewReader(data))
	if err != nil {
		os.Remove(symPath)
		return nil, err
	}

	_, err = s.db.Exec(
		"INSERT INTO symbol_files (id, project_id, release, debug_id, type, filepath, size) VALUES (?,?,?,?,?,?,?)",
		id, projectID, release, debugID, symType, symPath, size,
	)
	if err != nil {
		os.Remove(symPath)
		return nil, err
	}

	return &SymbolFile{ID: id, ProjectID: projectID, Release: release, DebugID: debugID, Type: symType, Filepath: symPath, Size: size}, nil
}

// lineScanner is a minimal line scanner over a byte slice.
type lineScanner struct {
	data   []byte
	offset int
	start  int
}

// Scan advances to the next line. Returns false when there are no more lines.
func (s *lineScanner) Scan() bool {
	if s.offset >= len(s.data) {
		return false
	}
	s.start = s.offset
	idx := bytes.IndexByte(s.data[s.offset:], '\n')
	if idx >= 0 {
		s.offset += idx + 1
	} else {
		s.offset = len(s.data)
	}
	return true
}

// Text returns the current line without the trailing newline.
func (s *lineScanner) Text() string {
	end := s.offset
	if end > s.start && s.data[end-1] == '\n' {
		end--
	}
	if end > s.start && s.data[end-1] == '\r' {
		end--
	}
	return string(s.data[s.start:end])
}

// FindByDebugID returns the first symbol file matching the given debug ID.
func (s *Service) FindByDebugID(debugID string) (*SymbolFile, error) {
	var sf SymbolFile
	err := s.db.QueryRow(
		"SELECT id, project_id, release, debug_id, type, filepath, size, uploaded_at FROM symbol_files WHERE debug_id = ? LIMIT 1",
		debugID,
	).Scan(&sf.ID, &sf.ProjectID, &sf.Release, &sf.DebugID, &sf.Type, &sf.Filepath, &sf.Size, &sf.UploadedAt)
	if err != nil {
		return nil, err
	}
	return &sf, nil
}

// GetCachedSymbol looks up a previously resolved symbol from the cache.
func (s *Service) GetCachedSymbol(addr, debugID string) (*ResolvedFrame, error) {
	var rf ResolvedFrame
	err := s.db.QueryRow(
		"SELECT function, file, line FROM symbols_cache WHERE addr = ? AND debug_id = ?",
		addr, debugID,
	).Scan(&rf.Function, &rf.File, &rf.Line)
	if err != nil {
		return nil, err
	}
	rf.Addr = addr
	return &rf, nil
}

// CacheSymbol stores a resolved symbol in the cache for future lookups.
func (s *Service) CacheSymbol(addr, debugID, module, function, file string, line int) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO symbols_cache (addr, debug_id, module, function, file, line) VALUES (?,?,?,?,?,?)",
		addr, debugID, module, function, file, line,
	)
	return err
}

// ResolvedFrame holds the result of symbolication for a single address.
type ResolvedFrame struct {
	Addr     string `json:"addr"`
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// HashDebugID normalizes and hashes a debug ID for comparison purposes.
func HashDebugID(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:])
}
