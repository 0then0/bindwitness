package witness

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() Config {
	c := testConfig()
	c.Command = []string{"/scope/main"}
	c.WorkingDirectory = "/scope"
	c.Environment = map[string]string{}
	c.Deadline = "1s"
	c.Limits = Limits{StdoutBytes: 10, StderrBytes: 10, TraceBytes: 100}
	return c
}
func TestConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
	}{
		{"zero output", func(c *Config) { c.Limits.TraceBytes = 0 }},
		{"negative deadline", func(c *Config) { c.Deadline = "-1s" }},
		{"unknown provider", func(c *Config) { c.Selectors[0].Providers = []string{"missing"} }},
		{"managed output", func(c *Config) { c.Environment["LD_DEBUG_OUTPUT"] = "/tmp/user-file" }},
		{"managed debug", func(c *Config) { c.Environment["LD_DEBUG"] = "all" }},
		{"root traversal", func(c *Config) { c.Objects["a"] = ObjectSpec{Root: "scope", Path: "../a.so"} }},
		{"bad pin", func(c *Config) { c.Objects["a"] = ObjectSpec{Root: "scope", Path: "a.so", SHA256: "invalid"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(&c)
			if ValidateConfig(c) == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
func TestBoundedCopy(t *testing.T) {
	// io.Copy previously discovered a promoted bytes.Buffer.ReadFrom and bypassed
	// Write's cap. LimitReader ensures this exercises destination dispatch.
	b := &boundedBuffer{limit: 7}
	n, e := io.Copy(b, io.LimitReader(strings.NewReader(strings.Repeat("x", 10000)), 10000))
	if e != nil || n != 10000 || b.Len() != 7 || !b.truncated {
		t.Fatalf("read=%d kept=%d truncated=%v err=%v", n, b.Len(), b.truncated, e)
	}
}
func TestJSONReports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observation.json")
	o := testObservation("a", "")
	if e := WriteJSON(path, o); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadObservation(path); e != nil {
		t.Fatal(e)
	}
	r := Evaluate(o, testConfig())
	if e := WriteJSON(path, r); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadObservation(path); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte(`{"schema_version":1,"unexpected":true}`), 0600); e != nil {
		t.Fatal(e)
	}
	var c Config
	if ReadJSON(path, &c) == nil {
		t.Fatal("unknown fields accepted")
	}
}
