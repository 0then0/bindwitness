package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"bindwitness/internal/witness"
)

func TestCLIInfrastructure(t *testing.T) {
	report := filepath.Join(t.TempDir(), "error.json")
	var out, errout bytes.Buffer
	if code := run([]string{"check", "--config", "missing.json", "--report", report}, &out, &errout); code != 3 {
		t.Fatal(code)
	}
	b, e := os.ReadFile(report)
	if e != nil {
		t.Fatal(e)
	}
	var r witness.Result
	if e = json.Unmarshal(b, &r); e != nil || r.Outcome != witness.InfrastructureError || r.Kind != "error" {
		t.Fatal(r, e)
	}
}
func TestReportWriteFailure(t *testing.T) {
	var out, errout bytes.Buffer
	report := filepath.Join(t.TempDir(), "missing-dir", "report.json")
	if code := run([]string{"check", "--config", "missing.json", "--report", report}, &out, &errout); code != 3 {
		t.Fatal(code)
	}
	if !bytes.Contains(errout.Bytes(), []byte("save report")) {
		t.Fatal(errout.String())
	}
}
func TestCLIVersion(t *testing.T) {
	var out, errout bytes.Buffer
	if code := run([]string{"--version"}, &out, &errout); code != 0 || out.String() != "bindwitness 0.1.0\n" {
		t.Fatal(code, out.String())
	}
}
