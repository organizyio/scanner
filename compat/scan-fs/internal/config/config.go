package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Config holds CLI flags for scan-fs and scan-engine passthrough.
type Config struct {
	Roots       []string
	Excludes    []string
	Hash        bool
	HashPartial bool
	Sha256      bool
	SniffFormat bool
	Metadata    bool
	Video       bool
	Phash       bool
	Format      string // output: json, table, plain
	Workers     int    // if > 0, set RAYON_NUM_THREADS for the engine process
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// Parse builds Config from command-line arguments (typically os.Args[1:]).
func Parse(args []string) (*Config, error) {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			printUsage()
			return nil, flag.ErrHelp
		}
	}

	fs := flag.NewFlagSet("scan-fs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var roots stringList
	fs.Var(&roots, "root", "root directory to scan (repeatable, required)")
	var excludes stringList
	fs.Var(&excludes, "exclude", "extra ignore-style glob (repeatable)")

	hash := fs.Bool("hash", false, "full-file xxHash64 (passed to scan-engine)")
	hashPartial := fs.Bool("hash-partial", false, "partial xxHash64 for large files (passed to scan-engine)")
	sha256 := fs.Bool("sha256", false, "SHA-256 (passed to scan-engine)")
	sniffFormat := fs.Bool("sniff-format", false, "sniff MIME / magic (scan-engine --format)")
	metadata := fs.Bool("metadata", false, "EXIF, audio tags, PDF, DOCX (passed to scan-engine)")
	video := fs.Bool("video", false, "ffprobe for video (passed to scan-engine)")
	phash := fs.Bool("phash", false, "perceptual hash for images (passed to scan-engine)")

	format := fs.String("format", "plain", "output format: json, table, or plain")
	workers := fs.Int("workers", 0, "if > 0, sets RAYON_NUM_THREADS for scan-engine")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, errors.New("at least one --root is required")
	}
	f := strings.ToLower(strings.TrimSpace(*format))
	switch f {
	case "json", "table", "plain":
	default:
		return nil, fmt.Errorf("invalid --format %q (want json, table, or plain)", *format)
	}

	return &Config{
		Roots:       []string(roots),
		Excludes:    []string(excludes),
		Hash:        *hash,
		HashPartial: *hashPartial,
		Sha256:      *sha256,
		SniffFormat: *sniffFormat,
		Metadata:    *metadata,
		Video:       *video,
		Phash:       *phash,
		Format:      f,
		Workers:     *workers,
	}, nil
}

// EngineArgs builds argv for scan-engine (scan binary).
func (c *Config) EngineArgs() []string {
	var argv []string
	for _, r := range c.Roots {
		argv = append(argv, "--root", r)
	}
	for _, e := range c.Excludes {
		argv = append(argv, "--exclude", e)
	}
	if c.Hash {
		argv = append(argv, "--hash")
	}
	if c.HashPartial {
		argv = append(argv, "--hash-partial")
	}
	if c.Sha256 {
		argv = append(argv, "--sha256")
	}
	if c.SniffFormat {
		argv = append(argv, "--format")
	}
	if c.Metadata {
		argv = append(argv, "--metadata")
	}
	if c.Video {
		argv = append(argv, "--video")
	}
	if c.Phash {
		argv = append(argv, "--phash")
	}
	return argv
}

// EngineEnv returns the environment for the engine child (inherits os.Environ + optional Rayon).
func (c *Config) EngineEnv() []string {
	env := os.Environ()
	if c.Workers > 0 {
		env = append(env, "RAYON_NUM_THREADS="+strconv.Itoa(c.Workers))
	}
	return env
}

func printUsage() {
	const usage = "" +
		"Usage: scan-fs --root <path> [options]\n\n" +
		"scan-fs runs the bundled scan-engine binary and prints normalized results.\n" +
		"Set SCAN_ENGINE_PATH to use an external engine instead of the embedded binary.\n\n" +
		"Options:\n" +
		"  --root path          Root directory to scan (required, repeatable)\n" +
		"  --exclude glob       Extra ignore glob (repeatable)\n" +
		"  --hash               Full-file xxHash64 (engine)\n" +
		"  --hash-partial       Partial xxHash64 for large files (engine)\n" +
		"  --sha256             SHA-256 (engine)\n" +
		"  --sniff-format       Sniff MIME / magic (engine --format)\n" +
		"  --metadata           EXIF, audio, PDF, DOCX (engine)\n" +
		"  --video              ffprobe for video (engine)\n" +
		"  --phash              Perceptual hash (engine)\n" +
		"  --format string      Output: json, table, plain (default plain)\n" +
		"  --workers n          If > 0, sets RAYON_NUM_THREADS for the engine\n" +
		"  -h, --help           Show this help\n"
	_, _ = io.WriteString(os.Stderr, usage)
}
