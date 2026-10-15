package scan

import (
	"strings"

	"scan-projects/internal/markers"
)

// NoteMarkerFile records presence of a known marker filename in the parent folder aggregate.
// It does not set ProjectKind; analysis infers kind from MarkerFiles after the scan.
func NoteMarkerFile(fr *FolderRecord, name string) {
	if fr.MarkerFiles == nil {
		fr.MarkerFiles = map[string]bool{}
	}
	l := strings.ToLower(name)

	if _, ok := markers.MarkerKind(l); ok {
		fr.MarkerFiles[l] = true
	}
}
