package witness

import (
	"path/filepath"
	"strings"
	"testing"
)

func testObservation(provider, version string) *Observation {
	line := "10: binding file /scope/ref.so [0] to /scope/" + provider + ".so [0]: normal symbol `value'"
	if version != "" {
		line += " [" + version + "]"
	}
	o := &Observation{SchemaVersion: 1, Kind: "observation", Provenance: Provenance{PID: 10, WorkingDirectory: "/scope"}, Workload: Workload{Started: true, Completed: true}, Capture: CaptureState{Complete: true, Issues: []Finding{}}, Trace: []string{"10: transferring control: /scope/main", line}, Objects: map[string]Identity{}}
	o.Bindings, _ = ParseTrace(o.Trace, 10)
	o.Provenance.TraceSHA256 = traceHash(o.Trace)
	for _, name := range []string{"ref", provider} {
		p := "/scope/" + name + ".so"
		o.Objects[p] = Identity{ObservedPath: p, ResolvedPath: p, SHA256: strings.Repeat("a", 64), ELFClass: "ELFCLASS64", Machine: "EM_AARCH64", Stability: "pre_post_unchanged"}
	}
	return o
}

func TestCheckReportSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "check.json")
	for _, version := range []int{0, 999, SchemaVersion} {
		r := Evaluate(testObservation("a", ""), testConfig())
		r.SchemaVersion = version
		if err := WriteJSON(path, r); err != nil {
			t.Fatal(err)
		}
		_, err := LoadObservation(path)
		if (err == nil) != (version == SchemaVersion) {
			t.Fatalf("version %d: %v", version, err)
		}
	}
}

func TestRelativeSavedIdentityCannotSupplyVerdict(t *testing.T) {
	c := testConfig()
	for _, provider := range []string{"a", "b"} {
		o := testObservation(provider, "")
		name := "./" + provider + ".so"
		o.Trace[1] = strings.ReplaceAll(o.Trace[1], "/scope/"+provider+".so", name)
		o.Bindings, _ = ParseTrace(o.Trace, o.Provenance.PID)
		o.Provenance.TraceSHA256 = traceHash(o.Trace)
		o.Objects[name] = o.Objects["/scope/"+provider+".so"]
		if r := Evaluate(o, c); r.Outcome != Unresolved || hasFinding(r.Findings, "PROVIDER_NOT_ALLOWED") {
			t.Fatalf("relative identity %q supplied a verdict: %+v", name, r)
		}
		cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
		if r := Compare(testObservation("a", ""), o, cc); r.Outcome != Unresolved || hasFinding(r.Findings, "PROVIDER_CHANGED") {
			t.Fatalf("relative identity %q supplied a comparison witness: %+v", name, r)
		}
	}
}

func TestCompareProviderCoverage(t *testing.T) {
	c := testConfig()
	cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
	multi := testObservation("a", "")
	other := testObservation("b", "")
	multi.Trace = append(multi.Trace, other.Trace[1])
	multi.Bindings, _ = ParseTrace(multi.Trace, multi.Provenance.PID)
	multi.Provenance.TraceSHA256 = traceHash(multi.Trace)
	multi.Objects["/scope/b.so"] = other.Objects["/scope/b.so"]
	for _, partial := range []bool{false, true} {
		one := testObservation("a", "")
		if partial {
			one.Capture = CaptureState{TraceTruncated: true, Issues: []Finding{{ID: "TRACE_TRUNCATED"}}}
		}
		for _, sides := range [][2]*Observation{{multi, one}, {one, multi}} {
			r := Compare(sides[0], sides[1], cc)
			if r.Outcome != Unresolved || !hasFinding(r.Findings, "COMPARE_COVERAGE_GAP") || hasFinding(r.Findings, "PROVIDER_CHANGED") {
				t.Fatal(r)
			}
		}
	}
	if r := Compare(multi, multi, cc); r.Outcome != Pass {
		t.Fatal(r)
	}
	// A concrete different-provider witness survives independent trace loss.
	other.Capture = CaptureState{TraceTruncated: true, Issues: []Finding{{ID: "TRACE_TRUNCATED"}}}
	if r := Compare(testObservation("a", ""), other, cc); r.Outcome != Fail || !hasFinding(r.Findings, "TRACE_TRUNCATED") {
		t.Fatal(r)
	}
}
func testConfig() Config {
	return Config{SchemaVersion: 1, Roots: map[string]string{"scope": "/scope"}, Objects: map[string]ObjectSpec{"ref": {Root: "scope", Path: "ref.so"}, "a": {Root: "scope", Path: "a.so"}, "b": {Root: "scope", Path: "b.so"}}, Selectors: []Selector{{Reference: "ref", Symbol: "value", Providers: []string{"a"}, Required: true}}}
}
func TestContractOutcomes(t *testing.T) {
	c := testConfig()
	a, b := testObservation("a", ""), testObservation("b", "")
	if r := Evaluate(a, c); r.Outcome != Pass {
		t.Fatal(r)
	}
	if r := Evaluate(b, c); r.Outcome != Fail || !hasFinding(r.Findings, "PROVIDER_NOT_ALLOWED") {
		t.Fatal(r)
	}
	b.Capture.Complete = false
	b.Capture.TraceTruncated = true
	b.Capture.Issues = []Finding{{ID: "TRACE_TRUNCATED"}}
	if r := Evaluate(b, c); r.Outcome != Fail || !hasFinding(r.Findings, "TRACE_TRUNCATED") {
		t.Fatal(r)
	}
	if r := Evaluate(a, func() Config { x := testConfig(); x.Selectors[0].Symbol = "unused"; return x }()); r.Outcome != Unresolved {
		t.Fatal(r)
	}
	empty := ""
	c.Selectors[0].TraceVersion = &empty
	if r := Evaluate(testObservation("a", "VER_1"), c); r.Outcome != Unresolved {
		t.Fatal(r)
	}
	id := a.Objects["/scope/a.so"]
	id.Stability = "changed"
	id.Problem = "changed"
	a.Objects["/scope/a.so"] = id
	if r := Evaluate(a, c); r.Outcome != Unresolved {
		t.Fatal(r)
	}
}
func TestArtifactChangeIsSeparate(t *testing.T) {
	c := testConfig()
	cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
	a, b := testObservation("a", ""), testObservation("a", "")
	id := b.Objects["/scope/a.so"]
	id.SHA256 = strings.Repeat("b", 64)
	b.Objects["/scope/a.so"] = id
	r := Compare(a, b, cc)
	if r.Outcome != Pass || len(r.ArtifactChanges) != 1 {
		t.Fatal(r)
	}
	spec := c.Objects["a"]
	spec.SHA256 = strings.Repeat("a", 64)
	c.Objects["a"] = spec
	if r = Evaluate(b, c); r.Outcome != Fail || !hasFinding(r.Findings, "ARTIFACT_PIN_MISMATCH") {
		t.Fatal(r)
	}
	if r = Compare(a, testObservation("b", ""), cc); r.Outcome != Fail || !hasFinding(r.Findings, "PROVIDER_CHANGED") {
		t.Fatal(r)
	}
	if r = Compare(a, testObservation("a", "VER_1"), cc); r.Outcome != Unresolved || hasFinding(r.Findings, "PROVIDER_CHANGED") {
		t.Fatal(r)
	}
}
func TestEvidenceIntegrity(t *testing.T) {
	o := testObservation("a", "")
	o.Trace = o.Trace[:1]
	if ValidateObservation(o) == nil {
		t.Fatal("truncated trace accepted")
	}
	o = testObservation("a", "")
	o.Bindings[0].Provider = "fake"
	if ValidateObservation(o) == nil {
		t.Fatal("edited binding accepted")
	}
}

func TestComparisonEvidenceSide(t *testing.T) {
	c := testConfig()
	cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
	left, right := testObservation("a", ""), testObservation("a", "")
	id := right.Objects["/scope/a.so"]
	id.Stability = "post_only"
	right.Objects["/scope/a.so"] = id
	r := Compare(left, right, cc)
	if r.Outcome != Unresolved {
		t.Fatal(r)
	}
	for _, f := range r.Findings {
		if f.ID == "IDENTITY_UNRESOLVED" {
			if f.Witness != nil || f.OtherWitness == nil {
				t.Fatal("right evidence attributed to left", f)
			}
			return
		}
	}
	t.Fatal("missing identity finding")
}
