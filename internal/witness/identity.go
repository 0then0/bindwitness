package witness

import (
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Bare loader names are deliberately not searched by basename or SONAME.
func observedFile(path, commandPath, argv0 string) (string, error) {
	// The executable was resolved before launch. A DSO's relative name may refer
	// to any later working directory; LD_DEBUG does not establish that directory.
	if path == argv0 && commandPath != "" {
		return filepath.EvalSymlinks(commandPath)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("loader name has no unambiguous absolute file path")
	}
	return filepath.EvalSymlinks(path)
}
func inspectFile(observed, path string) (Identity, os.FileInfo) {
	id := Identity{ObservedPath: observed, Stability: "post_only"}
	resolved, e := filepath.EvalSymlinks(path)
	if e != nil {
		id.Problem = e.Error()
		return id, nil
	}
	id.ResolvedPath = resolved
	f, e := os.Open(resolved)
	if e != nil {
		id.Problem = e.Error()
		return id, nil
	}
	defer f.Close()
	before, e := f.Stat()
	if e != nil || !before.Mode().IsRegular() {
		id.Problem = "artifact must be a regular file"
		return id, nil
	}
	// Hash the very same open descriptor used to inspect the ELF headers.
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		id.Problem = e.Error()
		return id, nil
	}
	id.SHA256 = hex.EncodeToString(h.Sum(nil))
	ef, e := elf.NewFile(f)
	if e != nil {
		id.Problem = e.Error()
		return id, nil
	}
	id.ELFClass = ef.Class.String()
	id.Machine = ef.Machine.String()
	if names, err := ef.DynString(elf.DT_SONAME); err == nil && len(names) == 1 {
		id.SONAME = names[0]
	}
	if s := ef.Section(".note.gnu.build-id"); s != nil {
		b, err := s.Data()
		if err == nil {
			id.BuildID = buildID(b, ef)
		}
	}
	after, e := f.Stat()
	now, e2 := os.Stat(resolved)
	if e != nil || e2 != nil || !sameFile(before, after) || !sameFile(before, now) {
		id.Problem = "artifact changed during identity read"
		id.Stability = "changed"
	}
	return id, before
}
func buildID(b []byte, f *elf.File) string {
	for len(b) >= 12 {
		n, d, t := uint64(f.ByteOrder.Uint32(b)), uint64(f.ByteOrder.Uint32(b[4:])), f.ByteOrder.Uint32(b[8:])
		np := (n + 3) &^ 3
		dp := (d + 3) &^ 3
		if 12+np+dp > uint64(len(b)) {
			return ""
		}
		if t == 3 && string(b[12:12+n]) == "GNU\x00" {
			return hex.EncodeToString(b[12+np : 12+np+d])
		}
		b = b[12+np+dp:]
	}
	return ""
}
func sameFile(a, b os.FileInfo) bool {
	return a != nil && b != nil && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime() == b.ModTime() && a.Mode() == b.Mode()
}
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
