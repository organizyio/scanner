package scan

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Capture metadata first and remove the file before hashing. This deterministic
// open failure exercises the same unconditional error path as ACL/sharing denial
// without pretending to qualify platform-specific permission enforcement.
func TestHashOpenFailureReportsPartialResultWarning(t *testing.T) {
	for _, useCallback := range []bool{false, true} {
		name := "stderr"
		if useCallback {
			name = "callback"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "vanished.txt")
			if err := os.WriteFile(path, []byte("synthetic"), 0600); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			info, err := entries[0].Info()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			warnings := 0
			opts := ScanOptions{HashFiles: true}
			var diagnostic string
			if useCallback {
				opts.OnWarning = func(p string, e error) {
					if p != path || e == nil {
						t.Errorf("warning %s %v", p, e)
					}
					warnings++
				}
			}
			original := os.Stderr
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stderr = writer
			defer func() { os.Stderr = original }()
			rec, handled := ingestFile(path, entries[0], info, map[string]*FolderRecord{}, opts)
			writer.Close()
			data, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			diagnostic = string(data)
			if !handled || rec.HashStatus != "error" || rec.Sha256 != "" {
				t.Fatalf("partial record %+v handled=%v", rec, handled)
			}
			if useCallback {
				if warnings != 1 {
					t.Fatalf("warnings=%d", warnings)
				}
			} else if !strings.Contains(diagnostic, "[WARN] Partial scan:") || !strings.Contains(diagnostic, "vanished.txt") {
				t.Fatalf("missing stderr diagnostic: %q", diagnostic)
			}
		})
	}
}
