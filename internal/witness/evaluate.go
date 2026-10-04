package witness

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
)

// ValidateObservation recomputes parser results from embedded evidence, without
// reading historical paths from the current filesystem. Saved reports are trusted inputs,
// not signed attestations, but accidental truncation and event edits are detected.
func ValidateObservation(o *Observation) error {
	if o.SchemaVersion != SchemaVersion || o.Kind != "observation" {
		return fmt.Errorf("unsupported observation schema or kind")
	}
	if o.Provenance.TraceSHA256 != traceHash(o.Trace) {
		return fmt.Errorf("raw trace checksum mismatch")
	}
	events, issues := ParseTrace(o.Trace, o.Provenance.PID)
	if !reflect.DeepEqual(events, o.Bindings) {
		return fmt.Errorf("bindings disagree with embedded raw trace")
	}
	for _, issue := range issues {
		if !slices.ContainsFunc(o.Capture.Issues, func(f Finding) bool { return f.ID == issue.ID }) {
			return fmt.Errorf("parser issue %s absent from saved capture state", issue.ID)
		}
	}
	if o.Capture.Complete && (len(o.Capture.Issues) > 0 || o.Capture.TraceTruncated || !o.Workload.Started || !o.Workload.Completed || o.Workload.DeadlineExceeded) {
		return fmt.Errorf("inconsistent capture completion state")
	}
	return nil
}
func LoadObservation(path string) (*Observation, error) {
	// Both capture observations and check reports are reusable offline evidence.
	var header struct {
		Kind string `json:"kind"`
	} // read full concrete types below
	// Avoid DisallowUnknownFields only for this discriminator.
	raw, err := readKind(path)
	if err != nil {
		return nil, err
	}
	header.Kind = raw
	var o *Observation
	switch header.Kind {
	case "observation":
		o = &Observation{}
		err = ReadJSON(path, o)
	case "check":
		r := Result{}
		err = ReadJSON(path, &r)
		if err == nil && r.SchemaVersion != SchemaVersion {
			return nil, fmt.Errorf("unsupported check report schema_version %d", r.SchemaVersion)
		}
		o = r.Observation
	default:
		return nil, fmt.Errorf("expected observation or check report")
	}
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, fmt.Errorf("report has no observation")
	}
	return o, ValidateObservation(o)
}
func mapping(o *Observation, paths map[string]string) map[string]string {
	out := map[string]string{}
	for name, ident := range o.Objects {
		observed := ""
		if filepath.IsAbs(name) || len(name) > 0 && name[0] == '.' {
			observed = absolute(o.Provenance.WorkingDirectory, name)
		}
		ids := []string{}
		for _, id := range sortedKeys(paths) {
			p := paths[id]
			resolved := p
			if declared, ok := o.Objects[p]; ok && declared.ResolvedPath != "" {
				resolved = declared.ResolvedPath
			}
			if resolved == ident.ResolvedPath || p == observed {
				ids = append(ids, id)
			}
		}
		if len(ids) == 1 {
			out[name] = ids[0]
		}
	}
	return out
}
func selected(b Binding, s Selector, ids map[string]string, pid int) bool {
	return b.PID == pid && b.ReferenceNamespace == 0 && b.ProviderNamespace == 0 && ids[b.Reference] == s.Reference && b.Symbol == s.Symbol && (s.TraceVersion == nil || b.TraceVersion == *s.TraceVersion)
}
func trustedIdentity(o *Observation, name string) bool {
	id, ok := o.Objects[name]
	return objectPathKnown(o, name) && ok && id.Problem == "" && id.ResolvedPath != "" && validHash(id.SHA256) && id.ELFClass != "" && id.Machine != "" && id.Stability == "pre_post_unchanged"
}
func objectPathKnown(o *Observation, name string) bool {
	return filepath.IsAbs(name) || len(o.Provenance.Command) > 0 && name == o.Provenance.Command[0]
}
func baseResult(kind string) *Result {
	return &Result{SchemaVersion: SchemaVersion, Kind: kind, Findings: []Finding{}, Coverage: []Coverage{}, Outcome: Pass, BindingVerdict: Pass}
}
func unresolved(r *Result, f Finding) {
	r.Findings = append(r.Findings, f)
	if r.Outcome != Fail {
		r.Outcome = Unresolved
	}
}
func failure(r *Result, f Finding) {
	r.Findings = append(r.Findings, f)
	r.Outcome = Fail
	r.BindingVerdict = Fail
}
func captureIssues(r *Result, o *Observation, side string) {
	if !o.Capture.Complete {
		unresolved(r, Finding{ID: "CAPTURE_INCOMPLETE", Message: side + " capture incomplete"})
	}
	for _, f := range o.Capture.Issues {
		f.Message = side + " " + f.Message
		unresolved(r, f)
	}
}
func Evaluate(o *Observation, c Config) *Result {
	r := baseResult("check")
	r.Observation = o
	if e := ValidateObservation(o); e != nil {
		unresolved(r, Finding{ID: "INVALID_OBSERVATION", Message: e.Error()})
		r.BindingVerdict = Unresolved
		return r
	}
	paths, err := objectPaths(c.Roots, c.Objects)
	if err != nil {
		r.Outcome = InfrastructureError
		r.BindingVerdict = Unresolved
		r.Findings = append(r.Findings, Finding{ID: "INVALID_CONTRACT", Message: err.Error()})
		return r
	}
	ids := mapping(o, paths)
	captureIssues(r, o, "")
	for i, s := range c.Selectors {
		count := 0
		for _, b := range o.Bindings {
			if !selected(b, s, ids, o.Provenance.PID) {
				continue
			}
			count++
			index := i
			binding := b
			if !trustedIdentity(o, b.Reference) || !trustedIdentity(o, b.Provider) {
				unresolved(r, Finding{ID: "IDENTITY_UNRESOLVED", Message: "selected binding lacks an unchanged pre/post file identity", SelectorIndex: &index, Witness: &binding})
			}
			provider := ids[b.Provider]
			// A fully resolved observed path is a positive witness, even when its artifact
			// stability is independently unresolved. Never infer a provider from exports.
			if !slices.Contains(s.Providers, provider) {
				ident := o.Objects[b.Provider]
				if trustedIdentity(o, b.Reference) && objectPathKnown(o, b.Provider) && ident.ResolvedPath != "" && ident.Problem == "" {
					failure(r, Finding{ID: "PROVIDER_NOT_ALLOWED", Message: fmt.Sprintf("%s -> %s [%s] -> %s (%s); expected %v", s.Reference, b.Symbol, b.TraceVersion, provider, b.Provider, s.Providers), SelectorIndex: &index, Witness: &binding, ExpectedProviders: s.Providers})
				} else {
					unresolved(r, Finding{ID: "PROVIDER_IDENTITY_AMBIGUOUS", Message: "cannot establish observed provider identity", SelectorIndex: &index, Witness: &binding})
				}
			}
			for _, name := range []string{b.Reference, b.Provider} {
				id := ids[name]
				pin := c.Objects[id].SHA256
				if pin != "" && trustedIdentity(o, name) && o.Objects[name].SHA256 != pin {
					failure(r, Finding{ID: "ARTIFACT_PIN_MISMATCH", Message: "observed artifact hash differs from exact pin for " + id, SelectorIndex: &index, Witness: &binding})
				}
			}
		}
		r.Coverage = append(r.Coverage, Coverage{SelectorIndex: i, Required: s.Required, Observed: count})
		if s.Required && count == 0 {
			idx := i
			unresolved(r, Finding{ID: "REQUIRED_NOT_OBSERVED", Message: fmt.Sprintf("insufficient workload coverage for %s -> %s", s.Reference, s.Symbol), SelectorIndex: &idx})
		}
	}
	if r.Outcome == Unresolved && r.BindingVerdict != Fail {
		r.BindingVerdict = Unresolved
	}
	return finishResult(r)
}
func Compare(left, right *Observation, c CompareConfig) *Result {
	r := baseResult("compare")
	r.Left = left
	r.Right = right
	for _, o := range []*Observation{left, right} {
		if e := ValidateObservation(o); e != nil {
			unresolved(r, Finding{ID: "INVALID_OBSERVATION", Message: e.Error()})
			r.BindingVerdict = Unresolved
			return r
		}
	}
	lp, e := objectPaths(c.LeftRoots, c.Objects)
	if e != nil {
		return invalidCompare(r, e)
	}
	rp, e := objectPaths(c.RightRoots, c.Objects)
	if e != nil {
		return invalidCompare(r, e)
	}
	lm, rm := mapping(left, lp), mapping(right, rp)
	captureIssues(r, left, "left")
	captureIssues(r, right, "right")
	artifactSeen := map[string]bool{}
	for i, s := range c.Selectors {
		ls, rs := map[string][]Binding{}, map[string][]Binding{}
		for side, o := range []*Observation{left, right} {
			ids := lm
			dst := ls
			if side == 1 {
				ids = rm
				dst = rs
			}
			for _, b := range o.Bindings {
				if !selected(b, s, ids, o.Provenance.PID) {
					continue
				}
				if !trustedIdentity(o, b.Reference) || !trustedIdentity(o, b.Provider) || ids[b.Provider] == "" {
					bb := b
					finding := Finding{ID: "IDENTITY_UNRESOLVED", Message: "comparison binding has unresolved identity or unmapped provider"}
					if side == 0 {
						finding.Witness = &bb
					} else {
						finding.OtherWitness = &bb
					}
					unresolved(r, finding)
					continue
				}
				dst[b.TraceVersion] = append(dst[b.TraceVersion], b)
			}
		}
		versions := map[string]bool{}
		for v := range ls {
			versions[v] = true
		}
		for v := range rs {
			versions[v] = true
		}
		if len(versions) == 0 && s.Required {
			idx := i
			unresolved(r, Finding{ID: "REQUIRED_NOT_OBSERVED", Message: "selector absent on both comparison sides", SelectorIndex: &idx})
		}
		r.Coverage = append(r.Coverage, Coverage{SelectorIndex: i, Required: s.Required, Observed: min(bindingCount(ls), bindingCount(rs))})
		for _, v := range sortedKeys(versions) {
			a, b := ls[v], rs[v]
			idx := i
			if len(a) == 0 || len(b) == 0 {
				unresolved(r, Finding{ID: "COMPARE_COVERAGE_GAP", Message: fmt.Sprintf("no matched observation on one side for %s [%s]", s.Symbol, v), SelectorIndex: &idx})
				continue
			}
			aset, bset := map[string]Binding{}, map[string]Binding{}
			for _, x := range a {
				if lm[x.Provider] != "" {
					aset[lm[x.Provider]] = x
				}
			}
			for _, x := range b {
				if rm[x.Provider] != "" {
					bset[rm[x.Provider]] = x
				}
			}
			if len(aset) == 0 || len(bset) == 0 {
				continue
			}
			if !reflect.DeepEqual(sortedKeys(aset), sortedKeys(bset)) {
				// A common provider with additional observations on either side is
				// a coverage difference, not a matched provider-change witness.
				shared := false
				for id := range aset {
					if _, ok := bset[id]; ok {
						shared = true
						break
					}
				}
				if shared {
					unresolved(r, Finding{ID: "COMPARE_COVERAGE_GAP", Message: fmt.Sprintf("unmatched provider observations for %s [%s]: %v -> %v", s.Symbol, v, sortedKeys(aset), sortedKeys(bset)), SelectorIndex: &idx})
				} else {
					// Select an actually differing provider for the first witness.
					ak, bk := sortedKeys(aset)[0], sortedKeys(bset)[0]
					for _, k := range sortedKeys(aset) {
						if _, ok := bset[k]; !ok {
							ak = k
							break
						}
					}
					for _, k := range sortedKeys(bset) {
						if _, ok := aset[k]; !ok {
							bk = k
							break
						}
					}
					x, y := aset[ak], bset[bk]
					failure(r, Finding{ID: "PROVIDER_CHANGED", Message: fmt.Sprintf("%s -> %s [%s]: %v -> %v", s.Reference, s.Symbol, v, sortedKeys(aset), sortedKeys(bset)), SelectorIndex: &idx, Witness: &x, OtherWitness: &y})
				}
			}
			for _, id := range sortedKeys(aset) {
				x := aset[id]
				y, ok := bset[id]
				if !ok {
					continue
				}
				xh, yh := left.Objects[x.Provider].SHA256, right.Objects[y.Provider].SHA256
				if xh != yh && !artifactSeen[id] {
					r.ArtifactChanges = append(r.ArtifactChanges, ArtifactChange{LogicalID: id, LeftSHA256: xh, RightSHA256: yh})
					artifactSeen[id] = true
				}
			}
		}
	}
	if r.Outcome == Unresolved && r.BindingVerdict != Fail {
		r.BindingVerdict = Unresolved
	}
	return finishResult(r)
}
func bindingCount(m map[string][]Binding) int {
	n := 0
	for _, v := range m {
		n += len(v)
	}
	return n
}
func invalidCompare(r *Result, e error) *Result {
	r.Outcome = InfrastructureError
	r.BindingVerdict = Unresolved
	r.Findings = append(r.Findings, Finding{ID: "INVALID_MAPPING", Message: e.Error()})
	return finishResult(r)
}

func finishResult(r *Result) *Result {
	slices.SortStableFunc(r.Findings, func(a, b Finding) int {
		rank := func(f Finding) int {
			switch f.ID {
			case "PROVIDER_NOT_ALLOWED", "PROVIDER_CHANGED", "ARTIFACT_PIN_MISMATCH":
				return 0
			}
			if f.Witness != nil {
				return 1
			}
			return 2
		}
		return rank(a) - rank(b)
	})
	return r
}
