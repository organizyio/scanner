package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"scan-projects/internal/pipeline"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return fmt.Sprintf("%v", []string(*i))
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// RunCatalog runs the catalog / inventory CLI (same flags as legacy scan-git). Returns a process exit code.
func RunCatalog(args []string) int {
	var roots arrayFlags
	var excludes arrayFlags
	var output string
	var verbose bool

	fs := flag.NewFlagSet("catalog", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: scan-cli catalog [options]\n\n")
		fs.PrintDefaults()
	}

	fs.Var(&roots, "root", "Root directory to scan (can be specified multiple times)")
	fs.Var(&excludes, "exclude", "Directory name to exclude from scanning (can be specified multiple times)")
	fs.StringVar(&output, "output", "", "Output JSON file path (required)")
	fs.StringVar(&output, "o", "", "Output JSON file path (shorthand)")
	fs.BoolVar(&verbose, "verbose", false, "Enable verbose/debug output")
	fs.BoolVar(&verbose, "v", false, "Enable verbose/debug output (shorthand)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "Error: at least one --root must be specified")
		return 1
	}
	if output == "" {
		fmt.Fprintln(os.Stderr, "Error: --output/-o is required")
		return 1
	}

	if _, err := exec.LookPath("git"); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] git command not found in PATH. Git metadata will not be collected.\n")
	}

	extra := make([]string, len(excludes))
	copy(extra, excludes)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	allProjects := pipeline.CatalogInventory(pipeline.CatalogInput{
		Context:       ctx,
		Roots:         []string(roots),
		ExtraExcludes: extra,
		Verbose:       verbose,
	})

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output dir: %v\n", err)
		return 1
	}

	f, err := os.Create(output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		return 1
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(allProjects); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
		return 1
	}

	fmt.Printf("[INFO] Written %d projects to %s\n", len(allProjects), output)
	return 0
}
