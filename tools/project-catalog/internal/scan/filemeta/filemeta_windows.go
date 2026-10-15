//go:build windows

package filemeta

import (
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Fill reads creation, access, and write times and attribute flags from the Win32 API.
func Fill(path string, name string, info os.FileInfo) Extra {
	var fad windows.Win32FileAttributeData
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fallbackExtra(name, info)
	}
	err = windows.GetFileAttributesEx(p, windows.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&fad)))
	if err != nil {
		return fallbackExtra(name, info)
	}

	mod := time.Unix(0, fad.LastWriteTime.Nanoseconds()).UTC()
	cre := ""
	if fad.CreationTime.HighDateTime != 0 || fad.CreationTime.LowDateTime != 0 {
		cre = time.Unix(0, fad.CreationTime.Nanoseconds()).UTC().Format(time.RFC3339)
	}
	acc := time.Unix(0, fad.LastAccessTime.Nanoseconds()).UTC()

	fa := fad.FileAttributes
	return Extra{
		CreatedAt:  cre,
		ModifiedAt: mod.Format(time.RFC3339),
		AccessedAt: acc.Format(time.RFC3339),
		ReadOnly:   fa&windows.FILE_ATTRIBUTE_READONLY != 0,
		Hidden:     fa&windows.FILE_ATTRIBUTE_HIDDEN != 0,
		SystemFile: fa&windows.FILE_ATTRIBUTE_SYSTEM != 0,
	}
}
