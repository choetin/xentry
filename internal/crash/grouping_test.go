package crash

import "testing"

func TestFingerprint_SameStack(t *testing.T) {
	stack1 := []Frame{
		{Function: "main", File: "main.cpp", Line: 10},
		{Function: "crash_func", File: "crash.cpp", Line: 42},
	}
	stack2 := []Frame{
		{Function: "main", File: "main.cpp", Line: 11},
		{Function: "crash_func", File: "crash.cpp", Line: 42},
	}
	fp1 := Fingerprint(stack1)
	fp2 := Fingerprint(stack2)
	if fp1 != fp2 {
		t.Errorf("same stack pattern should have same fingerprint: %s != %s", fp1, fp2)
	}
}

func TestFingerprint_DifferentStack(t *testing.T) {
	stack1 := []Frame{
		{Function: "main", File: "main.cpp", Line: 10},
		{Function: "crash_func", File: "crash.cpp", Line: 42},
	}
	stack2 := []Frame{
		{Function: "main", File: "main.cpp", Line: 10},
		{Function: "other_func", File: "other.cpp", Line: 99},
	}
	fp1 := Fingerprint(stack1)
	fp2 := Fingerprint(stack2)
	if fp1 == fp2 {
		t.Error("different stacks should have different fingerprints")
	}
}

func TestFingerprint_Empty(t *testing.T) {
	fp := Fingerprint(nil)
	if fp == "" {
		t.Error("fingerprint should not be empty even for nil input")
	}
}

func TestFingerprint_SameFunctionDifferentFile(t *testing.T) {
	stack1 := []Frame{{Function: "foo", File: "a.cpp", Line: 1}}
	stack2 := []Frame{{Function: "foo", File: "b.cpp", Line: 1}}
	if Fingerprint(stack1) == Fingerprint(stack2) {
		t.Error("different files should produce different fingerprints")
	}
}

func TestTitleFromStack(t *testing.T) {
	stack := []Frame{
		{Function: "main", File: "main.cpp", Line: 10},
		{Function: "crash_here", File: "crash.cpp", Line: 42},
	}
	title := TitleFromStack(stack)
	if title == "" {
		t.Error("title should not be empty")
	}
	// Should use the topmost frame
	if title != "main at main.cpp" {
		t.Errorf("unexpected title: %s", title)
	}
}

func TestTitleFromStack_Empty(t *testing.T) {
	title := TitleFromStack(nil)
	if title != "Unknown Crash" {
		t.Errorf("expected 'Unknown Crash', got %s", title)
	}
}

func TestTitleFromStack_UnknownFrames(t *testing.T) {
	stack := []Frame{
		{Function: "??", File: "??:0", Line: 0},
		{Function: "??", File: "??:0", Addr: "0x7fff1234"},
	}
	title := TitleFromStack(stack)
	// Should fall back to address since all functions are "??"
	if title == "" {
		t.Error("title should not be empty")
	}
}
