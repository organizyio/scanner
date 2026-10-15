package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/meshory/scan-fs/internal/config"
	"github.com/meshory/scan-fs/internal/engine"
	"github.com/meshory/scan-fs/internal/normalize"
	"github.com/meshory/scan-fs/internal/output"
	"github.com/meshory/scan-fs/internal/stream"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, "scan-fs:", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	enginePath, cleanup, err := engine.Materialize()
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan-fs:", err)
		return 1
	}
	defer cleanup()

	cmd, stdout, err := engine.StartStdoutPipe(ctx, enginePath, cfg.EngineArgs(), cfg.EngineEnv())
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan-fs:", err)
		return 1
	}
	defer func() { _ = stdout.Close() }()

	source := os.Getenv("SCAN_FS_SOURCE")
	if source == "" {
		var hErr error
		source, hErr = os.Hostname()
		if hErr != nil {
			source = "unknown"
		}
	}
	device := cfg.Roots[0]

	lines := stream.DecodeLines(ctx, stdout)

	var enc *json.Encoder
	if cfg.Format == "json" {
		enc = output.JSONEncoder(os.Stdout)
	}
	var tab *output.Table
	if cfg.Format == "table" {
		tab = output.NewTable(os.Stdout)
	}

	for res := range lines {
		if res.Err != nil {
			if errors.Is(res.Err, context.Canceled) {
				_ = cmd.Process.Kill()
				fmt.Fprintln(os.Stderr, "scan-fs: interrupted")
				return 130
			}
			fmt.Fprintln(os.Stderr, "scan-fs:", res.Err)
			_ = cmd.Wait()
			return 1
		}
		it := normalize.ToItem(res.Record, source, device)
		switch cfg.Format {
		case "plain":
			if err := output.Plain(os.Stdout, it); err != nil {
				fmt.Fprintln(os.Stderr, "scan-fs:", err)
				_ = cmd.Wait()
				return 1
			}
		case "json":
			if err := output.JSONLine(enc, it); err != nil {
				fmt.Fprintln(os.Stderr, "scan-fs:", err)
				_ = cmd.Wait()
				return 1
			}
		case "table":
			tab.Add(it)
		}
	}

	if err := cmd.Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "scan-fs: scan-engine:", err)
		return 1
	}

	if cfg.Format == "table" {
		tab.Flush()
	}

	return 0
}
