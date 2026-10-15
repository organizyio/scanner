package scan

import (
	"context"
	"os"

	"scan-projects/internal/scan/filter"
	"scan-projects/internal/scan/walk"
)

// StreamKind classifies a ScanStream event.
type StreamKind int

const (
	StreamFile StreamKind = iota
	StreamFinished
	StreamError
)

// StreamEvent is emitted by ScanStream for each ingested file, then a terminal event.
type StreamEvent struct {
	Kind   StreamKind
	File   FileRecord
	Result ScanResult
	Err    error
}

// StartScanStream runs ScanStream in a goroutine. Cancel the returned function to stop early.
// buffer is the channel buffer size (minimum 1). Events are StreamFile per file, then either
// StreamFinished or StreamError with partial Result in both cases.
func StartScanStream(ctx context.Context, roots []string, excludeDirs []string, opts ScanOptions, resumeFromPath string, buffer int) (<-chan StreamEvent, context.CancelFunc) {
	if buffer < 1 {
		buffer = 64
	}
	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan StreamEvent, buffer)
	go scanStream(ctx, roots, excludeDirs, opts, resumeFromPath, ch)
	return ch, cancel
}

func scanStream(ctx context.Context, roots []string, excludeDirs []string, opts ScanOptions, resumeFromPath string, ch chan<- StreamEvent) {
	defer close(ch)

	files := []FileRecord{}
	folders := map[string]*FolderRecord{}
	excluded := filter.DirSet(excludeDirs)

	onPaused := func() {
		if opts.Progress != nil {
			opts.Progress.SetState("paused")
		}
	}
	onResumed := func() {
		if opts.Progress != nil {
			opts.Progress.SetState("running")
		}
	}

	h := &walk.Handler{
		HandleDir: func(path string, d os.DirEntry) error {
			initFolder(folders, path)
			return nil
		},
		HandleFile: func(path string, d os.DirEntry, info os.FileInfo) error {
			rec, ok := ingestFile(path, d, info, folders, opts)
			if ok {
				files = append(files, rec)
				select {
				case ch <- StreamEvent{Kind: StreamFile, File: rec}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		},
	}

	err := walk.Walk(walk.Config{
		Ctx:        ctx,
		Roots:      roots,
		Excluded:   excluded,
		ResumeFrom: resumeFromPath,
		Pause:      opts.Pause,
		OnPaused:   onPaused,
		OnResumed:  onResumed,
	}, h)

	finalizeFolders(folders)
	if opts.Progress != nil && ctx.Err() == nil && err == nil {
		opts.Progress.SetState("finished")
	}

	res := ScanResult{Files: files, Folders: folders}
	if err != nil {
		ch <- StreamEvent{Kind: StreamError, Result: res, Err: err}
		return
	}
	ch <- StreamEvent{Kind: StreamFinished, Result: res}
}
