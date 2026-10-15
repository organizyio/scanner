package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"scan-projects/internal/scan/filemeta"
	"scan-projects/internal/scan/hash"
)

// ingestFile updates folder aggregates and returns a file record. handled is false when the file is skipped (e.g. size limit).
func ingestFile(path string, d os.DirEntry, info os.FileInfo, folders map[string]*FolderRecord, opts ScanOptions) (rec FileRecord, handled bool) {
	if opts.MaxFileSizeMB > 0 && info.Size() > opts.MaxFileSizeMB*1024*1024 {
		return FileRecord{}, false
	}

	if opts.Progress != nil {
		opts.Progress.FileSeen(info.Size())
		opts.Progress.SetLastPath(path)
	}

	ext := strings.ToLower(filepath.Ext(d.Name()))
	dir := filepath.Dir(path)

	initFolder(folders, dir)
	updateFolderStats(folders[dir], ext, info)
	NoteMarkerFile(folders[dir], d.Name())

	hashFiles := opts.HashFiles
	hashMaxBytes := opts.HashMaxFileSizeMB * 1024 * 1024
	if opts.Runtime != nil {
		hashFiles = opts.Runtime.HashFiles.Load()
		// Zero explicitly means unlimited; it must not restore the initial limit.
		hashMaxBytes = opts.Runtime.HashMaxSizeBytes.Load()
	}

	sha := ""
	hashStatus := "skipped"
	if hashFiles {
		if hashMaxBytes <= 0 || info.Size() <= hashMaxBytes {
			h, herr := hash.SHA256(path)
			if herr != nil {
				hashStatus = "error"
				warningReporter(opts)(path, herr)
			} else {
				sha = h
				hashStatus = "ok"
				if opts.Progress != nil {
					opts.Progress.FileHashed()
				}
			}
		}
	}

	sizeBytes := info.Size()
	meta := filemeta.Fill(path, d.Name(), info)
	rec = FileRecord{
		Path:       path,
		Name:       d.Name(),
		Directory:  dir,
		Type:       "file",
		Extension:  ext,
		SizeBytes:  sizeBytes,
		SizeMB:     float64(sizeBytes) / (1024 * 1024),
		CreatedAt:  meta.CreatedAt,
		ModifiedAt: meta.ModifiedAt,
		AccessedAt: meta.AccessedAt,
		ReadOnly:   meta.ReadOnly,
		Hidden:     meta.Hidden,
		SystemFile: meta.SystemFile,
		Sha256:     sha,
		HashStatus: hashStatus,
	}

	if opts.OnCheckpoint != nil {
		opts.OnCheckpoint(path)
	}

	return rec, true
}

func warningReporter(opts ScanOptions) func(string, error) {
	return func(path string, err error) {
		if opts.OnWarning != nil {
			opts.OnWarning(path, err)
		} else {
			fmt.Fprintf(os.Stderr, "[WARN] Partial scan: %s: %v\n", path, err)
		}
	}
}
