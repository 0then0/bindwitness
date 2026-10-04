package witness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestObservedFileRequiresAbsoluteDSOPath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "object.so")
	if err := os.WriteFile(file, []byte("path resolution fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"object.so", "./object.so", "nested/../object.so"} {
		if path, err := observedFile(name, file, "./main"); err == nil || path != "" {
			t.Fatalf("relative DSO %q attributed to %q: %v", name, path, err)
		}
	}
	for _, name := range []string{file, "main", "./main"} {
		path, err := observedFile(name, file, name)
		if err != nil || path != resolved {
			t.Fatalf("known executable/absolute path %q: %q %v", name, path, err)
		}
	}
}
