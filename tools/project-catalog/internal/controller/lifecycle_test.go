//go:build cgo

package controller

import (
	"context"
	"os"
	"path/filepath"
	"scan-projects/internal/scan"
	"scan-projects/internal/scan/checkpoint"
	"scan-projects/internal/storage"
	"testing"
	"time"
)

func TestLatestPauseIntentReplacesQueuedPause(t *testing.T) {
	channel := make(chan bool, 1)
	publishPause(channel, true)
	publishPause(channel, false)
	select {
	case state := <-channel:
		if state {
			t.Fatal("Resume lost behind queued Pause")
		}
	default:
		t.Fatal("latest intent missing")
	}
}

func TestCancelWaitsForTerminalWorkerState(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctrl := &Controller{ctx: ctx, cancel: cancel, running: true, paused: true, progress: scan.NewProgress()}
	ctrl.Cancel()
	if ctx.Err() != context.Canceled || ctrl.Progress().State() != "canceling" || ctrl.IsPaused() {
		t.Fatal("cancellation request did not enter canceling state")
	}
	if !ctrl.IsRunning() {
		t.Fatal("requested cancellation falsely reported worker stopped")
	}
	ctrl.Resume()
	if ctrl.Progress().State() != "canceling" {
		t.Fatal("Resume overwrote cancellation")
	}
}

func TestFreshSettingsPersistAndDoNotShareRootSlices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "ScannerApp", "settings.yaml")
	profiles := newProfiles(path)
	if profiles.LastError() != nil {
		t.Fatal(profiles.LastError())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	p := profiles.Active()
	p.ScanRoots = []storage.ScanRoot{{Path: t.TempDir(), Enabled: true}}
	if err := profiles.UpdateActive(p); err != nil {
		t.Fatal(err)
	}
	copy := profiles.Active()
	copy.ScanRoots[0].Path = "not persisted"
	if profiles.Active().ScanRoots[0].Path == "not persisted" {
		t.Fatal("profile getter leaked mutable roots")
	}
	loaded := newProfiles(path)
	if loaded.Active().ScanRoots[0].Path != p.ScanRoots[0].Path {
		t.Fatal("settings changed across restart")
	}
}

func TestSettingsSaveFailureIsVisible(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	profiles := newProfiles(filepath.Join(file, "settings.yaml"))
	if profiles.LastError() == nil {
		t.Fatal("fresh settings save failure hidden")
	}
}

func TestTerminalCheckpointFlushUsesLastProcessedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	ctrl := &Controller{lastSeen: "synthetic/last.txt"}
	profile := storage.ScanProfile{CheckpointEnabled: true, CheckpointPath: path}
	ctrl.flushCheckpoint(profile)
	cp, ok := checkpoint.Load(path)
	if !ok || cp.LastPath != "synthetic/last.txt" {
		t.Fatalf("checkpoint %+v loaded=%v", cp, ok)
	}
	root := t.TempDir()
	file := filepath.Join(root, "not-directory")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	profile.CheckpointPath = filepath.Join(file, "checkpoint.json")
	ctrl.flushCheckpoint(profile)
	if ctrl.LastError() == "" {
		t.Fatal("checkpoint save failure hidden")
	}
}

func TestStartCompletionAndRestartResumeAreTruthful(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	settings := filepath.Join(t.TempDir(), "ScannerApp", "settings.yaml")
	profiles := newProfiles(settings)
	p := profiles.Active()
	p.ScanRoots = []storage.ScanRoot{{Path: root, Enabled: true}}
	p.HashFiles = true
	p.CheckpointEnabled = true
	p.CheckpointPath = filepath.Join(t.TempDir(), "checkpoint.json")
	if err := profiles.UpdateActive(p); err != nil {
		t.Fatal(err)
	}
	ctrl := &Controller{profiles: profiles, runtime: NewRuntime(), progress: scan.NewProgress(), folders: map[string]*scan.FolderRecord{}}
	if err := ctrl.Start(false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for ctrl.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if ctrl.IsRunning() || ctrl.Progress().State() != "finished" {
		t.Fatalf("terminal state %s", ctrl.Progress().State())
	}
	if count := ctrl.SnapshotStats()["files"]; count != 3 {
		t.Fatalf("file count %v", count)
	}
	cp, ok := checkpoint.Load(p.CheckpointPath)
	if !ok || cp.LastPath != filepath.Join(root, "c.txt") {
		t.Fatalf("terminal checkpoint %+v", cp)
	}
	restarted := &Controller{profiles: newProfiles(settings), runtime: NewRuntime(), progress: scan.NewProgress(), folders: map[string]*scan.FolderRecord{}}
	if err := restarted.Start(true); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for restarted.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if restarted.IsRunning() || restarted.ResumeFrom() != cp.LastPath || restarted.SnapshotStats()["files"] != 1 {
		t.Fatalf("resume %+v from %s", restarted.SnapshotStats(), restarted.ResumeFrom())
	}
}
