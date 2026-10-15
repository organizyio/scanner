package controller

import (
	"fmt"
	"os"

	"scan-projects/internal/storage"
)

func ValidateProfile(p storage.ScanProfile) []error {
	var errs []error
	enabledRoots := 0

	for _, r := range p.ScanRoots {
		if !r.Enabled {
			continue
		}
		enabledRoots++

		info, err := os.Stat(r.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("root not accessible: %s", r.Path))
			continue
		}
		if !info.IsDir() {
			errs = append(errs, fmt.Errorf("not a directory: %s", r.Path))
		}
	}

	if enabledRoots == 0 {
		errs = append(errs, fmt.Errorf("no enabled scan roots"))
	}
	return errs
}

func JoinErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	s := "Validation errors:\n"
	for _, e := range errs {
		s += " - " + e.Error() + "\n"
	}
	return s
}
