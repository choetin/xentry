package crash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Frame represents a single stack frame, either symbolicated (with Function/File)
// or raw (with Module/Addr).
type Frame struct {
	Function string
	File     string
	Line     int
	Addr     string
	Module   string
}

// Fingerprint computes a stable SHA-256 hash for issue grouping based on the
// call stack. Symbolicated frames use function+file, unsymbolicated frames
// use module+address. An empty stack yields "empty-stack".
func Fingerprint(stack []Frame) string {
	if len(stack) == 0 {
		return "empty-stack"
	}
	var b strings.Builder
	for _, f := range stack {
		if f.Function != "" && f.Function != "??" {
			// Symbolicated: use function + file
			b.WriteString(f.Function)
			b.WriteByte('|')
			b.WriteString(baseName(f.File))
		} else {
			// Unsymbolicated: use module + address
			b.WriteString(f.Module)
			b.WriteByte('|')
			b.WriteString(f.Addr)
		}
		b.WriteByte('|')
	}
	hash := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(hash[:])
}

// baseName returns the file name portion of a path, handling both Unix and Windows separators.
func baseName(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	if idx := strings.LastIndex(path, "\\"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// TitleFromStack generates a human-readable issue title from the first
// symbolicated frame in the stack. Falls back to the raw address or "Unknown Crash".
func TitleFromStack(stack []Frame) string {
	if len(stack) == 0 {
		return "Unknown Crash"
	}
	for _, f := range stack {
		if f.Function != "" && f.Function != "??" {
			return fmt.Sprintf("%s at %s", f.Function, baseName(f.File))
		}
	}
	if stack[0].Addr != "" {
		return fmt.Sprintf("0x%s", stack[0].Addr)
	}
	return "Unknown Crash"
}
