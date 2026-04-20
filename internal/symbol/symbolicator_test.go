package symbol

import (
	"testing"
)

func TestSymbolize_NoSymbolFile(t *testing.T) {
	svc := NewService(nil, "") // nil DB — FindByDebugID will fail
	sym := NewSymbolicator(svc)

	// Should return unresolved frame gracefully when no symbol file found
	rf := sym.Symbolize("0x1234", "nonexistent-debug-id", "test.exe")
	if rf == nil {
		t.Fatal("expected non-nil result")
	}
	// When no symbol file found, returns fallback (addr with "??")
	if rf.Function != "??" {
		t.Errorf("expected '??' function, got %s", rf.Function)
	}
}

func TestSymbolize_InvalidAddr(t *testing.T) {
	// Just verify it doesn't panic
	svc := NewService(nil, "")
	sym := NewSymbolicator(svc)
	rf := sym.Symbolize("invalid", "nonexistent", "test")
	if rf == nil {
		t.Fatal("expected non-nil result")
	}
}
