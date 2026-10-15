// Command scan-cli runs catalog or deep filesystem scan subcommands.
package main

import (
	"fmt"
	"os"

	"scan-projects/internal/cli"
)

func main() {
	if len(os.Args) < 2 {
		cli.PrintRootUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "fs":
		os.Exit(cli.RunFS(os.Args[2:]))
	case "catalog":
		os.Exit(cli.RunCatalog(os.Args[2:]))
	case "-h", "-help", "--help":
		cli.PrintRootUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "scan-cli: unknown command %q\n\n", os.Args[1])
		cli.PrintRootUsage()
		os.Exit(2)
	}
}
