//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !solaris && !android && !aix

package filemeta

import "os"

// Fill falls back to ModTime-only metadata on uncommon build targets.
func Fill(path string, name string, info os.FileInfo) Extra {
	_ = path
	return fallbackExtra(name, info)
}
