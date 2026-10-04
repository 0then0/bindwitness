package witness

import (
	"path/filepath"
	"testing"
)

func TestOfflineSavedELFPlatformBoundary(t *testing.T) {
	setAMD64CaptureForTest(t, "true")
	for _, architecture := range []string{"amd64", "arm64"} {
		t.Run(architecture, func(t *testing.T) {
			foreign := "EM_AARCH64"
			if architecture == "arm64" {
				foreign = "EM_X86_64"
			}
			for _, tc := range []struct {
				name, path, class, machine, extraTrace, issue string
			}{
				{name: "native ELF64"},
				{name: "compat executable", path: "/scope/main", class: "ELFCLASS32", machine: "EM_386", issue: "UNVALIDATED_PLATFORM"},
				{name: "compat reference", path: "/scope/ref.so", class: "ELFCLASS32", machine: "EM_386", issue: "UNVALIDATED_PLATFORM"},
				{name: "x32 provider", path: "/scope/a.so", class: "ELFCLASS32", machine: "EM_X86_64", issue: "UNVALIDATED_PLATFORM"},
				{name: "foreign executable", path: "/scope/main", class: "ELFCLASS64", machine: foreign, issue: "UNVALIDATED_PLATFORM"},
				{name: "foreign provider", path: "/scope/a.so", class: "ELFCLASS64", machine: foreign, issue: "UNVALIDATED_PLATFORM"},
				{name: "unused declared foreign", path: "/scope/unused.so", class: "ELFCLASS64", machine: foreign},
				{name: "observed unselected foreign", path: "/scope/extra.so", class: "ELFCLASS64", machine: foreign, extraTrace: "10: binding file /scope/extra.so [0] to /scope/extra.so [0]: normal symbol `extra'", issue: "UNVALIDATED_PLATFORM"},
				{name: "child foreign ignored", path: "/scope/extra.so", class: "ELFCLASS64", machine: foreign, extraTrace: "11: binding file /scope/extra.so [0] to /scope/extra.so [0]: normal symbol `extra'", issue: "UNSUPPORTED_PROCESS_TREE"},
				{name: "namespace foreign ignored", path: "/scope/extra.so", class: "ELFCLASS64", machine: foreign, extraTrace: "10: binding file /scope/extra.so [1] to /scope/extra.so [1]: normal symbol `extra'", issue: "UNSUPPORTED_NAMESPACE"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					c := testConfig()
					o := testObservation("a", "")
					o.Provenance.ToolVersion = "0.1.0"
					setObservationArchitectureForTest(o, architecture)
					if tc.path != "" {
						identity := o.Objects["/scope/a.so"]
						identity.ObservedPath, identity.ResolvedPath = tc.path, tc.path
						identity.ELFClass, identity.Machine = tc.class, tc.machine
						o.Objects[tc.path] = identity
					}
					if tc.name == "unused declared foreign" {
						c.Objects["unused"] = ObjectSpec{Root: "scope", Path: "unused.so"}
					}
					if tc.extraTrace != "" {
						o.Trace = append(o.Trace, tc.extraTrace)
						o.Bindings, o.Capture.Issues = ParseTrace(o.Trace, o.Provenance.PID)
						o.Provenance.TraceSHA256 = traceHash(o.Trace)
						o.Capture.Complete = len(o.Capture.Issues) == 0
					}
					path := filepath.Join(t.TempDir(), "legacy.json")
					if err := WriteJSON(path, o); err != nil {
						t.Fatal(err)
					}
					saved, err := LoadObservation(path)
					if err != nil {
						t.Fatal(err)
					}
					cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
					want := Pass
					if tc.issue != "" {
						want = Unresolved
					}
					for _, result := range []*Result{Evaluate(saved, c), Compare(saved, saved, cc)} {
						if result.Outcome != want || result.BindingVerdict != want || tc.issue != "" && !hasFinding(result.Findings, tc.issue) {
							t.Fatalf("want %s/%s, got %+v", want, tc.issue, result)
						}
						if tc.issue != "UNVALIDATED_PLATFORM" && hasFinding(result.Findings, "UNVALIDATED_PLATFORM") {
							t.Fatalf("unused or out-of-scope object changed platform verdict: %+v", result)
						}
					}
				})
			}
		})
	}
}

func TestOfflineUnsupportedELFPreservesFailure(t *testing.T) {
	setAMD64CaptureForTest(t, "true")
	for _, architecture := range []string{"amd64", "arm64"} {
		for _, target := range []string{"/scope/main", "/scope/ref.so", "/scope/b.so"} {
			t.Run(architecture+target, func(t *testing.T) {
				c := testConfig()
				cc := CompareConfig{SchemaVersion: 1, LeftRoots: c.Roots, RightRoots: c.Roots, Objects: c.Objects, Selectors: c.Selectors}
				left, right := testObservation("a", ""), testObservation("b", "")
				setObservationArchitectureForTest(left, architecture)
				setObservationArchitectureForTest(right, architecture)
				identity := right.Objects[target]
				identity.ELFClass, identity.Machine = "ELFCLASS32", "EM_386"
				right.Objects[target] = identity
				for _, result := range []*Result{Evaluate(right, c), Compare(left, right, cc)} {
					finding := "PROVIDER_NOT_ALLOWED"
					if result.Kind == "compare" {
						finding = "PROVIDER_CHANGED"
					}
					if result.Outcome != Fail || result.BindingVerdict != Fail || !hasFinding(result.Findings, finding) || !hasFinding(result.Findings, "UNVALIDATED_PLATFORM") {
						t.Fatalf("unsupported ELF hid concrete failure: %+v", result)
					}
				}
			})
		}
	}
}
