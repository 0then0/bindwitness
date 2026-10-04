//go:build linux

package witness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func nativeFixtures(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command("sh", "../../scripts/build-fixtures.sh", root)
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build fixtures: %s: %v", b, e)
	}
	return root
}
func nativeConfig(root string) Config {
	objects := map[string]ObjectSpec{}
	for id, p := range map[string]string{"host": "host", "normal": "normal", "consumer": "consumer.so", "a": "a/libsame.so", "b": "b/libsame.so", "private": "libprivate.so", "private-consumer": "private-consumer.so", "version": "libversion.so", "version-consumer": "version-consumer.so"} {
		objects[id] = ObjectSpec{Root: "fixtures", Path: p}
	}
	return Config{SchemaVersion: 1, Command: []string{"./host", "./a/libsame.so", "./b/libsame.so", "./consumer.so", "normal"}, WorkingDirectory: root, Environment: map[string]string{}, Deadline: "5s", Limits: Limits{StdoutBytes: 4096, StderrBytes: 4096, TraceBytes: 1 << 20}, Roots: map[string]string{"fixtures": root}, Objects: objects, Selectors: []Selector{{Reference: "consumer", Symbol: "shared_value", Providers: []string{"a"}, Required: true}}}
}
func observe(t *testing.T, c Config) *Observation {
	t.Helper()
	o, e := Capture(c)
	if e != nil {
		t.Fatal(e)
	}
	if e = ValidateObservation(o); e != nil {
		t.Fatal(e)
	}
	return o
}
func assertOutcome(t *testing.T, o *Observation, c Config, want Outcome, id string) {
	t.Helper()
	r := Evaluate(o, c)
	if r.Outcome != want || id != "" && !hasFinding(r.Findings, id) {
		t.Fatalf("want %s/%s, got %s: %+v capture %+v", want, id, r.Outcome, r.Findings, o.Capture)
	}
}
func TestNativeBindings(t *testing.T) {
	root := nativeFixtures(t)
	c := nativeConfig(root)
	t.Run("normal executable", func(t *testing.T) {
		x := c
		x.Command = []string{"./normal"}
		x.Environment = nil
		x.Selectors = []Selector{{Reference: "normal", Symbol: "shared_value", Providers: []string{"a"}, Required: true}}
		o := observe(t, x)
		assertOutcome(t, o, x, Pass, "")
		if o.Provenance.EnvironmentOverrides == nil {
			t.Fatal("null overrides violate report schema")
		}
		if o.Workload.Stdout != "result=12\n" {
			t.Fatal(o.Workload)
		}
	})
	a := observe(t, c)
	assertOutcome(t, a, c, Pass, "")
	reverse := c
	reverse.Command = []string{"./host", "./b/libsame.so", "./a/libsame.so", "./consumer.so", "normal"}
	b := observe(t, reverse)
	assertOutcome(t, b, c, Fail, "PROVIDER_NOT_ALLOWED")
	if a.Workload.Stdout != b.Workload.Stdout || a.Workload.Stdout != "result=12\n" || *b.Workload.ExitCode != 0 {
		t.Fatal("same smoke result required")
	}
	cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
	if r := Compare(a, b, cc); r.Outcome != Fail || !hasFinding(r.Findings, "PROVIDER_CHANGED") {
		t.Fatal(r)
	}
	for _, o := range []*Observation{a, b} {
		for name, id := range o.Objects {
			if strings.Contains(name, "libsame.so") && id.ResolvedPath != "" && id.SONAME != "libsame.so" {
				t.Fatal(id)
			}
		}
	}
	t.Run("private names control", func(t *testing.T) {
		x := c
		x.Command = []string{"./host", "./b/libsame.so", "./a/libsame.so", "./private-consumer.so", "normal"}
		x.Selectors = []Selector{{Reference: "private-consumer", Symbol: "bw_private_value", Providers: []string{"private"}, Required: true}}
		assertOutcome(t, observe(t, x), x, Pass, "")
	})
	t.Run("versioned reference unversioned provider", func(t *testing.T) {
		x := c
		x.Command = []string{"./host", "./b/libsame.so", "./a/libsame.so", "./version-consumer.so", "normal"}
		v := "BW_1.0"
		x.Selectors = []Selector{{Reference: "version-consumer", Symbol: "shared_value", TraceVersion: &v, Providers: []string{"b"}, Required: true}}
		o := observe(t, x)
		assertOutcome(t, o, x, Pass, "")
		empty := ""
		x.Selectors[0].TraceVersion = &empty
		assertOutcome(t, o, x, Unresolved, "REQUIRED_NOT_OBSERVED")
	})
	t.Run("versioned provider control", func(t *testing.T) {
		x := c
		x.Command = []string{"./host", "./libversion.so", "./b/libsame.so", "./version-consumer.so", "normal"}
		v := "BW_1.0"
		x.Selectors = []Selector{{Reference: "version-consumer", Symbol: "shared_value", TraceVersion: &v, Providers: []string{"version"}, Required: true}}
		assertOutcome(t, observe(t, x), x, Pass, "")
	})
	t.Run("uncovered required", func(t *testing.T) {
		x := c
		x.Command = []string{"./normal", "idle"}
		x.Selectors = []Selector{{Reference: "normal", Symbol: "shared_value", Providers: []string{"a"}, Required: true}}
		assertOutcome(t, observe(t, x), x, Unresolved, "REQUIRED_NOT_OBSERVED")
	})
	t.Run("trace limit", func(t *testing.T) {
		x := c
		x.Limits.TraceBytes = 128
		assertOutcome(t, observe(t, x), x, Unresolved, "TRACE_TRUNCATED")
	})
	t.Run("large stdout stderr", func(t *testing.T) {
		x := c
		x.Command = append([]string{}, c.Command...)
		x.Command[4] = "noise"
		start := time.Now()
		o := observe(t, x)
		if time.Since(start) > 6*time.Second || !o.Workload.StdoutTruncated || !o.Workload.StderrTruncated {
			t.Fatalf("output truncation flags: stdout=%v stderr=%v", o.Workload.StdoutTruncated, o.Workload.StderrTruncated)
		}
		assertOutcome(t, o, x, Unresolved, "TRACE_TRUNCATED")
	})
	for _, tc := range []struct{ mode, id string }{{"reload", "UNSUPPORTED_RELOAD"}, {"namespace", "UNSUPPORTED_NAMESPACE"}, {"child", "UNSUPPORTED_PROCESS_TREE"}, {"malformed", "MALFORMED_BINDING"}} {
		t.Run(tc.mode, func(t *testing.T) {
			x := c
			x.Command = append([]string{}, c.Command...)
			x.Command[4] = tc.mode
			if tc.mode == "namespace" {
				x.Command[3] = "./version-consumer.so"
				x.Selectors = []Selector{{Reference: "version-consumer", Symbol: "shared_value", Providers: []string{"version"}, Required: true}}
			}
			o := observe(t, x)
			if o.Capture.Complete || !hasFinding(o.Capture.Issues, tc.id) {
				t.Fatal(o.Capture)
			}
			if Evaluate(o, x).Outcome == Pass {
				t.Fatal("false PASS")
			}
		})
	}
	t.Run("failure survives later truncation", func(t *testing.T) {
		x := reverse
		x.Command = append([]string{}, reverse.Command...)
		x.Command[4] = "noise"
		o := observe(t, x)
		assertOutcome(t, o, c, Fail, "PROVIDER_NOT_ALLOWED")
		if !hasFinding(Evaluate(o, c).Findings, "TRACE_TRUNCATED") {
			t.Fatal(o.Capture)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		x := c
		x.Command = append([]string{}, c.Command...)
		x.Command[4] = "sleep"
		x.Deadline = "100ms"
		o := observe(t, x)
		assertOutcome(t, o, x, Unresolved, "WORKLOAD_DEADLINE")
	})
	t.Run("nonzero workload separate", func(t *testing.T) {
		x := c
		x.Command = []string{"./host", "./a/libsame.so", "./b/libsame.so", "./consumer.so", "nonzero"}
		o := observe(t, x)
		if *o.Workload.ExitCode != 7 {
			t.Fatal(o.Workload)
		}
		assertOutcome(t, o, x, Pass, "")
	})
	t.Run("ambiguous aliases", func(t *testing.T) {
		x := c
		x.Objects = map[string]ObjectSpec{}
		for id, spec := range c.Objects {
			x.Objects[id] = spec
		}
		if e := os.Symlink("a/libsame.so", filepath.Join(root, "alias.so")); e != nil {
			t.Fatal(e)
		}
		x.Objects["alias"] = ObjectSpec{Root: "fixtures", Path: "alias.so"}
		assertOutcome(t, observe(t, x), x, Unresolved, "IDENTITY_UNRESOLVED")
	})
	t.Run("artifact changed", func(t *testing.T) {
		x := c
		x.Command = append([]string{}, c.Command...)
		x.Command[4] = "mutate"
		o := observe(t, x)
		if o.Objects[filepath.Join(root, "a/libsame.so")].Stability != "changed" {
			t.Fatal(o.Objects[filepath.Join(root, "a/libsame.so")])
		}
		assertOutcome(t, o, x, Unresolved, "IDENTITY_UNRESOLVED")
	})
}

func TestNativeArgvPreserved(t *testing.T) {
	root := nativeFixtures(t)
	for _, argv0 := range []string{"./host", "host"} {
		c := nativeConfig(root)
		c.Command[0] = argv0
		c.Environment["BW_EXPECT_ARGV0"] = argv0
		c.Environment["PATH"] = root + string(os.PathListSeparator) + os.Getenv("PATH")
		o := observe(t, c)
		assertOutcome(t, o, c, Pass, "")
		if *o.Workload.ExitCode != 0 || o.Workload.Stdout != "result=12\n" {
			t.Fatal(o.Workload)
		}
	}
}

func TestNativeRelativeProviderAfterChdir(t *testing.T) {
	root := nativeFixtures(t)
	c := nativeConfig(root)
	c.WorkingDirectory = filepath.Join(root, "a")
	c.Command = []string{filepath.Join(root, "relative-host"), filepath.Join(root, "b"), filepath.Join(root, "consumer.so")}
	c.Objects["relative-a"] = ObjectSpec{Root: "fixtures", Path: "a/librelative.so"}
	c.Objects["relative-b"] = ObjectSpec{Root: "fixtures", Path: "b/librelative.so"}
	c.Selectors[0].Providers = []string{"relative-a"}
	o := observe(t, c)
	assertOutcome(t, o, c, Unresolved, "IDENTITY_UNRESOLVED")
	if o.Workload.Stdout != "result=13\n" || *o.Workload.ExitCode != 0 {
		t.Fatal(o.Workload)
	}
	id := o.Objects["./librelative.so"]
	if id.ResolvedPath != "" || id.SHA256 != "" || id.LogicalID != "" || id.Stability != "ambiguous" {
		t.Fatalf("relative provider falsely attributed: %+v", id)
	}
}

func TestNativeCompareTruncatedProviderCoverage(t *testing.T) {
	root := nativeFixtures(t)
	c := nativeConfig(root)
	c.Objects["weak"] = ObjectSpec{Root: "fixtures", Path: "libweak.so"}
	c.Selectors[0].Providers = []string{"weak", "a"}
	c.Command = []string{filepath.Join(root, "multi-host"), filepath.Join(root, "libweak.so"), filepath.Join(root, "a/libsame.so"), filepath.Join(root, "consumer.so")}
	c.Environment = map[string]string{"LD_BIND_NOT": "1", "LD_DYNAMIC_WEAK": "1"}
	full := observe(t, c)
	assertOutcome(t, full, c, Pass, "")
	var selected []Binding
	for _, b := range full.Bindings {
		if b.Reference == filepath.Join(root, "consumer.so") && b.Symbol == "shared_value" {
			selected = append(selected, b)
		}
	}
	if len(selected) != 2 || selected[0].Provider == selected[1].Provider {
		t.Fatalf("fixture needs two distinct providers: %+v", selected)
	}
	end := (selected[0].TraceLine + selected[1].TraceLine) / 2
	c.Limits.TraceBytes = 0
	for _, line := range full.Trace[:end] {
		c.Limits.TraceBytes += len(line) + 1
	}
	partial := observe(t, c)
	assertOutcome(t, partial, c, Unresolved, "TRACE_TRUNCATED")
	if partial.Workload.Stdout != full.Workload.Stdout || partial.Workload.Stdout != "result=12\nresult=12\n" {
		t.Fatal(partial.Workload, full.Workload)
	}
	cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
	r := Compare(full, partial, cc)
	if r.Outcome != Unresolved || !hasFinding(r.Findings, "COMPARE_COVERAGE_GAP") || hasFinding(r.Findings, "PROVIDER_CHANGED") {
		t.Fatal(r)
	}
}
