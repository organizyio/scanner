//go:build darwin || freebsd || netbsd

package filemeta

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// Fill uses birth time (Btim) when non-zero, else empty created_at.
func Fill(path string, name string, info os.FileInfo) Extra {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return fallbackExtra(name, info)
	}

	mod := timespecUTC(st.Mtim)
	acc := timespecUTC(st.Atim)
	cre := ""
	if st.Btim.Sec != 0 || st.Btim.Nsec != 0 {
		cre = timespecUTC(st.Btim).Format(time.RFC3339)
	}

	m := info.Mode()
	return Extra{
		CreatedAt:  cre,
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
