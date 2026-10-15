package analysis

import (
	"scan-projects/internal/scan"
	"testing"
)

func TestProjectKindAlphabeticalStackNotMarker(t *testing.T) {
	fr := &scan.FolderRecord{MarkerFiles: map[string]bool{"Cargo.toml": true, "go.mod": true, "build.gradle": true}}
	inferProjectKind(fr)
	if fr.ProjectKind != "go" {
		t.Fatalf("kind %s", fr.ProjectKind)
	}
}
