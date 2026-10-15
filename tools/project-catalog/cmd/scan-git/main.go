// Command scan-git discovers project roots (Git or markers), stacks, and Git metadata → JSON.
// Legacy binary name: scan-projects. Prefer: scan-cli catalog
package main

import (
	"os"

	"scan-projects/internal/cli"
)

func main() {
	os.Exit(cli.RunCatalog(os.Args[1:]))
}
