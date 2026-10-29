package catalog

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExplicitProjectRootDoesNotInheritAncestorMarker(t *testing.T) {
	for _, marker := range []string{"package.json", "unrelated.py"} {
		t.Run(marker, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "selected")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(parent, marker), []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module synthetic\n"), 0600); err != nil {
				t.Fatal(err)
			}
			got := FindProjectsContext(context.Background(), root, nil, nil)
			if len(got) != 1 || got[0] != root {
				t.Fatalf("explicit root suppressed by ancestor marker: %v", got)
			}
		})
	}
}
