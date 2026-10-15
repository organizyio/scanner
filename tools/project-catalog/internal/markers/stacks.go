package markers

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	"python": {
		Filenames: map[string]struct{}{
			"pyproject.toml":   {},
			"requirements.txt": {},
			"setup.cfg":        {},
			"setup.py":         {},
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

// HasProjectMarker reports whether dir path contains catalog project markers or stack markers.
func HasProjectMarker(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	filenames := make(map[string]struct{})
	exts := make(map[string]struct{})

	for _, entry := range entries {
		name := entry.Name()
		filenames[name] = struct{}{}

		if _, ok := CatalogRootFilenames[name]; ok {
			return true
		}

		if entry.Type().IsRegular() || entry.Type().Type() == fs.ModeDir {
			ext := filepath.Ext(name)
			if ext != "" {
				exts[ext] = struct{}{}
			}
		}
	}

	for _, m := range StackMarkers {
		for fn := range m.Filenames {
			if _, ok := filenames[fn]; ok {
				return true
			}
		}
		for ext := range m.Extensions {
			if _, ok := exts[ext]; ok {
				return true
			}
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

	filenames := make(map[string]struct{})
	exts := make(map[string]struct{})

	for _, entry := range entries {
		name := entry.Name()
		filenames[name] = struct{}{}

		if entry.Type().IsRegular() || entry.Type().Type() == fs.ModeDir {
			ext := filepath.Ext(name)
			if ext != "" {
				exts[ext] = struct{}{}
			}
		}
	}

	var stacks []string
	for stack, m := range StackMarkers {
		found := false

		if len(m.Filenames) > 0 {
			for fn := range m.Filenames {
				if _, ok := filenames[fn]; ok {
					found = true
					break
				}
			}
		}

		if !found && len(m.Extensions) > 0 {
			for ext := range m.Extensions {
				if _, ok := exts[ext]; ok {
					found = true
					break
				}
			}
		}

		if found {
			stacks = append(stacks, stack)
		}
	}

	sort.Strings(stacks)
	return stacks
}
