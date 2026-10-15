package cli

import (
	"fmt"
	"os"
)

// PrintRootUsage writes scan-cli top-level help to w.
func PrintRootUsage() {
	fmt.Fprintf(os.Stderr, `scan-cli — unified catalog and deep filesystem scan

Usage:
  scan-cli <command> [options]

Commands:
  catalog   Find project roots (Git or markers), stacks, Git metadata → JSON
  fs        Walk files under roots, optional SHA-256, folder stats → JSON

Run "scan-cli <command> -h" for command-specific flags.
`)
}
