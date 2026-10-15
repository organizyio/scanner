package markers

import (
	"os"
	"sort"
	"strings"
)

// StackMarker holds detection markers for a language or technology stack.
type StackMarker struct {
	Filenames  map[string]struct{}
	Extensions map[string]struct{}
}

// CatalogRootFilenames are generic root-level files that indicate a project for catalog-style discovery.
var CatalogRootFilenames = map[string]struct{}{
	"pyproject.toml":      {},
	"requirements.txt":    {},
	"setup.cfg":           {},
	"setup.py":            {},
	"package.json":        {},
	"Dockerfile":          {},
	"docker-compose.yml":  {},
	"Makefile":            {},
	".env":                {},
	"build.gradle":        {},
	"build.gradle.kts":    {},
	"settings.gradle":     {},
	"settings.gradle.kts": {},
	"Package.swift":       {},
}

// StackMarkers lists stack-specific markers used for catalog classification.
var StackMarkers = map[string]StackMarker{
	"go":   {Filenames: map[string]struct{}{"go.mod": {}}},
	"rust": {Filenames: map[string]struct{}{"Cargo.toml": {}}},
	"java": {Filenames: map[string]struct{}{"pom.xml": {}}},
	"python": {
		Filenames: map[string]struct{}{
			"pyproject.toml":   {},
			"requirements.txt": {},
			"setup.cfg":        {},
			"setup.py":         {},
			"poetry.lock":      {},
		},
		Extensions: map[string]struct{}{
			".py": {},
		},
	},
	"node": {
		Filenames: map[string]struct{}{
			"package.json": {},
		},
		Extensions: map[string]struct{}{},
	},
	"dotnet": {
		Filenames: map[string]struct{}{},
		Extensions: map[string]struct{}{
			".sln":    {},
			".csproj": {},
			".fsproj": {},
		},
	},
	"kotlin": {
		Filenames: map[string]struct{}{
			"build.gradle":        {},
			"build.gradle.kts":    {},
			"settings.gradle":     {},
			"settings.gradle.kts": {},
		},
		Extensions: map[string]struct{}{
			".kt": {},
		},
	},
	"swift": {
		Filenames: map[string]struct{}{
			"Package.swift": {},
		},
		Extensions: map[string]struct{}{
			".swift":       {},
			".xcodeproj":   {},
			".xcworkspace": {},
		},
	},
}

// MarkerKind is the shared case-insensitive registry for catalog and deep scans.
func MarkerKind(name string) (string, bool) {
	name = strings.ToLower(name)
	if name == ".git" {
		return "git", true
	}
	for stack, marker := range StackMarkers {
		for f := range marker.Filenames {
			if strings.ToLower(f) == name {
				return stack, true
			}
		}
		for ext := range marker.Extensions {
			if strings.HasSuffix(name, ext) {
				return stack, true
			}
		}
	}
	for f := range CatalogRootFilenames {
		if strings.ToLower(f) == name {
			return "unknown", true
		}
	}
	return "", false
}

// HasProjectMarker reports whether dir path contains catalog project markers or stack markers.
func HasProjectMarker(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if kind, ok := MarkerKind(entry.Name()); ok && kind != "git" {
			return true
		}
	}
	return false
}

// DetectStacks returns sorted stack names for a project directory.
func DetectStacks(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	found := map[string]bool{}
	for _, entry := range entries {
		if kind, ok := MarkerKind(entry.Name()); ok && kind != "git" && kind != "unknown" {
			found[kind] = true
		}
	}
	var stacks []string
	for kind := range found {
		stacks = append(stacks, kind)
	}
	sort.Strings(stacks)
	return stacks
}
