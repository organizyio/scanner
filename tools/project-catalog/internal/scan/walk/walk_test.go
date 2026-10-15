package walk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMissingPathWarnsAndKeepsPartialSuccess(t *testing.T) {
	warnings := 0
	err := Walk(Config{Ctx: context.Background(), Roots: []string{filepath.Join(t.TempDir(), "missing")}, OnError: func(string, error) { warnings++ }}, &Handler{})
	if err != nil || warnings != 1 {
		t.Fatalf("err=%v warnings=%d", err, warnings)
	}
}
func TestCanceledMissingPathDoesNotSucceed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Walk(Config{Ctx: ctx, Roots: []string{filepath.Join(t.TempDir(), "missing")}}, &Handler{})
	if err != context.Canceled {
		t.Fatal(err)
	}
}
func TestInaccessiblePathReportsWarning(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "locked")
	if err := os.Mkdir(p, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(p, 0700)
	if _, err := os.ReadDir(p); !os.IsPermission(err) {
		t.Skip("host does not deny permission-bit fixture; Windows ACL qualification remains separate")
	}
	warnings := 0
	err := Walk(Config{Ctx: context.Background(), Roots: []string{root}, OnError: func(string, error) { warnings++ }}, &Handler{})
	if err != nil || warnings == 0 {
		t.Fatalf("warning=%d err=%v", warnings, err)
	}
}
