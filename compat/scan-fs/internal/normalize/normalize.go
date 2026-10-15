package normalize

import (
	"path/filepath"

	"github.com/meshory/scan-fs/pkg/item"
)

// ToItem wraps a FileRecord with source and device identifiers and path-derived fields.
// Directory, Name, and Extension use path/filepath for the host OS (same machine as the scan).
func ToItem(rec item.FileRecord, source, device string) item.Item {
	return item.Item{
		FileRecord: rec,
		Source:     source,
		Device:     device,
		Directory:  filepath.Dir(rec.Path),
		Name:       filepath.Base(rec.Path),
		Extension:  filepath.Ext(rec.Path),
	}
}
