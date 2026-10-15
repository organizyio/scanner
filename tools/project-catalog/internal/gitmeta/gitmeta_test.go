package gitmeta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"scan-projects/internal/model"
	"strings"
	"testing"
	"time"
)

func gitFixture(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git: %v %s", err, out)
	}
}
func TestGitdirFilesAndSanitizedMetadata(t *testing.T) {
	root := t.TempDir()
	gitFixture(t, root, "init", "-q")
	gitFixture(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "fixture")
	worktree := filepath.Join(t.TempDir(), "linked")
	gitFixture(t, root, "worktree", "add", "-qb", "linked", worktree)
	if !IsRepository(context.Background(), worktree) {
		t.Fatal("linked worktree not detected")
	}
	sub := filepath.Join(t.TempDir(), "submodule")
	dir := filepath.Join(t.TempDir(), "module.git")
	gitFixture(t, root, "init", "-q", "--separate-git-dir", dir, sub)
	if !IsRepository(context.Background(), sub) {
		t.Fatal("separate gitdir not detected")
	}
	bad := t.TempDir()
	if err := os.WriteFile(filepath.Join(bad, ".git"), []byte("gitdir: missing"), 0600); err != nil {
		t.Fatal(err)
	}
	if IsRepository(context.Background(), bad) {
		t.Fatal("invalid gitdir accepted")
	}
	gitFixture(t, root, "remote", "add", "origin", "https://user:FAKE_SECRET@example.invalid/team/repo.git?token=FAKE_QUERY#FAKE_FRAGMENT")
	var info model.ProjectInfo
	var logger *nilLogger
	Enrich(root, &info, logger)
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"FAKE_SECRET", "FAKE_QUERY", "FAKE_FRAGMENT", "user:"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("secret retained: %s", secret)
		}
	}
	if info.Host == nil || *info.Host != "example.invalid" || info.RepoSlug == nil || *info.RepoSlug != "repo" {
		t.Fatalf("metadata %+v", info)
	}
}

type nilLogger struct{}

func (l *nilLogger) Debugf(string, ...interface{}) {
	if l == nil {
		panic("typed nil called")
	}
}
func TestSanitizeRemoteURL(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://user:FAKE_PASSWORD@example.invalid/repo.git?FAKE_TOKEN#fragment", "https://example.invalid/repo.git"},
		{"ssh://git:FAKE_PASSWORD@example.invalid/repo.git?FAKE_TOKEN", "ssh://git@example.invalid/repo.git"},
		{"git@example.invalid:team/repo.git", "git@example.invalid:team/repo.git"},
		{"file:///synthetic/repo", "file:///synthetic/repo"},
		{`C:\synthetic\repo`, `C:\synthetic\repo`},
		{"example.com:owner/repo", "example.com:owner/repo"},
		{"example.com:owner/repo?FAKE_TOKEN#fragment", "example.com:owner/repo"},
		{"https://user:FAKE_PASSWORD@%invalid/repo", "[redacted remote]"},
		{"user:FAKE_PASSWORD@example.invalid/repo", "[redacted remote]"},
	} {
		if got := SanitizeRemoteURL(tc.in); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
func TestGitCancellationAndFailureUnknownDirty(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := runGit(ctx, t.TempDir(), "status")
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("cancel: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	var info model.ProjectInfo
	Enrich(t.TempDir(), &info, nil)
	if info.Dirty != nil {
		t.Fatal("failed inspection reported clean")
	}
}

func TestMain(m *testing.M) {
	if os.Getenv("SCANNER_FAKE_GIT") == "fail" {
		fmt.Fprintln(os.Stderr, "https://user:FAKE_PRIVATE_PASSWORD@example.invalid?FAKE_PRIVATE_QUERY")
		os.Exit(1)
	}
	if os.Getenv("SCANNER_FAKE_GIT") == "sleep" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestGitTimeoutAndMidCommandCancellation(t *testing.T) {
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(output, input); err != nil {
		t.Fatal(err)
	}
	if err = output.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("SCANNER_FAKE_GIT", "sleep")
	start := time.Now()
	_, err = runGit(context.Background(), dir, "status")
	elapsed := time.Since(start)
	if err != context.DeadlineExceeded || elapsed < 9*time.Second || elapsed > 15*time.Second {
		t.Fatalf("timeout=%v elapsed=%v", err, elapsed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start = time.Now()
	_, err = runGit(ctx, dir, "status")
	if err != context.DeadlineExceeded || time.Since(start) > 3*time.Second {
		t.Fatalf("cancel=%v", err)
	}
}

type recordingLogger struct{ text strings.Builder }

func (l *recordingLogger) Debugf(format string, args ...interface{}) {
	fmt.Fprintf(&l.text, format, args...)
}
func TestGitFailureDiagnosticsDoNotEchoCredentialOutput(t *testing.T) {
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := "git"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), data, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("SCANNER_FAKE_GIT", "fail")
	original := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	defer func() { os.Stderr = original }()
	logger := &recordingLogger{}
	var info model.ProjectInfo
	Enrich(dir, &info, logger)
	writer.Close()
	out, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(info)
	all := string(out) + logger.text.String() + string(payload)
	if strings.Contains(all, "FAKE_PRIVATE_") {
		t.Fatal("credential escaped through diagnostics")
	}
	if info.Dirty != nil {
		t.Fatal("failed status reported clean")
	}
}
