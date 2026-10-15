// Package analysis interprets raw scan output (e.g. project roots) without performing traversal.
package analysis

import (
	"sort"
	"strings"

	"scan-projects/internal/markers"
	"scan-projects/internal/scan"
)

// PostProcessFolders applies project-kind inference and project-root detection to folder aggregates.
func PostProcessFolders(folders map[string]*scan.FolderRecord, projectDetection bool) {
	for _, fr := range folders {
		if projectDetection {
			inferProjectKind(fr)
		}
		FinalizeProjectRoot(fr, projectDetection)
	}
}

// inferProjectKind sets ProjectKind from MarkerFiles using sorted marker names (deterministic).
func inferProjectKind(fr *scan.FolderRecord) {
	fr.ProjectKind = ""
	if len(fr.MarkerFiles) == 0 {
		return
	}
	keys := make([]string, 0, len(fr.MarkerFiles))
	for k, v := range fr.MarkerFiles {
		if v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, l := range keys {
		if kind, ok := markers.DeepFileKind[l]; ok {
			if fr.ProjectKind == "" || fr.ProjectKind == "unknown" {
				fr.ProjectKind = kind
			}
		}
		if strings.HasSuffix(l, ".sln") || strings.HasSuffix(l, ".csproj") || strings.HasSuffix(l, ".fsproj") {
			if fr.ProjectKind == "" || fr.ProjectKind == "unknown" {
				fr.ProjectKind = "dotnet"
			}
		}
	}
}

// FinalizeProjectRoot sets IsProjectRoot and ProjectKind from marker files and heuristics.
func FinalizeProjectRoot(fr *scan.FolderRecord, enabled bool) {
	if !enabled {
		fr.IsProjectRoot = false
		fr.ProjectKind = ""
		return
	}
	if fr.ProjectKind == "" {
		fr.ProjectKind = "unknown"
	}
	if len(fr.MarkerFiles) > 0 {
		fr.IsProjectRoot = true
		return
	}

	if fr.FileCount >= 10 {
		codeExts := 0
		for ext := range fr.Extensions {
			switch ext {
			case ".go", ".py", ".js", ".ts", ".java", ".cs", ".cpp", ".c", ".rs":
				codeExts++
			}
		}
		if codeExts >= 3 {
			fr.IsProjectRoot = true
		}
	}
}
