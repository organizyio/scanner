package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"scan-projects/internal/scan/filter"
	"scan-projects/internal/scan/walk"
)

func Scan(ctx context.Context, roots []string, excludeDirs []string, opts ScanOptions, resumeFromPath string) (ScanResult, error) {
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
		OnError:    warningReporter(opts),
	}, h)

	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded || err == ctx.Err() {
			return ScanResult{Files: files, Folders: folders}, err
		}
	}

	finalizeFolders(folders)

	if opts.Progress != nil && ctx.Err() == nil {
		opts.Progress.SetState("finished")
	}

	return ScanResult{Files: files, Folders: folders}, nil
}

func initFolder(folders map[string]*FolderRecord, path string) {
	if _, exists := folders[path]; exists {
		return
	}
	depth := len(strings.Split(filepath.Clean(path), string(os.PathSeparator)))
	folders[path] = &FolderRecord{
		Path:        path,
		Depth:       depth,
		Extensions:  map[string]int64{},
		MarkerFiles: map[string]bool{},
	}
}

func updateFolderStats(fr *FolderRecord, ext string, info os.FileInfo) {
	fr.FileCount++
	fr.TotalSizeBytes += info.Size()
	fr.Extensions[ext]++

	mod := info.ModTime().Format(time.RFC3339)
	if fr.OldestModifiedAt == "" || mod < fr.OldestModifiedAt {
		fr.OldestModifiedAt = mod
	}
	if fr.NewestModifiedAt == "" || mod > fr.NewestModifiedAt {
		fr.NewestModifiedAt = mod
	}
}

func finalizeFolders(folders map[string]*FolderRecord) {
	for path, fr := range folders {
		_ = path
		fr.SubfolderCount = 0
	}

	keys := make([]string, 0, len(folders))
	for k := range folders {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, parent := range keys {
		prefix := parent
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
			prefix += string(os.PathSeparator)
		}
		for _, child := range keys {
			if child != parent && strings.HasPrefix(child, prefix) {
				rest := strings.TrimPrefix(child, prefix)
				if rest != "" && !strings.Contains(rest, string(os.PathSeparator)) {
					folders[parent].SubfolderCount++
				}
			}
		}
	}

	for _, fr := range folders {
		var maxExt string
		var maxCount int64
		for ext, count := range fr.Extensions {
			if count > maxCount {
				maxCount = count
				maxExt = ext
			}
		}
		fr.DominantExtension = maxExt
		fr.UniqueExtensions = len(fr.Extensions)
		fr.ExtensionEntropy = extensionEntropy(fr.Extensions)
	}
}
