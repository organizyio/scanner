//go:build linux || openbsd || dragonfly || solaris || android || aix

package filemeta

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// Fill uses atime/mtime from Lstat. created_at is empty (no birth time in Stat_t).
func Fill(path string, name string, info os.FileInfo) Extra {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return fallbackExtra(name, info)
	}

	mod := timespecUTC(st.Mtim)
	acc := timespecUTC(st.Atim)

	m := info.Mode()
	return Extra{
		CreatedAt:  "",
		ModifiedAt: mod.Format(time.RFC3339),
		AccessedAt: acc.Format(time.RFC3339),
		ReadOnly:   m.IsRegular() && m&0200 == 0,
		Hidden:     isUnixHidden(name),
		SystemFile: false,
	}
}

func timespecUTC(ts unix.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec).UTC()
}

func isUnixHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
