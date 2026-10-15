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
	running      bool
	paused       bool
	lastErr      string
	warningCount int

	ctx    context.Context
	cancel context.CancelFunc
	pause  chan bool

	// results for UI
	folders map[string]*scan.FolderRecord
	stats   map[string]interface{}

	// checkpoint throttle
	lastCPWrite time.Time
	lastSeen    string
	resumeFrom  string
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

func (c *Controller) IsRunning() bool    { c.mu.Lock(); defer c.mu.Unlock(); return c.running }
func (c *Controller) IsPaused() bool     { c.mu.Lock(); defer c.mu.Unlock(); return c.paused }
func (c *Controller) LastError() string  { c.mu.Lock(); defer c.mu.Unlock(); return c.lastErr }
func (c *Controller) ResumeFrom() string { c.mu.Lock(); defer c.mu.Unlock(); return c.resumeFrom }

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
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.pause = make(chan bool, 1)
	ctx := c.ctx
	pause := c.pause
	c.running = true
	c.paused = false
	c.lastErr = ""
	c.warningCount = 0
	c.folders = map[string]*scan.FolderRecord{}
	c.stats = nil
	c.lastSeen = ""
	c.lastCPWrite = time.Time{}
	c.mu.Unlock()

	c.progress.Reset()
	c.progress.SetState("running")

	resumeFrom := ""
	if resume && p.CheckpointEnabled && p.CheckpointPath != "" {
		if cp, ok := checkpoint.Load(p.CheckpointPath); ok {
			resumeFrom = cp.LastPath
		}
	}

	c.mu.Lock()
	c.resumeFrom = resumeFrom
	c.mu.Unlock()
	opts := scan.ScanOptions{
		MaxFileSizeMB:     p.MaxFileSizeMB,
		HashFiles:         p.HashFiles,
		HashMaxFileSizeMB: p.HashMaxFileSizeMB,
		ProjectDetection:  p.ProjectDetection,
		Pause:             pause,
		Progress:          c.progress,
		Runtime:           c.runtime.Opts,
		OnWarning: func(path string, err error) {
			c.reportWarning(path, err)
		},
		OnCheckpoint: func(path string) {
			c.onCheckpoint(p, path)
		},
	}

	go func() {
		res, err := pipeline.DeepScan(ctx, pipeline.DeepScanInput{
			Roots:       roots,
			ExcludeDirs: p.ExcludeDirs,
			ResumeFrom:  resumeFrom,
			Options:     opts,
		})

		c.flushCheckpoint(p)
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
		if ctx.Err() != nil {
			c.progress.SetState("canceled")
		} else if err != nil {
			c.progress.SetState("error")
		} else {
			c.progress.SetState("finished")
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
		if err := checkpoint.Save(p.CheckpointPath, currentPath); err != nil {
			c.reportWarning(p.CheckpointPath, err)
		}
	}
}

// publishPause replaces a stale queued intent; rapid Pause/Resume must not lose Resume.
func publishPause(ch chan bool, paused bool) {
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- paused:
	default:
	}
}

func (c *Controller) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || c.paused || (c.ctx != nil && c.ctx.Err() != nil) {
		return
	}
	c.paused = true
	c.progress.SetState("paused")
	publishPause(c.pause, true)
}
func (c *Controller) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || !c.paused || (c.ctx != nil && c.ctx.Err() != nil) {
		return
	}
	c.paused = false
	c.progress.SetState("running")
	publishPause(c.pause, false)
}
func (c *Controller) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running || c.cancel == nil {
		return
	}
	c.paused = false
	c.progress.SetState("canceling")
	c.cancel()
}
func (c *Controller) reportWarning(path string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.warningCount++
	c.lastErr = fmt.Sprintf("Partial scan (%d warnings): %s: %v", c.warningCount, path, err)
}
func (c *Controller) flushCheckpoint(p storage.ScanProfile) {
	c.mu.Lock()
	last := c.lastSeen
	c.mu.Unlock()
	if p.CheckpointEnabled && p.CheckpointPath != "" && last != "" {
		if err := checkpoint.Save(p.CheckpointPath, last); err != nil {
			c.reportWarning(p.CheckpointPath, err)
		}
	}
}

func (c *Controller) DefaultCheckpointPath() string {
	return filepath.Join(storage.AppDataDir(), "output", "checkpoint.json")
}
