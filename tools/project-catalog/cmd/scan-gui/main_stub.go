//go:build !cgo

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "The scan-gui app requires CGO and a C compiler (OpenGL via Fyne).")
	fmt.Fprintln(os.Stderr, "On Windows: install MinGW-w64 or MSYS2, add gcc to PATH, then set CGO_ENABLED=1 and rebuild.")
	os.Exit(1)
}
