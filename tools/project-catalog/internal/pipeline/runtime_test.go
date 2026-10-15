package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"scan-projects/internal/scan"
	"testing"
)

func TestProjectDetectionUsesFinalLiveSettingIncludingCancellation(t *testing.T) {
	cases := []struct {
		name                   string
		initial, final, cancel bool
	}{
		{"enable", false, true, false}, {"disable", true, false, false}, {"enable partial", false, true, true}, {"disable partial", true, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"go.mod", "later.txt"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("synthetic"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			runtime := scan.NewRuntimeOptions()
			runtime.ProjectDetection.Store(tc.initial)
			runtime.HashFiles.Store(false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			opts := scan.ScanOptions{ProjectDetection: tc.initial, Runtime: runtime, OnCheckpoint: func(string) {
				runtime.ProjectDetection.Store(tc.final)
				if tc.cancel {
					cancel()
				}
			}}
			result, err := DeepScan(ctx, DeepScanInput{Roots: []string{root}, Options: opts})
			if tc.cancel {
				if err != context.Canceled {
					t.Fatalf("cancellation %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			folder := result.Folders[root]
			if folder == nil {
				t.Fatal("missing folder")
			}
			if folder.IsProjectRoot != tc.final {
				t.Fatalf("initial=%v final=%v project=%v", tc.initial, tc.final, folder.IsProjectRoot)
			}
			if tc.final && folder.ProjectKind != "go" {
				t.Fatalf("detected kind %q", folder.ProjectKind)
			}
			if !tc.final && folder.ProjectKind != "" {
				t.Fatalf("disabled project kind %q", folder.ProjectKind)
			}
		})
	}
}
