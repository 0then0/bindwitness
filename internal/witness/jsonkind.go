package witness

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func readKind(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, MaxJSONBytes+1))
	if e != nil {
		return "", e
	}
	if len(b) > MaxJSONBytes {
		return "", fmt.Errorf("report too large")
	}
	var h struct {
		Kind string `json:"kind"`
	}
	e = json.Unmarshal(b, &h)
	return h.Kind, e
}
