// Package filemeta fills OS-specific file metadata for deep scan records.
package filemeta

import (
	"os"
	"strings"
	"time"
)

// Extra holds timestamps and attribute flags from the filesystem.
type Extra struct {
	CreatedAt  string
	ModifiedAt string
	AccessedAt string
	ReadOnly   bool
	Hidden     bool
	SystemFile bool
}

func fallbackExtra(name string, info os.FileInfo) Extra {
	t := info.ModTime().UTC()
	ts := t.Format(time.RFC3339)
	return Extra{
		CreatedAt:  "",
		ModifiedAt: ts,
		AccessedAt: "",
		ReadOnly:   false,
		Hidden:     strings.HasPrefix(name, "."),
		SystemFile: false,
	}
}
