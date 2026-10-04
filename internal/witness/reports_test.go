package witness

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepresentativeReports(t *testing.T) {
	count := 0
	e := filepath.WalkDir("../../testdata/reports", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".config.json") {
			return nil
		}
		count++
		t.Run(path, func(t *testing.T) {
			var r Result
			if e := ReadJSON(path, &r); e != nil {
				t.Fatal(e)
			}
			if r.Kind == "check" {
				c, e := LoadConfig(strings.TrimSuffix(path, ".json") + ".config.json")
				if e != nil {
					t.Fatal(e)
				}
				actual := Evaluate(r.Observation, c)
				if actual.Outcome != r.Outcome {
					t.Fatalf("recorded=%s actual=%s", r.Outcome, actual.Outcome)
				}
			} else if r.Kind == "compare" {
				c, e := LoadCompareConfig(strings.TrimSuffix(path, ".json") + ".config.json")
				if e != nil {
					t.Fatal(e)
				}
				actual := Compare(r.Left, r.Right, c)
				if actual.Outcome != r.Outcome {
					t.Fatalf("recorded=%s actual=%s", r.Outcome, actual.Outcome)
				}
			} else {
				t.Fatal("unexpected representative report kind")
			}
		})
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if count < 10 {
		t.Fatalf("only %d real report fixtures", count)
	}
}
