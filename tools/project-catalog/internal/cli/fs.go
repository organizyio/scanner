package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"scan-projects/internal/pipeline"
	"scan-projects/internal/scan"
)

// RunFS runs the deep scan CLI (same flags as legacy scan-fs). Returns a process exit code.
func RunFS(args []string) int {
	var roots []string
	var excludes []string
	var output string
	var noHash bool
	var maxFileMB int64
	var noProjects bool
	var noDefaultExcludes bool

	fs := flag.NewFlagSet("fs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: scan-cli fs [options]\n\n")
		fs.PrintDefaults()
	}

	fs.Func("root", "Root directory (repeatable)", func(v string) error {
		abs, err := filepath.Abs(v)
		if err != nil {
			return err
		}
		roots = append(roots, abs)
		return nil
	})
	fs.Func("exclude", "Directory name to exclude (repeatable)", func(v string) error {
		excludes = append(excludes, strings.TrimSpace(v))
		return nil
	})
	fs.StringVar(&output, "output", "", "Output JSON path (required)")
	fs.StringVar(&output, "o", "", "Output JSON path (shorthand)")
	fs.BoolVar(&noHash, "no-hash", false, "Disable SHA-256 hashing")
	fs.Int64Var(&maxFileMB, "max-file-mb", 0, "Skip files larger than this size in MB (0 = no limit)")
	fs.BoolVar(&noProjects, "no-projects", false, "Disable project root detection")
	fs.BoolVar(&noDefaultExcludes, "no-default-excludes", false, "Do not apply built-in exclude dir names (e.g. node_modules, .git)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if output == "" {
		fmt.Fprintln(os.Stderr, "error: -output / -o is required")
		return 2
	}
	if len(roots) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one -root is required")
		return 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	excludeDirs := pipeline.MergeDefaultExcludes(excludes, noDefaultExcludes)
	opts := scan.ScanOptions{
		MaxFileSizeMB:     maxFileMB,
		HashFiles:         !noHash,
		HashMaxFileSizeMB: 0,
		ProjectDetection:  !noProjects,
	}

	res, err := pipeline.DeepScan(ctx, pipeline.DeepScanInput{
		Roots:       roots,
		ExcludeDirs: excludeDirs,
		ResumeFrom:  "",
		Options:     opts,
	})
	if err != nil && err != context.Canceled {
		fmt.Fprintln(os.Stderr, "scan error:", err)
		return 1
	}

	type out struct {
		Files   []scan.FileRecord             `json:"files"`
		Folders map[string]*scan.FolderRecord `json:"folders"`
	}
	payload := out{Files: res.Files, Folders: res.Folders}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "json error:", err)
		return 1
	}
	if err := os.WriteFile(output, b, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "write error:", err)
		return 1
	}
	fmt.Println("wrote", output)
	return 0
}
