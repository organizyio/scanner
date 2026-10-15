// Package markers holds shared filename conventions for project detection across scan and catalog.
package markers

import "strings"

// DeepFileKind maps lowercase file or directory names to a coarse technology kind.
// Scan records marker filenames only; internal/analysis infers ProjectKind from MarkerFiles.
// DeepFileKind remains a compatibility view; detection uses MarkerKind for extensions too.
var DeepFileKind = func() map[string]string {
	out := map[string]string{".git": "git"}
	for f := range CatalogRootFilenames {
		out[strings.ToLower(f)] = "unknown"
	}
	for kind, m := range StackMarkers {
		for f := range m.Filenames {
			out[strings.ToLower(f)] = kind
		}
	}
	return out
}()
