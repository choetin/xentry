package crash

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// MinidumpResult contains the parsed output from minidump_stackwalk.
type MinidumpResult struct {
	Threads         []IngestThread
	CrashReason     string
	CrashAddress    string
	CrashedThreadRaw string // raw text output for the crashed thread
}

// stackwalkBin returns the platform-appropriate binary name.
func stackwalkBin() string {
	if runtime.GOOS == "windows" {
		return "minidump-stackwalk.exe"
	}
	return "minidump-stackwalk"
}

// ParseMinidump runs minidump_stackwalk on a minidump file and returns
// the extracted threads and crash info. Returns nil threads (not error)
// if the tool is not installed or the minidump cannot be parsed.
func ParseMinidump(dumpPath, symbolsDir string) (*MinidumpResult, error) {
	cmd := exec.Command(stackwalkBin(), "--symbols-path="+symbolsDir, dumpPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("minidump_stackwalk: %w: %s", err, stderr.String())
	}

	return parseStackwalkOutput(stdout.String()), nil
}

var (
	threadRe       = regexp.MustCompile(`^Thread (\d+)`)
	frameRe        = regexp.MustCompile(`^ (\d+)\s+`)
	crashReasonRe  = regexp.MustCompile(`(?i)^Crash reason:\s+(.+)`)
	crashAddressRe = regexp.MustCompile(`(?i)^Crash address:\s+(.+)`)
)

// parseStackwalkOutput parses the text output of minidump_stackwalk into a
// MinidumpResult, extracting per-thread frames and the crash reason/address.
func parseStackwalkOutput(output string) *MinidumpResult {
	result := &MinidumpResult{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	var currentThread *IngestThread
	var crashedThreadLines []string
	inCrashedThread := false

	for scanner.Scan() {
		line := scanner.Text()

		if m := crashReasonRe.FindStringSubmatch(line); m != nil {
			result.CrashReason = strings.TrimSpace(m[1])
			continue
		}
		if m := crashAddressRe.FindStringSubmatch(line); m != nil {
			result.CrashAddress = strings.TrimSpace(m[1])
			continue
		}

		if m := threadRe.FindStringSubmatch(line); m != nil {
			if currentThread != nil {
				result.Threads = append(result.Threads, *currentThread)
			}
			crashed := strings.Contains(line, "(crashed)")
			currentThread = &IngestThread{
				Crashed: crashed,
			}
			if crashed {
				inCrashedThread = true
				crashedThreadLines = nil
			} else {
				inCrashedThread = false
			}
			continue
		}

		// Frame lines start with " N " (space, number, spaces).
		// Skip metadata lines (eip=, esp=, Found by:) that start with 4+ spaces.
		if currentThread != nil && frameRe.MatchString(line) {
			currentThread.Frames = append(currentThread.Frames, parseFrameLine(line))
		}

		// Collect all lines belonging to the crashed thread (frames + metadata).
		if inCrashedThread {
			crashedThreadLines = append(crashedThreadLines, line)
		}
	}

	if currentThread != nil {
		result.Threads = append(result.Threads, *currentThread)
	}

	result.CrashedThreadRaw = strings.Join(crashedThreadLines, "\n")

	return result
}

// parseFrameLine parses a single frame line from minidump_stackwalk output.
//
//	Crashpad format (no symbols):
//	  0  ntdll.dll + 0x1234 (0x7ffd00001234 0x0 0x0 0x0 0x0)
//
//	Crashpad format (with symbols):
//	  0  myapp.exe!Crash + 0x5 (0x7ffd00001000 0x0 0x0 0x0 0x0)
//	       Found by: call frame info
//
//	Or with inline source info:
//	  0  myapp.exe!Crash [file.cpp : 42]
func parseFrameLine(line string) IngestFrame {
	var frame IngestFrame

	// Strip leading " N  " to get the content part.
	line = strings.TrimSpace(line)
	if idx := strings.IndexFunc(line, func(r rune) bool { return r >= '0' && r <= '9' }); idx >= 0 {
		rest := line[idx:]
		if spaceIdx := strings.Index(rest, " "); spaceIdx >= 0 {
			line = strings.TrimSpace(rest[spaceIdx:])
		}
	}

	// Strip trailing register values in parentheses: " (0x1234 0x0 ...)"
	if parenIdx := strings.Index(line, " ("); parenIdx >= 0 {
		line = line[:parenIdx]
	}

	// Check for "[file : line]" or "[file:line]" at the end.
	bracketStart := strings.LastIndex(line, " [")
	if bracketStart >= 0 {
		bracketEnd := strings.Index(line[bracketStart:], "]")
		if bracketEnd >= 0 {
			fileLine := line[bracketStart+2 : bracketStart+bracketEnd]
			// Try " : " first, then ":".
			colonIdx := strings.Index(fileLine, " : ")
			if colonIdx < 0 {
				colonIdx = strings.Index(fileLine, ":")
			}
			if colonIdx >= 0 {
				frame.File = strings.TrimSpace(fileLine[:colonIdx])
				rest := strings.TrimSpace(fileLine[colonIdx+1:])
				if spaceIdx := strings.Index(rest, " "); spaceIdx >= 0 {
					rest = rest[:spaceIdx]
				}
				// Remove leading "0x" prefix if present (e.g. "0x2a" -> "2a").
				rest = strings.TrimPrefix(rest, "0x")
				n, _ := strconv.ParseInt(rest, 10, 32)
				frame.Line = int(n)
			}
			line = strings.TrimSpace(line[:bracketStart])
		}
	}

	// Strip trailing " + offset" and capture the address for fingerprinting.
	if plusIdx := strings.LastIndex(line, " + "); plusIdx >= 0 {
		frame.Addr = strings.TrimSpace(line[plusIdx+3:])
		line = strings.TrimSpace(line[:plusIdx])
	}

	// Split "module!function" into module and function.
	if exclIdx := strings.Index(line, "!"); exclIdx >= 0 {
		frame.Module = line[:exclIdx]
		frame.Function = line[exclIdx+1:]
	} else if line != "" {
		frame.Module = line
	}

	return frame
}
