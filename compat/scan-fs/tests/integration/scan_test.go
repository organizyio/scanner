//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func engineBinaryName() string {
	switch {
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return "scan-linux-amd64"
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return ""
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return "scan-darwin-arm64"
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return "scan-windows-amd64.exe"
	default:
		return ""
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestScanFS_JSON(t *testing.T) {
	root := moduleRoot(t)
	eng := os.Getenv("SCAN_ENGINE_PATH")
	if eng == "" {
		name := engineBinaryName()
		if name == "" {
			t.Fatal("unsupported integration host; set SCAN_ENGINE_PATH or use a release target")
		}
		eng = filepath.Join(root, "internal", "engine", "bin", name)
	}
	if _, err := os.Stat(eng); err != nil {
		t.Fatalf("required engine missing (%v); run scripts/build-components.py wrapper", err)
	}

	ctx := context.Background()
	testdata := filepath.Join(root, "tests", "integration", "testdata")
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/scan-fs",
		"--root", testdata,
		"--format", "json",
	)
	cmd.Dir = root
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("scan-fs: %v", err)
	}
	lines := nonemptyLines(stdout.String())
	if len(lines) < 1 {
		t.Fatalf("expected NDJSON lines, got %q", stdout.String())
	}

	var row struct {
		Path      string `json:"path"`
		Source    string `json:"source"`
		Device    string `json:"device"`
		Directory string `json:"directory"`
		Name      string `json:"name"`
		Extension string `json:"extension"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
		t.Fatal(err)
	}
	if row.Path == "" || row.Source == "" || row.Device == "" {
		t.Fatalf("item %+v", row)
	}
	if !strings.Contains(row.Path, "hello.txt") {
		t.Fatalf("expected hello.txt in path, got %q", row.Path)
	}
	if row.Name != "hello.txt" {
		t.Fatalf("name %q, want hello.txt", row.Name)
	}
	if row.Extension != ".txt" {
		t.Fatalf("extension %q, want .txt", row.Extension)
	}
	if row.Directory != filepath.Dir(row.Path) {
		t.Fatalf("directory %q, want %q", row.Directory, filepath.Dir(row.Path))
	}
	if row.Name != filepath.Base(row.Path) || row.Extension != filepath.Ext(row.Path) {
		t.Fatalf("derived fields mismatch path %q: %+v", row.Path, row)
	}
}

func nonemptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
