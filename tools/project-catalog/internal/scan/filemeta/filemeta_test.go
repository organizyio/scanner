package filemeta

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFallbackDoesNotInventBirthTime(t *testing.T) {
	p := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(p, nil, 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	got := fallbackExtra("file", info)
	if got.CreatedAt != "" || got.ModifiedAt == "" {
		t.Fatalf("metadata %+v", got)
	}
}
