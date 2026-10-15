// Package pipeline orchestrates multi-step scans for CLI and GUI callers.
package pipeline

import (
	"context"

	"scan-projects/internal/analysis"
	"scan-projects/internal/scan"
	"scan-projects/internal/scan/filter"
)

// DeepScanInput configures a filesystem deep scan plus post-processing.
type DeepScanInput struct {
	Roots       []string
	ExcludeDirs []string
	ResumeFrom  string
	Options     scan.ScanOptions
}

// DeepScan runs scan.Scan then analysis.PostProcessFolders on success or cancel.
func DeepScan(ctx context.Context, in DeepScanInput) (scan.ScanResult, error) {
	res, err := scan.Scan(ctx, in.Roots, in.ExcludeDirs, in.Options, in.ResumeFrom)
	if err == nil || err == context.Canceled {
		analysis.PostProcessFolders(res.Folders, in.Options.ProjectDetection)
	}
	return res, err
}

// MergeDefaultExcludes returns user excludes plus built-in directory names unless omitDefaults is true.
func MergeDefaultExcludes(user []string, omitDefaults bool) []string {
	if omitDefaults {
		out := make([]string, len(user))
		copy(out, user)
		return out
	}
	seen := make(map[string]struct{}, len(user)+len(filter.DefaultExcludeDirNames))
	for _, e := range user {
		seen[e] = struct{}{}
	}
	out := append([]string(nil), user...)
	for _, d := range filter.DefaultExcludeDirNames {
		if _, ok := seen[d]; !ok {
			out = append(out, d)
			seen[d] = struct{}{}
		}
	}
	return out
}
