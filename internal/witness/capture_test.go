package witness

import (
	"strings"
	"testing"
)

func TestCaptureELFPlatformBoundary(t *testing.T) {
	amd64 := Identity{ELFClass: "ELFCLASS64", Machine: "EM_X86_64"}
	arm64 := Identity{ELFClass: "ELFCLASS64", Machine: "EM_AARCH64"}
	compat := Identity{ELFClass: "ELFCLASS32", Machine: "EM_386"}
	x32 := Identity{ELFClass: "ELFCLASS32", Machine: "EM_X86_64"}
	for _, tc := range []struct {
		name, arch, issueArtifact string
		executable, loader        Identity
	}{
		{"native amd64", "amd64", "", amd64, amd64},
		{"native arm64", "arm64", "", arm64, arm64},
		{"compat workload", "amd64", "workload executable", compat, compat},
		{"x32 workload", "amd64", "workload executable", x32, amd64},
		{"foreign workload", "amd64", "workload executable", arm64, amd64},
		{"compat loader", "amd64", "loader", amd64, compat},
		{"foreign loader", "arm64", "loader", arm64, amd64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issue := captureELFPlatformIssue(tc.arch, tc.executable, tc.loader)
			if tc.issueArtifact == "" {
				if issue != nil {
					t.Fatal(issue)
				}
				return
			}
			if issue == nil || issue.ID != "UNVALIDATED_PLATFORM" || !strings.Contains(issue.Message, tc.issueArtifact) {
				t.Fatalf("unsupported ELF got issue %+v", issue)
			}
		})
	}
}
