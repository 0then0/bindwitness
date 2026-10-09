// Package witness observes glibc diagnostics and evaluates exact binding contracts.
package witness

const Version = "0.1.2"
const SchemaVersion = 1

type Outcome string

const (
	Pass                Outcome = "PASS"
	Fail                Outcome = "FAIL"
	Unresolved          Outcome = "UNRESOLVED"
	InfrastructureError Outcome = "INFRASTRUCTURE_ERROR"
)

func ExitCode(o Outcome) int {
	switch o {
	case Pass:
		return 0
	case Fail:
		return 1
	case Unresolved:
		return 2
	default:
		return 3
	}
}

type Limits struct {
	StdoutBytes int `json:"stdout_bytes"`
	StderrBytes int `json:"stderr_bytes"`
	TraceBytes  int `json:"trace_bytes"`
}
type ObjectSpec struct {
	Root   string `json:"root"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}
type Selector struct {
	Reference string `json:"reference"`
	Symbol    string `json:"symbol"`
	// nil means any trace version; pointer to empty string means unversioned only.
	TraceVersion *string  `json:"trace_version,omitempty"`
	Providers    []string `json:"providers"`
	Required     bool     `json:"required"`
}
type Config struct {
	SchemaVersion    int                   `json:"schema_version"`
	Command          []string              `json:"command"`
	WorkingDirectory string                `json:"working_directory"`
	Environment      map[string]string     `json:"environment"`
	Deadline         string                `json:"deadline"`
	Limits           Limits                `json:"limits"`
	Roots            map[string]string     `json:"roots"`
	Objects          map[string]ObjectSpec `json:"objects"`
	Selectors        []Selector            `json:"selectors"`
}
type Identity struct {
	ObservedPath string `json:"observed_path"`
	ResolvedPath string `json:"resolved_path,omitempty"`
	LogicalID    string `json:"logical_id,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	ELFClass     string `json:"elf_class,omitempty"`
	Machine      string `json:"machine,omitempty"`
	SONAME       string `json:"soname,omitempty"`
	BuildID      string `json:"build_id,omitempty"`
	Stability    string `json:"stability"`
	Problem      string `json:"problem,omitempty"`
}
type Binding struct {
	PID                int    `json:"pid"`
	Reference          string `json:"reference"`
	Provider           string `json:"provider"`
	Symbol             string `json:"symbol"`
	TraceVersion       string `json:"trace_version"`
	ReferenceNamespace int    `json:"reference_namespace"`
	ProviderNamespace  int    `json:"provider_namespace"`
	TraceLine          int    `json:"trace_line"`
}
type Finding struct {
	ID                string   `json:"id"`
	Message           string   `json:"message"`
	SelectorIndex     *int     `json:"selector_index,omitempty"`
	Witness           *Binding `json:"witness,omitempty"`
	OtherWitness      *Binding `json:"other_witness,omitempty"`
	ExpectedProviders []string `json:"expected_providers,omitempty"`
}
type Workload struct {
	Started          bool   `json:"started"`
	Completed        bool   `json:"completed"`
	ExitCode         *int   `json:"exit_code,omitempty"`
	DeadlineExceeded bool   `json:"deadline_exceeded"`
	Stdout           string `json:"stdout"`
	Stderr           string `json:"stderr"`
	StdoutTruncated  bool   `json:"stdout_truncated"`
	StderrTruncated  bool   `json:"stderr_truncated"`
}
type CaptureState struct {
	Complete       bool      `json:"complete"`
	TraceTruncated bool      `json:"trace_truncated"`
	Issues         []Finding `json:"issues"`
}
type Provenance struct {
	ToolVersion           string            `json:"tool_version"`
	Backend               string            `json:"backend"`
	OS                    string            `json:"os"`
	Architecture          string            `json:"architecture"`
	OSRelease             string            `json:"os_release"`
	LoaderPath            string            `json:"loader_path"`
	LoaderSHA256          string            `json:"loader_sha256"`
	GlibcVersion          string            `json:"glibc_version"`
	StartedAt             string            `json:"started_at"`
	FinishedAt            string            `json:"finished_at"`
	PID                   int               `json:"pid"`
	Command               []string          `json:"command"`
	WorkingDirectory      string            `json:"working_directory"`
	EnvironmentOverrides  map[string]string `json:"environment_overrides"`
	LinkerEnvironment     map[string]string `json:"linker_environment"`
	DiagnosticsPolicy     string            `json:"diagnostics_policy"`
	PreviousLDDEBUG       string            `json:"previous_ld_debug,omitempty"`
	PreviousLDDEBUGOUTPUT string            `json:"previous_ld_debug_output,omitempty"`
	TraceSHA256           string            `json:"trace_sha256"`
}
type Observation struct {
	SchemaVersion int                 `json:"schema_version"`
	Kind          string              `json:"kind"`
	Provenance    Provenance          `json:"provenance"`
	Workload      Workload            `json:"workload"`
	Capture       CaptureState        `json:"capture"`
	Trace         []string            `json:"trace"`
	Bindings      []Binding           `json:"bindings"`
	Objects       map[string]Identity `json:"objects"`
}
type Coverage struct {
	SelectorIndex int  `json:"selector_index"`
	Required      bool `json:"required"`
	Observed      int  `json:"observed"`
}
type Result struct {
	SchemaVersion   int              `json:"schema_version"`
	Kind            string           `json:"kind"`
	Outcome         Outcome          `json:"outcome"`
	BindingVerdict  Outcome          `json:"binding_verdict"`
	Findings        []Finding        `json:"findings"`
	Coverage        []Coverage       `json:"coverage"`
	Observation     *Observation     `json:"observation,omitempty"`
	Left            *Observation     `json:"left,omitempty"`
	Right           *Observation     `json:"right,omitempty"`
	ArtifactChanges []ArtifactChange `json:"artifact_changes,omitempty"`
}
type ArtifactChange struct {
	LogicalID   string `json:"logical_id"`
	LeftSHA256  string `json:"left_sha256"`
	RightSHA256 string `json:"right_sha256"`
}
type CompareConfig struct {
	SchemaVersion int                   `json:"schema_version"`
	LeftRoots     map[string]string     `json:"left_roots"`
	RightRoots    map[string]string     `json:"right_roots"`
	Objects       map[string]ObjectSpec `json:"objects"`
	Selectors     []Selector            `json:"selectors"`
}
