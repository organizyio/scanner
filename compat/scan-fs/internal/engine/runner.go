package engine

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Embedded payloads live in this package's bin/ subtree (copied from repo root bin/ by make fetch-engine).
//
//go:embed all:bin
var binEmbed embed.FS

// ErrNoBundledBinary is returned when the embedded bin/ tree has no engine for this platform.
var ErrNoBundledBinary = errors.New("bundled scan-engine binary not found (run make fetch-engine)")

// Materialize returns an executable path to scan-engine, a cleanup function, and an error.
// If env SCAN_ENGINE_PATH is set, that path is used and cleanup is a no-op.
// Otherwise the matching file is extracted from the embedded bin/ tree into a temp directory
// removed by cleanup.
func Materialize() (path string, cleanup func(), err error) {
	cleanup = func() {}
	if p := os.Getenv("SCAN_ENGINE_PATH"); p != "" {
		return p, cleanup, nil
	}

	name, err := BinaryName()
	if err != nil {
		return "", cleanup, err
	}

	rel := filepath.Join("bin", name)
	data, err := fs.ReadFile(binEmbed, filepath.ToSlash(rel))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", cleanup, fmt.Errorf("%w: %s", ErrNoBundledBinary, rel)
		}
		return "", cleanup, err
	}

	dir, err := os.MkdirTemp("", "scan-engine-*")
	if err != nil {
		return "", cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	outPath := filepath.Join(dir, name)
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		cleanup()
		cleanup = func() {}
		return "", cleanup, err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(outPath, 0o755); err != nil {
			cleanup()
			cleanup = func() {}
			return "", cleanup, err
		}
	}

	return outPath, cleanup, nil
}

// StartStdoutPipe starts scan-engine with argv; read NDJSON from the returned reader.
// Close the reader after EOF, then call cmd.Wait().
func StartStdoutPipe(ctx context.Context, enginePath string, argv []string, env []string) (*exec.Cmd, io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, enginePath, argv...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = r.Close()
		return nil, nil, err
	}
	return cmd, r, nil
}
