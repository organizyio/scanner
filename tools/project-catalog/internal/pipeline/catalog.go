package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"scan-projects/internal/catalog"
	"scan-projects/internal/model"
	"scan-projects/internal/scan/filter"
)

// CatalogInput configures catalog-style project discovery.
type CatalogInput struct {
	Roots         []string
	ExtraExcludes []string // merged with filter.DefaultExcludeDirNames
	Verbose       bool
}

// CatalogInventory walks roots, discovers projects, and returns sorted ProjectInfo rows.
func CatalogInventory(in CatalogInput) []model.ProjectInfo {
	excludeMap := make(map[string]struct{})
	for _, ex := range filter.DefaultExcludeDirNames {
		excludeMap[ex] = struct{}{}
	}
	for _, ex := range in.ExtraExcludes {
		excludeMap[ex] = struct{}{}
	}

	logger := catalog.NewLogger(in.Verbose)
	var all []model.ProjectInfo

	for _, root := range in.Roots {
		rootPath := filepath.Clean(root)
		if _, err := os.Stat(rootPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[WARN] Root does not exist: %s\n", rootPath)
			continue
		}
		fmt.Printf("[INFO] Scanning root: %s\n", rootPath)
		paths := catalog.FindProjectsWithProgress(rootPath, excludeMap, logger)
		for _, p := range paths {
			fmt.Printf("  - Found project: %s\n", p)
			all = append(all, catalog.CollectProjectInfo(p, logger))
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Path < all[j].Path
	})
	return all
}
