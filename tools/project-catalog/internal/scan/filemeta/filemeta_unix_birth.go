//go:build darwin || freebsd || netbsd

package filemeta

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// Fill uses birth time (Btim) when non-zero, else modification time for created_at.
func Fill(path string, name string, info os.FileInfo) Extra {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return fallbackExtra(name, info)
	}

	mod := timespecUTC(st.Mtim)
	acc := timespecUTC(st.Atim)
	cre := birthOrMod(st.Btim, st.Mtim)

	m := info.Mode()
	return Extra{
		CreatedAt:   cre.Format(time.RFC3339),
		ModifiedAt:  mod.Format(time.RFC3339),
		AccessedAt:  acc.Format(time.RFC3339),
		ReadOnly:    m.IsRegular() && m&0200 == 0,
		Hidden:      isUnixHidden(name),
		SystemFile:  false,
	}
}

func birthOrMod(birth, mod unix.Timespec) time.Time {
	if birth.Sec != 0 || birth.Nsec != 0 {
		return timespecUTC(birth)
	}
	return timespecUTC(mod)
}

func timespecUTC(ts unix.Timespec) time.Time {
	return time.Unix(ts.Sec, ts.Nsec).UTC()
}

func isUnixHidden(name string) bool {
	return len(name) > 0 && name[0] == '.'
}
