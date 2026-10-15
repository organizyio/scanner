// Package walk runs filesystem traversal with context cancel, optional pause, and resume skip.
package walk

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Handler receives directory and file callbacks during WalkDir traversal.
type Handler struct {
	HandleDir  func(path string, d os.DirEntry) error
	HandleFile func(path string, d os.DirEntry, info os.FileInfo) error
}

// Config drives a multi-root WalkDir with shared policy.
type Config struct {
	Ctx        context.Context
	Roots      []string
	Excluded   map[string]bool
	ResumeFrom string
	Pause      <-chan bool
	OnPaused   func()
	OnResumed  func()
	OnError    func(path string, err error)
}

// Walk visits every root in sorted order. Excluded directory names skip the subtree.
// When ResumeFrom is non-empty, paths lexicographically before ResumeFrom are skipped until reached.
func Walk(cfg Config, h *Handler) error {
	sort.Strings(cfg.Roots)

	shouldSkip := cfg.ResumeFrom != ""
	skipDone := false

	for _, root := range cfg.Roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if cfg.Ctx.Err() != nil {
				return cfg.Ctx.Err()
			}
			if err != nil {
				if cfg.OnError != nil {
					cfg.OnError(path, err)
				}
				return nil
			}

			select {
			case <-cfg.Ctx.Done():
				return cfg.Ctx.Err()
			default:
			}

			if cfg.Pause != nil {
				select {
				case paused := <-cfg.Pause:
					if paused {
						if cfg.OnPaused != nil {
							cfg.OnPaused()
						}
						for {
							select {
							case <-cfg.Ctx.Done():
								return cfg.Ctx.Err()
							case v := <-cfg.Pause:
								if !v {
									if cfg.OnResumed != nil {
										cfg.OnResumed()
									}
									goto resumed
								}
							}
						}
					}
				default:
				}
			}
		resumed:

			if shouldSkip && !skipDone {
				if strings.Compare(path, cfg.ResumeFrom) < 0 {
					return nil
				}
				skipDone = true
			}

			if d.IsDir() {
				if cfg.Excluded[d.Name()] {
					return filepath.SkipDir
				}
				if h.HandleDir != nil {
					return h.HandleDir(path, d)
				}
				return nil
			}

			info, e := d.Info()
			if e != nil {
				if cfg.OnError != nil {
					cfg.OnError(path, e)
				}
				return nil
			}
			if h.HandleFile != nil {
				return h.HandleFile(path, d, info)
			}
			return nil
		})

		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded || err == cfg.Ctx.Err() {
				return err
			}
		}
	}
	return nil
}
