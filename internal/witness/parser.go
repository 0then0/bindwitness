package witness

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const MaxTraceLine = 64 << 10

var prefixRE = regexp.MustCompile(`^\s*([0-9]+):\s*(.*)$`)
var bindingRE = regexp.MustCompile("^binding file (.+) \\[([0-9]+)\\] to (.+) \\[([0-9]+)\\]: (normal|protected) symbol `([^']+)'(?: \\[([^]\\r\\n]+)\\])?$")
var mapRE = regexp.MustCompile(`^file=(.+) \[([0-9]+)\];\s+generating link map$`)
var namespaceRE = regexp.MustCompile(`\[([0-9]+)\]`)

func traceHash(lines []string) string {
	h := sha256.New()
	for _, s := range lines {
		h.Write([]byte(s))
		h.Write([]byte{'\n'})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func ParseTrace(lines []string, pid int) ([]Binding, []Finding) {
	events := []Binding{}
	issues := []Finding{}
	seenIssues := map[string]bool{}
	maps := map[string]int{}
	transfers := 0
	add := func(id, msg string) {
		if !seenIssues[id] {
			issues = append(issues, Finding{ID: id, Message: msg})
			seenIssues[id] = true
		}
	}
	for i, line := range lines {
		if len(line) > MaxTraceLine {
			add("TRACE_LINE_LIMIT", fmt.Sprintf("trace line %d exceeds parser limit", i+1))
			continue
		}
		p := prefixRE.FindStringSubmatch(line)
		body := line
		linePID := 0
		if p != nil {
			linePID, _ = strconv.Atoi(p[1])
			body = p[2]
			if linePID != pid {
				add("UNSUPPORTED_PROCESS_TREE", fmt.Sprintf("trace line %d belongs to PID %d, expected %d", i+1, linePID, pid))
			}
		}
		if strings.Contains(body, "binding file") || strings.Contains(body, "symbol `") {
			m := bindingRE.FindStringSubmatch(body)
			if m == nil || p == nil {
				add("MALFORMED_BINDING", fmt.Sprintf("unrecognized binding diagnostic at trace line %d", i+1))
				continue
			}
			rn, e1 := strconv.Atoi(m[2])
			pn, e2 := strconv.Atoi(m[4])
			if e1 != nil || e2 != nil {
				add("MALFORMED_BINDING", fmt.Sprintf("invalid namespace at trace line %d", i+1))
				continue
			}
			events = append(events, Binding{PID: linePID, Reference: m[1], Provider: m[3], Symbol: m[6], TraceVersion: m[7], ReferenceNamespace: rn, ProviderNamespace: pn, TraceLine: i + 1})
			if rn != 0 || pn != 0 {
				add("UNSUPPORTED_NAMESPACE", fmt.Sprintf("non-base namespace at trace line %d", i+1))
			}
		}
		if strings.Contains(body, "generating link map") {
			m := mapRE.FindStringSubmatch(body)
			if m == nil {
				add("UNSUPPORTED_FILES_FORMAT", fmt.Sprintf("unrecognized object lifetime diagnostic at trace line %d", i+1))
			} else {
				maps[m[1]]++
				if maps[m[1]] > 1 {
					add("UNSUPPORTED_RELOAD", fmt.Sprintf("repeated link map for %s at trace line %d", m[1], i+1))
				}
				if m[2] != "0" {
					add("UNSUPPORTED_NAMESPACE", "link map in non-base namespace")
				}
			}
		}
		// A second exec in the same PID has a new object lifetime, beyond this backend's scope.
		if strings.HasPrefix(body, "transferring control:") {
			transfers++
		}
		if strings.Contains(body, "file=") {
			for _, m := range namespaceRE.FindAllStringSubmatch(body, -1) {
				if m[1] != "0" {
					add("UNSUPPORTED_NAMESPACE", "files diagnostic in non-base namespace")
				}
			}
		}
	}
	if transfers == 0 {
		add("NO_LOADER_STARTUP", "no recognized transferring-control marker; diagnostics may be suppressed or unsupported")
	}
	if transfers > 1 {
		add("UNSUPPORTED_EXEC", "multiple executable lifetimes in one trace")
	}
	return events, issues
}

// SplitDiagnostics uses glibc's PID prefix. Workloads are trusted, including their stderr.
// Raw glibc lines are retained verbatim; stdout is handled by a separate pipe.
func SplitDiagnostics(raw string, limits Limits) ([]string, string, bool, bool) {
	trace := []string{}
	var stderr strings.Builder
	tn := 0
	tt, st := false, false
	for _, line := range strings.SplitAfter(raw, "\n") {
		if line == "" {
			continue
		}
		text := strings.TrimSuffix(line, "\n")
		diagnostic := prefixRE.MatchString(text) || strings.Contains(text, "binding file") || strings.Contains(text, "symbol `")
		if diagnostic {
			if tn+len(line) > limits.TraceBytes {
				tt = true
				continue
			}
			tn += len(line)
			trace = append(trace, text)
			if !strings.HasSuffix(line, "\n") {
				tt = true
			}
		} else {
			left := limits.StderrBytes - stderr.Len()
			if len(line) > left {
				st = true
				line = line[:left]
			}
			stderr.WriteString(line)
		}
	}
	return trace, stderr.String(), tt, st
}
