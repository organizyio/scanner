// Command scan-fs runs the deep scan pipeline without the Fyne GUI and writes JSON.
// Legacy binary name: scan-export. Prefer: scan-cli fs
package main

import (
	"os"

	"scan-projects/internal/cli"
)

func main() {
	os.Exit(cli.RunFS(os.Args[1:]))
}
