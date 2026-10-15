package scan

import (
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
	hashMaxMB := opts.HashMaxFileSizeMB
	projectDetect := opts.ProjectDetection
	if opts.Runtime != nil {
		hashFiles = opts.Runtime.HashFiles.Load()
		maxBytes := opts.Runtime.HashMaxSizeBytes.Load()
		if maxBytes > 0 {
			hashMaxMB = maxBytes / (1024 * 1024)
		}
		projectDetect = opts.Runtime.ProjectDetection.Load()
		_ = projectDetect
	}

	sha := ""
	hashStatus := "skipped"
	if hashFiles {
		if hashMaxMB <= 0 || info.Size() <= hashMaxMB*1024*1024 {
			h, herr := hash.SHA256(path)
			if herr != nil {
				hashStatus = "error"
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
