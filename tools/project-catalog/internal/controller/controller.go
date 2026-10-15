//go:build cgo

package controller

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"scan-projects/internal/pipeline"
	"scan-projects/internal/scan"
	"scan-projects/internal/scan/checkpoint"
	"scan-projects/internal/storage"
)

type Controller struct {
	mu sync.Mutex

	app      fyne.App
	profiles *Profiles
	runtime  *Runtime

	progress *scan.Progress

	// scan lifecycle
	running bool
	paused  bool
	lastErr string

	ctx    context.Context
	cancel context.CancelFunc
	pause  chan bool

	// results for UI
	folders map[string]*scan.FolderRecord
	stats   map[string]interface{}

	// checkpoint throttle
	lastCPWrite time.Time
	lastSeen    string
}

func New(a fyne.App) *Controller {
	return &Controller{
		app:      a,
		profiles: NewProfiles(),
		runtime:  NewRuntime(),
		progress: scan.NewProgress(),
		pause:    make(chan bool, 1),
		folders:  map[string]*scan.FolderRecord{},
	}
}

func (c *Controller) Profiles() *Profiles           { return c.profiles }
func (c *Controller) Runtime() *scan.RuntimeOptions { return c.runtime.Opts }
func (c *Controller) Progress() *scan.Progress      { return c.progress }

func (c *Controller) IsRunning() bool   { c.mu.Lock(); defer c.mu.Unlock(); return c.running }
func (c *Controller) IsPaused() bool    { c.mu.Lock(); defer c.mu.Unlock(); return c.paused }
func (c *Controller) LastError() string { c.mu.Lock(); defer c.mu.Unlock(); return c.lastErr }

func (c *Controller) SnapshotFolders() map[string]*scan.FolderRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.folders
}

func (c *Controller) SnapshotStats() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

func enabledRoots(p storage.ScanProfile) []string {
	out := []string{}
	for _, r := range p.ScanRoots {
		if r.Enabled && r.Path != "" {
			out = append(out, r.Path)
		}
	}
	return out
}

func (c *Controller) Start(resume bool) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	p := c.profiles.Active()
	if errs := ValidateProfile(p); len(errs) > 0 {
		return errors.New(JoinErrors(errs))
	}

	roots := enabledRoots(p)
	if len(roots) == 0 {
		return fmt.Errorf("no enabled scan roots")
	}

	// initialize runtime from profile (so live toggles reflect current)
	c.runtime.Opts.HashFiles.Store(p.HashFiles)
	c.runtime.Opts.HashMaxSizeBytes.Store(p.HashMaxFileSizeMB * 1024 * 1024)
	c.runtime.Opts.ProjectDetection.Store(p.ProjectDetection)

	c.mu.Lock()
	c.running = true
	c.paused = false
	c.lastErr = ""
	c.folders = map[string]*scan.FolderRecord{}
	c.stats = nil
	c.lastSeen = ""
	c.lastCPWrite = time.Time{}
	c.mu.Unlock()

	c.progress.Reset()
	c.progress.SetState("running")

	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.pause = make(chan bool, 1)

	resumeFrom := ""
	if resume && p.CheckpointEnabled && p.CheckpointPath != "" {
		if cp, ok := checkpoint.Load(p.CheckpointPath); ok {
			resumeFrom = cp.LastPath
		}
	}

	opts := scan.ScanOptions{
		MaxFileSizeMB:     p.MaxFileSizeMB,
		HashFiles:         p.HashFiles,
		HashMaxFileSizeMB: p.HashMaxFileSizeMB,
		ProjectDetection:  p.ProjectDetection,
		Pause:             c.pause,
		Progress:          c.progress,
		Runtime:           c.runtime.Opts,
		OnWarning: func(path string, err error) {
			c.mu.Lock()
			c.lastErr = fmt.Sprintf("Partial scan: %s: %v", path, err)
			c.mu.Unlock()
		},
		OnCheckpoint: func(path string) {
			c.onCheckpoint(p, path)
		},
	}

	go func() {
		res, err := pipeline.DeepScan(c.ctx, pipeline.DeepScanInput{
			Roots:       roots,
			ExcludeDirs: p.ExcludeDirs,
			ResumeFrom:  resumeFrom,
			Options:     opts,
		})

		c.mu.Lock()
		if err != nil && err != context.Canceled {
			c.lastErr = err.Error()
		}
		c.folders = res.Folders
		c.stats = map[string]interface{}{
			"files":   len(res.Files),
			"folders": len(res.Folders),
		}
		c.running = false
		c.paused = false
		if err == context.Canceled {
			c.progress.SetState("canceled")
		}
		c.mu.Unlock()
	}()

	return nil
}

func (c *Controller) onCheckpoint(p storage.ScanProfile, currentPath string) {
	c.mu.Lock()
	c.lastSeen = currentPath
	now := time.Now()
	shouldWrite := p.CheckpointEnabled && p.CheckpointPath != "" && now.Sub(c.lastCPWrite) > 2*time.Second
	if shouldWrite {
		c.lastCPWrite = now
	}
	c.mu.Unlock()

	if shouldWrite {
		_ = checkpoint.Save(p.CheckpointPath, currentPath)
	}
}

func (c *Controller) Pause() {
	c.mu.Lock()
	if !c.running || c.paused {
		c.mu.Unlock()
		return
	}
	c.paused = true
	c.mu.Unlock()
	c.progress.SetState("paused")
	select {
	case c.pause <- true:
	default:
	}
}

func (c *Controller) Resume() {
	c.mu.Lock()
	if !c.running || !c.paused {
		c.mu.Unlock()
		return
	}
	c.paused = false
	c.mu.Unlock()
	c.progress.SetState("running")
	select {
	case c.pause <- false:
	default:
	}
}

func (c *Controller) Cancel() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		c.progress.SetState("canceled")
		cancel()
	}
}

func (c *Controller) DefaultCheckpointPath() string {
	return filepath.Join(storage.AppDataDir(), "output", "checkpoint.json")
}
