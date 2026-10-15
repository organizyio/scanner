package pipeline

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"scan-projects/internal/catalog"
	"scan-projects/internal/gitmeta"
	"scan-projects/internal/model"
	"scan-projects/internal/scan"
	"scan-projects/internal/scan/checkpoint"
	"scan-projects/internal/storage"
)

func put(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogDiscoveryCharacterization(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"git/.git/HEAD", "git/nested/.git/HEAD", "node/package.json", "node/child/package.json", "excluded/package.json", "worktree/.git", "unicode-Ա/package.json"} {
		put(t, filepath.Join(root, p), "")
	}
	got := catalog.FindProjectsWithProgress(root, map[string]struct{}{"excluded": {}, ".git": {}}, nil)
	sort.Strings(got)
	want := []string{filepath.Join(root, "git"), filepath.Join(root, "git/nested"), filepath.Join(root, "node"), filepath.Join(root, "unicode-Ա")}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// A .git file alone is intentionally not recognized by the legacy catalog.
}

func TestGitDiscoveryDoesNotChangeRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	c := exec.Command("git", "init", "-q", root)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("init: %s %v", out, err)
	}
	put(t, filepath.Join(root, "hello.txt"), "hello")
	command := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		b, e := c.Output()
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	before := command("status", "--porcelain=v1")
	config, err := os.ReadFile(filepath.Join(root, ".git/config"))
	if err != nil {
		t.Fatal(err)
	}
	info := catalog.CollectProjectInfo(root, catalog.NewLogger(false))
	if !info.IsGit || info.Dirty == nil || !*info.Dirty {
		t.Fatalf("metadata: %+v", info)
	}
	after := command("status", "--porcelain=v1")
	afterConfig, _ := os.ReadFile(filepath.Join(root, ".git/config"))
	if before != after || string(config) != string(afterConfig) {
		t.Fatal("discovery mutated Git state")
	}
}

func TestDeepScanHashExclusionsCheckpointAndResume(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	put(t, a, "abc")
	put(t, b, "def")
	put(t, filepath.Join(root, "skip/x.txt"), "hidden")
	cp := filepath.Join(t.TempDir(), "cursor.json")
	opts := scan.ScanOptions{HashFiles: true, ProjectDetection: true, OnCheckpoint: func(p string) {
		if err := checkpoint.Save(cp, p); err != nil {
			t.Error(err)
		}
	}}
	res, err := DeepScan(context.Background(), DeepScanInput{Roots: []string{root}, ExcludeDirs: []string{"skip"}, Options: opts})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 2 || res.Folders[root].FileCount != 2 || res.Folders[root].TotalSizeBytes != 6 {
		t.Fatalf("result: %+v", res)
	}
	if res.Files[0].Sha256 != fmt.Sprintf("%x", sha256.Sum256([]byte("abc"))) {
		t.Fatal("hash changed")
	}
	cursor, ok := checkpoint.Load(cp)
	if !ok || cursor.LastPath != b {
		t.Fatalf("checkpoint %+v", cursor)
	}
	resumed, err := DeepScan(context.Background(), DeepScanInput{Roots: []string{root}, ExcludeDirs: []string{"skip"}, ResumeFrom: b, Options: opts})
	if err != nil || len(resumed.Files) != 1 || resumed.Files[0].Path != b {
		t.Fatalf("resume %+v %v", resumed, err)
	}
}

func TestPauseResumeAndCancel(t *testing.T) {
	root := t.TempDir()
	put(t, filepath.Join(root, "x"), "x")
	pause := make(chan bool, 2)
	pause <- true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := DeepScan(ctx, DeepScanInput{Roots: []string{root}, Options: scan.ScanOptions{Pause: pause}})
		done <- err
	}()
	select {
	case e := <-done:
		t.Fatalf("did not pause: %v", e)
	case <-time.After(50 * time.Millisecond):
	}
	pause <- false
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal("did not resume")
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	_, err := DeepScan(canceled, DeepScanInput{Roots: []string{root}})
	if err != context.Canceled {
		t.Fatalf("cancel %v", err)
	}
}

func TestProfilesKeepConfigurationIdentity(t *testing.T) {
	cfg := storage.DefaultConfig("checkpoint.json")
	cfg.ActiveProfile = "Custom"
	cfg.Profiles["Custom"] = storage.ScanProfile{Name: "Custom", ExcludeDirs: []string{}, ScanRoots: []storage.ScanRoot{{Path: "/example", Enabled: true}}, HashFiles: true, CheckpointEnabled: true, CheckpointPath: "custom.json"}
	path := filepath.Join(t.TempDir(), "profile.yaml")
	if err := storage.SaveYAML(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := storage.LoadYAML[storage.AppConfig](path)
	if err != nil || !reflect.DeepEqual(cfg, got) {
		t.Fatalf("profile %v %+v", err, got)
	}
	base, err := os.UserConfigDir()
	if err == nil && storage.SettingsPath() != filepath.Join(base, "ScannerApp", "settings.yaml") {
		t.Fatal("settings location changed")
	}
}

// Existing failure semantics are recorded, not corrected by relocation.
func TestMissingGitRetainsLegacyDirtyFalse(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var info model.ProjectInfo
	gitmeta.Enrich(t.TempDir(), &info, nil)
	if !info.IsGit || info.Dirty == nil || *info.Dirty {
		t.Fatalf("legacy failure result: %+v", info)
	}
}

func TestInaccessibleOrMissingRootRetainsPartialSuccess(t *testing.T) {
	result, err := DeepScan(context.Background(), DeepScanInput{Roots: []string{filepath.Join(t.TempDir(), "missing")}})
	if err != nil || len(result.Files) != 0 {
		t.Fatalf("legacy missing-root behavior: %+v %v", result, err)
	}
}
