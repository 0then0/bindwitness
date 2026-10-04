package witness

import (
	"os"
	"strings"
	"testing"
)

func hasFinding(fs []Finding, id string) bool {
	for _, f := range fs {
		if f.ID == id {
			return true
		}
	}
	return false
}
func TestRealTraceFixtures(t *testing.T) {
	for _, file := range []string{"glibc-2.36-echo.trace", "glibc-2.41-echo.trace"} {
		t.Run(file, func(t *testing.T) {
			b, e := os.ReadFile("../../testdata/traces/" + file)
			if e != nil {
				t.Fatal(e)
			}
			lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
			p := prefixRE.FindStringSubmatch(lines[0])
			if p == nil {
				t.Fatal("prefix")
			}
			pid := 20
			if strings.Contains(file, "2.41") {
				pid = 21
			}
			events, issues := ParseTrace(lines, pid)
			if len(issues) > 0 {
				t.Fatal(issues)
			}
			if len(events) < 30 {
				t.Fatalf("only %d events", len(events))
			}
			for _, b := range events {
				if b.TraceLine < 1 || b.TraceLine > len(lines) {
					t.Fatal("bad evidence pointer")
				}
			}
		})
	}
}
func TestParserBoundaries(t *testing.T) {
	startup := " 10: transferring control: ./main"
	valid := " 10: binding file ./ref.so [0] to ./provider.so [0]: normal symbol `_Z3foov' [VER_1]"
	for _, tc := range []struct{ name, line, id string }{
		{"valid", valid, ""},
		{"malformed", "10: binding file ref to provider symbol `bad'", "MALFORMED_BINDING"},
		{"unknown kind", strings.Replace(valid, "normal", "exotic", 1), "MALFORMED_BINDING"},
		{"missing prefix", strings.TrimPrefix(valid, " 10: "), "MALFORMED_BINDING"},
		{"namespace", strings.Replace(valid, "[0]", "[1]", 1), "UNSUPPORTED_NAMESPACE"},
		{"overflow namespace", strings.Replace(valid, "[0]", "[99999999999999999999999999]", 1), "MALFORMED_BINDING"},
		{"child", strings.Replace(valid, "10:", "11:", 1), "UNSUPPORTED_PROCESS_TREE"},
		{"long", strings.Repeat("x", MaxTraceLine+1), "TRACE_LINE_LIMIT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events, issues := ParseTrace([]string{startup, tc.line}, 10)
			if tc.id != "" && !hasFinding(issues, tc.id) {
				t.Fatal(issues)
			}
			if tc.id == "" {
				if len(issues) > 0 || len(events) != 1 || events[0].Symbol != "_Z3foov" || events[0].TraceVersion != "VER_1" {
					t.Fatal(events, issues)
				}
			}
		})
	}
	_, issues := ParseTrace([]string{startup, "10: file=./ref.so [0]; generating link map", "10: file=./ref.so [0]; generating link map"}, 10)
	if !hasFinding(issues, "UNSUPPORTED_RELOAD") {
		t.Fatal(issues)
	}
}
func TestSplitLimits(t *testing.T) {
	trace, stderr, tt, st := SplitDiagnostics("10: transferring control: ./main\nplain stderr\n10: binding file broken", Limits{TraceBytes: 200, StderrBytes: 3})
	if len(trace) != 2 || stderr != "pla" || !tt || !st {
		t.Fatal(trace, stderr, tt, st)
	}
}
func FuzzParseTrace(f *testing.F) {
	f.Add("10: binding file ./r [0] to ./p [0]: normal symbol `s' [VER]")
	f.Add("binding file broken")
	for _, p := range []string{"glibc-2.36-echo.trace", "glibc-2.41-echo.trace"} {
		b, e := os.ReadFile("../../testdata/traces/" + p)
		if e != nil {
			f.Fatal(e)
		}
		f.Add(string(b))
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 128<<10 {
			t.Skip()
		}
		lines := strings.Split(s, "\n")
		events, _ := ParseTrace(lines, 10)
		for _, b := range events {
			if b.TraceLine < 1 || b.TraceLine > len(lines) || b.Symbol == "" {
				t.Fatalf("bad event %+v", b)
			}
		}
	})
}
