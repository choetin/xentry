package symbol

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Symbolicator resolves raw addresses to function names, source files, and line numbers
// using platform-appropriate tools (addr2line, atos, llvm-symbolizer).
type Symbolicator struct {
	svc *Service
}

// NewSymbolicator creates a new Symbolicator backed by the given symbol Service.
func NewSymbolicator(svc *Service) *Symbolicator {
	return &Symbolicator{svc: svc}
}

// Symbolize resolves an address to a function name, file, and line number.
// It first checks the cache, then looks up the symbol file by debug ID and
// invokes the appropriate resolution tool based on the symbol file type.
func (s *Symbolicator) Symbolize(addr, debugID, module string) *ResolvedFrame {
	fallback := &ResolvedFrame{Addr: addr, Function: "??", File: "", Line: 0}
	if s.svc == nil || s.svc.db == nil {
		return fallback
	}

	// Check cache first
	cached, err := s.svc.GetCachedSymbol(addr, debugID)
	if err == nil && cached != nil {
		return cached
	}

	// Find symbol file
	sf, err := s.svc.FindByDebugID(debugID)
	if err != nil {
		return &ResolvedFrame{Addr: addr, Function: "??", File: "", Line: 0}
	}

	// Resolve based on symbol type
	var rf *ResolvedFrame
	switch sf.Type {
	case "breakpad":
		rf = s.resolveBreakpad(sf.Filepath, addr)
	case "dwarf":
		rf = s.resolveDWARF(sf.Filepath, addr)
	case "dsym":
		rf = s.resolveDSYM(sf.Filepath, addr)
	case "pdb":
		rf = s.resolvePDB(sf.Filepath, addr)
	default:
		rf = &ResolvedFrame{Addr: addr, Function: "??", File: "", Line: 0}
	}

	// Cache the result
	if rf != nil {
		s.svc.CacheSymbol(addr, debugID, module, rf.Function, rf.File, rf.Line)
	}

	return rf
}

// resolveDWARF uses addr2line to resolve an address in an ELF/Mach-O binary.
func (s *Symbolicator) resolveDWARF(binaryPath, addr string) *ResolvedFrame {
	tool := "addr2line"
	if runtime.GOOS == "windows" {
		tool = "addr2line.exe"
	}
	cmd := exec.Command(tool, "-e", binaryPath, "-f", addr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &ResolvedFrame{Addr: addr, Function: "??", File: "", Line: 0}
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	rf := &ResolvedFrame{Addr: addr}
	if len(lines) >= 1 {
		rf.Function = lines[0]
	}
	if len(lines) >= 2 {
		parts := strings.SplitN(lines[1], ":", 2)
		if len(parts) == 2 {
			rf.File = parts[0]
			rf.Line, _ = strconv.Atoi(parts[1])
		}
	}
	return rf
}

// resolveDSYM uses atos to resolve an address in a macOS dSYM bundle.
func (s *Symbolicator) resolveDSYM(dsymPath, addr string) *ResolvedFrame {
	cmd := exec.Command("atos", "-o", dsymPath, addr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &ResolvedFrame{Addr: addr, Function: "??", File: "", Line: 0}
	}
	// atos output format: "function (in module) (file:line)"
	text := strings.TrimSpace(string(out))
	rf := &ResolvedFrame{Addr: addr}
	// Parse "function_name (in module) (file:line)"
	if idx := strings.Index(text, " (in "); idx > 0 {
		rf.Function = text[:idx]
		rest := text[idx:]
		if start := strings.Index(rest, ") ("); start >= 0 {
			inner := rest[start+2:]
			if end := strings.Index(inner, ")"); end > 0 {
				loc := inner[:end]
				parts := strings.SplitN(loc, ":", 2)
				if len(parts) == 2 {
					rf.File = parts[0]
					rf.Line, _ = strconv.Atoi(parts[1])
				}
			}
		}
	}
	return rf
}

// resolvePDB uses llvm-symbolizer to resolve an address in a PDB file.
func (s *Symbolicator) resolvePDB(pdbPath, addr string) *ResolvedFrame {
	cmd := exec.Command("llvm-symbolizer", "--obj="+pdbPath, addr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &ResolvedFrame{Addr: addr, Function: "??", File: "", Line: 0}
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	rf := &ResolvedFrame{Addr: addr}
	if len(lines) >= 1 {
		rf.Function = lines[0]
	}
	if len(lines) >= 2 {
		parts := strings.SplitN(lines[1], ":", 2)
		if len(parts) == 2 {
			rf.File = parts[0]
			rf.Line, _ = strconv.Atoi(parts[1])
		}
	}
	return rf
}

// resolveBreakpad attempts to resolve an address from a Breakpad .sym file.
// Currently returns unresolved; full .sym parsing can be added later.
func (s *Symbolicator) resolveBreakpad(symPath, addr string) *ResolvedFrame {
	// Breakpad .sym format: MODULE os arch debug_id name
	// FUNC address size function_name
	// then lines: address size line file
	// For now, return unresolved — full .sym parsing is complex
	// This can be enhanced later
	return &ResolvedFrame{Addr: addr, Function: "??", File: symPath, Line: 0}
}

// BatchRequest is a single frame to symbolicate.
type BatchRequest struct {
	Addr    string `json:"addr"`
	DebugID string `json:"debug_id"`
	Module  string `json:"module"`
}

// BatchSymbolize resolves multiple frames in one call.
func (s *Symbolicator) BatchSymbolize(frames []BatchRequest) []*ResolvedFrame {
	results := make([]*ResolvedFrame, len(frames))
	for i, f := range frames {
		results[i] = &ResolvedFrame{Addr: f.Addr}
		if f.DebugID != "" {
			resolved := s.Symbolize(f.Addr, f.DebugID, f.Module)
			if resolved != nil {
				results[i] = resolved
			}
		}
	}
	return results
}
