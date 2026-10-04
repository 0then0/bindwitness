package witness

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxJSONBytes = 256 << 20

func ReadJSON(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxJSONBytes+1))
	if err != nil {
		return err
	}
	if len(b) > MaxJSONBytes {
		return fmt.Errorf("JSON exceeds %d bytes", MaxJSONBytes)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("expected a single JSON value")
	}
	return nil
}
func WriteJSON(path string, v any) error {
	// Atomic replacement prevents a half-written report from looking complete.
	f, err := os.CreateTemp(filepath.Dir(path), ".bindwitness-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	err = enc.Encode(v)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
func absolute(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
}
func LoadConfig(path string) (Config, error) {
	var c Config
	if err := ReadJSON(path, &c); err != nil {
		return c, err
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return c, err
	}
	c.WorkingDirectory = absolute(base, c.WorkingDirectory)
	for k, v := range c.Roots {
		c.Roots[k] = absolute(base, v)
	}
	return c, ValidateConfig(c)
}
func validHash(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && s == strings.ToLower(s)
}
func objectPaths(roots map[string]string, objects map[string]ObjectSpec) (map[string]string, error) {
	out := map[string]string{}
	seen := map[string]string{}
	for id, s := range objects {
		if id == "" || s.Path == "" {
			return nil, fmt.Errorf("object ID and path must be nonempty")
		}
		root, ok := roots[s.Root]
		if !ok || !filepath.IsAbs(root) {
			return nil, fmt.Errorf("object %s needs an absolute root %q", id, s.Root)
		}
		if filepath.IsAbs(s.Path) || s.Path == ".." || strings.HasPrefix(filepath.Clean(s.Path), ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("object %s path must stay within its root", id)
		}
		if s.SHA256 != "" && !validHash(s.SHA256) {
			return nil, fmt.Errorf("object %s has invalid sha256", id)
		}
		p := filepath.Join(root, s.Path)
		if old, ok := seen[p]; ok {
			return nil, fmt.Errorf("objects %s and %s have the same path", old, id)
		}
		seen[p] = id
		out[id] = p
	}
	return out, nil
}
func validateSelectors(objects map[string]ObjectSpec, selectors []Selector) error {
	if len(selectors) == 0 {
		return fmt.Errorf("at least one exact selector is required")
	}
	seen := map[string]bool{}
	for _, s := range selectors {
		if _, ok := objects[s.Reference]; !ok {
			return fmt.Errorf("unknown reference %q", s.Reference)
		}
		if s.Symbol == "" || strings.ContainsAny(s.Symbol, "\n\x00") {
			return fmt.Errorf("symbol must be nonempty and exact")
		}
		if len(s.Providers) == 0 {
			return fmt.Errorf("selector %s needs providers", s.Symbol)
		}
		for _, p := range s.Providers {
			if _, ok := objects[p]; !ok {
				return fmt.Errorf("unknown provider %q", p)
			}
		}
		v := "*"
		if s.TraceVersion != nil {
			v = "=" + *s.TraceVersion
		}
		key := s.Reference + "\x00" + s.Symbol + "\x00" + v
		if seen[key] {
			return fmt.Errorf("duplicate selector %s", s.Symbol)
		}
		seen[key] = true
	}
	return nil
}
func ValidateConfig(c Config) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported config schema_version %d", c.SchemaVersion)
	}
	if len(c.Command) == 0 || c.Command[0] == "" {
		return fmt.Errorf("command must be an argv array")
	}
	for _, a := range c.Command {
		if strings.ContainsRune(a, 0) {
			return fmt.Errorf("command contains NUL")
		}
	}
	if !filepath.IsAbs(c.WorkingDirectory) {
		return fmt.Errorf("working_directory must be absolute")
	}
	d, e := time.ParseDuration(c.Deadline)
	if e != nil || d <= 0 {
		return fmt.Errorf("deadline must be a positive Go duration")
	}
	for _, n := range []int{c.Limits.StdoutBytes, c.Limits.StderrBytes, c.Limits.TraceBytes} {
		if n <= 0 || n > 32<<20 {
			return fmt.Errorf("each output limit must be between 1 and 33554432 bytes")
		}
	}
	for k, v := range c.Environment {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return fmt.Errorf("invalid environment override")
		}
		if k == "LD_DEBUG" || k == "LD_DEBUG_OUTPUT" {
			return fmt.Errorf("%s is managed by the capture backend", k)
		}
	}
	if _, e = objectPaths(c.Roots, c.Objects); e != nil {
		return e
	}
	return validateSelectors(c.Objects, c.Selectors)
}
func LoadCompareConfig(path string) (CompareConfig, error) {
	var c CompareConfig
	if e := ReadJSON(path, &c); e != nil {
		return c, e
	}
	base, e := filepath.Abs(filepath.Dir(path))
	if e != nil {
		return c, e
	}
	// Compare roots name historical capture paths; relative roots are relative to this file.
	for k, v := range c.LeftRoots {
		c.LeftRoots[k] = absolute(base, v)
	}
	for k, v := range c.RightRoots {
		c.RightRoots[k] = absolute(base, v)
	}
	if c.SchemaVersion != SchemaVersion {
		return c, fmt.Errorf("unsupported comparison schema")
	}
	if _, e = objectPaths(c.LeftRoots, c.Objects); e != nil {
		return c, e
	}
	if _, e = objectPaths(c.RightRoots, c.Objects); e != nil {
		return c, e
	}
	return c, validateSelectors(c.Objects, c.Selectors)
}
