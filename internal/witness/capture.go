package witness

import (
	"bytes"
	"context"
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type boundedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Len() int       { return b.buffer.Len() }
func (b *boundedBuffer) String() string { return b.buffer.String() }
func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := b.limit - b.Len()
	if n > left {
		b.truncated = true
		p = p[:left]
	}
	b.buffer.Write(p)
	return n, nil
}
func environment(overrides map[string]string) map[string]string {
	m := map[string]string{}
	for _, s := range os.Environ() {
		k, v, ok := strings.Cut(s, "=")
		if ok {
			m[k] = v
		}
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}
func envList(m map[string]string) []string {
	out := []string{}
	for _, k := range sortedKeys(m) {
		out = append(out, k+"="+m[k])
	}
	return out
}
func commandPath(argv0, cwd, pathEnv string) (string, error) {
	if strings.Contains(argv0, "/") {
		return absolute(cwd, argv0), nil
	}
	for _, dir := range filepath.SplitList(pathEnv) {
		p := absolute(cwd, filepath.Join(dir, argv0))
		s, e := os.Stat(p)
		if e == nil && s.Mode().IsRegular() && s.Mode().Perm()&0111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("command %q not found in effective PATH", argv0)
}
func loaderFor(path string) (string, error) {
	f, e := elf.Open(path)
	if e != nil {
		return "", fmt.Errorf("capture requires a dynamic ELF executable, not a shell script: %w", e)
	}
	defer f.Close()
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			if p.Filesz > 4096 {
				return "", fmt.Errorf("invalid interpreter size")
			}
			b := make([]byte, p.Filesz)
			_, e = p.ReadAt(b, 0)
			return strings.TrimRight(string(b), "\x00"), e
		}
	}
	return "", fmt.Errorf("ELF has no dynamic interpreter")
}
func glibcVersion(loader string, env map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, loader, "--version")
	cmd.Env = envList(env)
	out := &boundedBuffer{limit: 8192}
	cmd.Stdout = out
	cmd.Stderr = out
	// One bounded pipe: no concurrent writes to the buffer from stdout/stderr.
	cmd.WaitDelay = time.Second
	prepareProcess(cmd)
	err := cmd.Run()
	if cmd.Process != nil {
		finishProcess(cmd)
	}
	if err != nil {
		return "", fmt.Errorf("query interpreter: %w", err)
	}
	line := strings.SplitN(out.String(), "\n", 2)[0]
	if !strings.Contains(line, "GLIBC") && !strings.Contains(line, "GNU libc") {
		return "", fmt.Errorf("interpreter is not recognized glibc: %s", line)
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", fmt.Errorf("empty interpreter version")
	}
	return strings.TrimSuffix(fields[len(fields)-1], "."), nil
}

var testedGlibc = map[string]bool{"2.36": true, "2.41": true}

var testedCapturePlatforms = map[[3]string]bool{
	{"linux", "amd64", "2.36"}: true,
	{"linux", "amd64", "2.41"}: true,
	{"linux", "arm64", "2.36"}: true,
	{"linux", "arm64", "2.41"}: true,
}

// Native validation covers AMD64 in both supported userspaces. Validation and
// release builds explicitly use this same profile; tests also exercise disabling it.
var enableAMD64Capture = "true"

// Use the evidence's platform, including for portable offline evaluation.
func capturePlatformIssue(osName, architecture, glibc string) *Finding {
	if testedCapturePlatforms[[3]string{osName, architecture, glibc}] {
		if architecture == "amd64" && enableAMD64Capture != "true" {
			return &Finding{ID: "UNVALIDATED_PLATFORM", Message: "amd64 capture is not enabled in this build"}
		}
		return nil
	}
	if !testedGlibc[glibc] {
		return &Finding{ID: "UNTESTED_GLIBC", Message: "parser was not validated against glibc " + glibc}
	}
	return &Finding{ID: "UNVALIDATED_PLATFORM", Message: fmt.Sprintf("capture was not validated for %s/%s with glibc %s", osName, architecture, glibc)}
}

func captureELFPlatformIssue(architecture string, executable, loader Identity) *Finding {
	for _, artifact := range []struct {
		name     string
		identity Identity
	}{{"workload executable", executable}, {"loader", loader}} {
		if issue := elfArtifactPlatformIssue(architecture, artifact.name, artifact.identity); issue != nil {
			return issue
		}
	}
	return nil
}

func elfArtifactPlatformIssue(architecture, name string, identity Identity) *Finding {
	var machine string
	switch architecture {
	case "amd64":
		machine = "EM_X86_64"
	case "arm64":
		machine = "EM_AARCH64"
	default:
		// The platform boundary already rejects other architectures.
		return nil
	}
	if identity.ELFClass != "ELFCLASS64" || identity.Machine != machine {
		return &Finding{ID: "UNVALIDATED_PLATFORM", Message: fmt.Sprintf("%s has ELF class/machine %s/%s; capture requires ELFCLASS64/%s for %s", name, identity.ELFClass, identity.Machine, machine, architecture)}
	}
	return nil
}

type snapshot struct {
	identity Identity
	info     os.FileInfo
}

func Capture(c Config) (*Observation, error) {
	if e := ValidateConfig(c); e != nil {
		return nil, e
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("capture is supported only on Linux/glibc; offline check and compare are portable")
	}
	env := environment(c.Environment)
	exe, e := commandPath(c.Command[0], c.WorkingDirectory, env["PATH"])
	if e != nil {
		return nil, e
	}
	loader, e := loaderFor(exe)
	if e != nil {
		return nil, e
	}
	oldDebug, oldOutput := env["LD_DEBUG"], env["LD_DEBUG_OUTPUT"]
	delete(env, "LD_DEBUG")
	delete(env, "LD_DEBUG_OUTPUT")
	ver, e := glibcVersion(loader, env)
	if e != nil {
		return nil, e
	}
	loaderBefore, loaderInfo := inspectFile(loader, loader)
	obs := &Observation{SchemaVersion: SchemaVersion, Kind: "observation", Trace: []string{}, Bindings: []Binding{}, Objects: map[string]Identity{}, Capture: CaptureState{Issues: []Finding{}}}
	p := &obs.Provenance
	overrides := map[string]string{}
	for k, v := range c.Environment {
		overrides[k] = v
	}
	*p = Provenance{ToolVersion: Version, Backend: "glibc-ld-debug-bindings-files", OS: runtime.GOOS, Architecture: runtime.GOARCH, LoaderPath: loader, LoaderSHA256: loaderBefore.SHA256, GlibcVersion: ver, Command: append([]string{}, c.Command...), WorkingDirectory: c.WorkingDirectory, EnvironmentOverrides: overrides, LinkerEnvironment: map[string]string{}, DiagnosticsPolicy: "replace LD_DEBUG with bindings,files; unset LD_DEBUG_OUTPUT; drain bounded stderr pipe", PreviousLDDEBUG: oldDebug, PreviousLDDEBUGOUTPUT: oldOutput}
	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		p.OSRelease = string(b)
	}
	paths, _ := objectPaths(c.Roots, c.Objects)
	pre := map[string]snapshot{}
	aliases := map[string][]string{}
	for _, id := range sortedKeys(paths) {
		path := paths[id]
		ident, info := inspectFile(path, path)
		ident.LogicalID = id
		pre[id] = snapshot{ident, info}
		obs.Objects[path] = ident
		if ident.ResolvedPath != "" {
			aliases[ident.ResolvedPath] = append(aliases[ident.ResolvedPath], id)
		}
	}
	// Capture executable mutation even when it is not selected by the contract.
	exeBefore, exeInfo := inspectFile(exe, exe)
	env["LD_DEBUG"] = "bindings,files"
	for k, v := range env {
		if strings.HasPrefix(k, "LD_") || strings.HasPrefix(k, "GLIBC_") {
			p.LinkerEnvironment[k] = v
		}
	}
	duration, _ := time.ParseDuration(c.Deadline)
	ctx, cancel := captureContext(duration)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, c.Command[1:]...)
	cmd.Args[0] = c.Command[0]
	cmd.Dir = c.WorkingDirectory
	cmd.Env = envList(env)
	cmd.WaitDelay = time.Second
	prepareProcess(cmd)
	stdout := &boundedBuffer{limit: c.Limits.StdoutBytes}
	raw := &boundedBuffer{limit: c.Limits.TraceBytes + c.Limits.StderrBytes}
	cmd.Stdout = stdout
	cmd.Stderr = raw
	p.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if e = cmd.Start(); e != nil {
		return obs, fmt.Errorf("start workload: %w", e)
	}
	obs.Workload.Started = true
	p.PID = cmd.Process.Pid
	e = cmd.Wait()
	obs.Workload.Completed = true
	exit := cmd.ProcessState.ExitCode()
	obs.Workload.ExitCode = &exit
	obs.Workload.DeadlineExceeded = errors.Is(ctx.Err(), context.DeadlineExceeded)
	finishProcess(cmd)
	p.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	obs.Workload.Stdout = stdout.String()
	obs.Workload.StdoutTruncated = stdout.truncated
	trace, stderr, tt, st := SplitDiagnostics(raw.String(), c.Limits)
	obs.Trace = trace
	obs.Workload.Stderr = stderr
	obs.Workload.StderrTruncated = st || raw.truncated
	obs.Capture.TraceTruncated = tt || raw.truncated
	events, issues := ParseTrace(trace, p.PID)
	obs.Bindings = events
	obs.Capture.Issues = issues
	add := func(id, msg string) { obs.Capture.Issues = append(obs.Capture.Issues, Finding{ID: id, Message: msg}) }
	if issue := capturePlatformIssue(p.OS, p.Architecture, p.GlibcVersion); issue != nil {
		add(issue.ID, issue.Message)
	}
	if issue := captureELFPlatformIssue(p.Architecture, exeBefore, loaderBefore); issue != nil {
		add(issue.ID, issue.Message)
	}
	if obs.Capture.TraceTruncated {
		add("TRACE_TRUNCATED", "diagnostics exceeded a capture limit")
	}
	if obs.Workload.DeadlineExceeded {
		add("WORKLOAD_DEADLINE", "workload deadline exceeded")
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		add("WORKLOAD_CANCELLED", "capture cancelled by an interrupt or termination signal")
	}
	if exit < 0 {
		add("WORKLOAD_SIGNAL", "workload was terminated by a signal")
	}
	if errors.Is(e, exec.ErrWaitDelay) {
		add("STREAM_INCOMPLETE", "inherited output descriptors did not close before WaitDelay")
	}
	var exitErr *exec.ExitError
	if e != nil && !errors.As(e, &exitErr) && !errors.Is(e, exec.ErrWaitDelay) {
		return obs, fmt.Errorf("workload harness: %w", e)
	}
	observed := map[string]bool{}
	for _, b := range events {
		observed[b.Reference] = true
		observed[b.Provider] = true
	}
	for _, id := range sortedKeys(pre) {
		snap := pre[id]
		path := paths[id]
		post, info := inspectFile(path, path)
		post.LogicalID = id
		if post.Problem == "" && snap.identity.Problem == "" && post.ResolvedPath == snap.identity.ResolvedPath && post.SHA256 == snap.identity.SHA256 && sameFile(snap.info, info) {
			post.Stability = "pre_post_unchanged"
		} else {
			post.Stability = "changed"
			if post.Problem == "" {
				post.Problem = "artifact differs from pre-run identity"
			}
		}
		if len(aliases[post.ResolvedPath]) > 1 {
			post.Problem = "multiple logical IDs resolve to the same file"
			post.Stability = "ambiguous"
		}
		obs.Objects[path] = post
	}
	for _, name := range sortedKeys(observed) {
		path, err := observedFile(name, exe, c.Command[0])
		if err != nil {
			obs.Objects[name] = Identity{ObservedPath: name, Stability: "ambiguous", Problem: err.Error()}
			continue
		}
		ident, observedInfo := inspectFile(name, path)
		if ids := aliases[path]; len(ids) == 1 {
			declared := obs.Objects[paths[ids[0]]]
			if declared.Problem == "" && (ident.Problem != "" || ident.SHA256 != declared.SHA256 || ident.ResolvedPath != declared.ResolvedPath || !sameFile(pre[ids[0]].info, observedInfo)) {
				declared.Stability = "changed"
				declared.Problem = "artifact changed while resolving observed object"
			}
			ident = declared
			ident.ObservedPath = name
		} else if len(ids) > 1 {
			ident.Stability = "ambiguous"
			ident.Problem = "multiple logical IDs resolve to the same file"
		}
		obs.Objects[name] = ident
	}
	loaderAfter, li := inspectFile(loader, loader)
	exeAfter, ei := inspectFile(exe, exe)
	if loaderBefore.Problem != "" || loaderAfter.Problem != "" || loaderBefore.SHA256 != loaderAfter.SHA256 || !sameFile(loaderInfo, li) {
		add("LOADER_CHANGED", "loader identity could not be held constant")
	}
	if exeBefore.Problem != "" || exeAfter.Problem != "" || exeBefore.SHA256 != exeAfter.SHA256 || !sameFile(exeInfo, ei) {
		add("EXECUTABLE_CHANGED", "workload executable identity changed")
	}
	p.TraceSHA256 = traceHash(trace)
	obs.Capture.Complete = len(obs.Capture.Issues) == 0
	return obs, nil
}
