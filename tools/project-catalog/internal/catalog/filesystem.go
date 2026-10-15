// Package catalog discovers project directories and builds inventory metadata (Git, stacks).
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"scan-projects/internal/gitmeta"
	"scan-projects/internal/markers"
	"scan-projects/internal/model"
)

// FindProjectsWithProgress scans a root directory recursively to find all projects,
// displaying a progress bar during the scan. A project is identified by the presence
// of a .git directory or project marker files. Returns a slice of project paths.
// excludeMap contains directory names to skip during scanning.
// If logger is provided, debug information will be logged when verbose mode is enabled.
func FindProjectsWithProgress(root string, excludeMap map[string]struct{}, logger *Logger) []string {
	var projects []string
	var totalDirs int

	pb := NewProgressBar(0) // Will update total dynamically

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Error accessing %s: %v\n", path, err)
			if logger != nil {
				logger.Debugf("Skipping directory due to error: %s", path)
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}

		dirName := d.Name()
		if _, excluded := excludeMap[dirName]; excluded {
			if logger != nil {
				logger.Debugf("Skipping excluded directory: %s", path)
			}
			return filepath.SkipDir
		}

		checkPath := path
		rootClean := filepath.Clean(root)
		for {
			component := filepath.Base(checkPath)
			checkPathClean := filepath.Clean(checkPath)

			if component == "." || component == "" {
				break
			}
			if checkPathClean == rootClean {
				break
			}

			if _, excluded := excludeMap[component]; excluded {
				if logger != nil {
					logger.Debugf("Skipping directory inside excluded parent: %s (excluded: %s)", path, component)
				}
				return filepath.SkipDir
			}

			parent := filepath.Dir(checkPath)
			parentClean := filepath.Clean(parent)
			if parentClean == checkPathClean || parentClean == rootClean {
				break
			}
			checkPath = parent
		}

		totalDirs++
		pb.Total = totalDirs

		isProject := false
		gitDir := filepath.Join(path, ".git")
		hasGit := false
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			projects = append(projects, path)
			isProject = true
			hasGit = true
			if logger != nil {
				logger.Debugf("Found Git project: %s", path)
			}
		} else if markers.HasProjectMarker(path) {
			isInsideProject := false
			checkPath := filepath.Dir(path)
			rootClean := filepath.Clean(root)
			for {
				checkPathClean := filepath.Clean(checkPath)
				if checkPathClean == rootClean {
					break
				}
				parentGitDir := filepath.Join(checkPath, ".git")
				if info, err := os.Stat(parentGitDir); err == nil && info.IsDir() {
					isInsideProject = true
					break
				}
				if markers.HasProjectMarker(checkPath) {
					isInsideProject = true
					break
				}
				parent := filepath.Dir(checkPath)
				parentClean := filepath.Clean(parent)
				if parentClean == checkPathClean || parentClean == rootClean {
					break
				}
				checkPath = parent
			}

			if !isInsideProject {
				projects = append(projects, path)
				isProject = true
				if logger != nil {
					logger.Debugf("Found project with markers: %s", path)
				}
			} else {
				if logger != nil {
					logger.Debugf("Skipping subdirectory inside project: %s", path)
				}
				return filepath.SkipDir
			}
		}

		pb.Increment(isProject)

		if isProject && !hasGit {
			return filepath.SkipDir
		}

		return nil
	})

	return projects
}

// CollectProjectInfo builds a ProjectInfo struct for a given directory path.
func CollectProjectInfo(path string, logger *Logger) model.ProjectInfo {
	name := filepath.Base(path)
	stacks := markers.DetectStacks(path)
	var primary string
	if len(stacks) > 0 {
		primary = stacks[0]
	}

	p := model.ProjectInfo{
		Name:           name,
		Path:           path,
		Stacks:         stacks,
		PrimaryStack:   primary,
		IsGit:          false,
		HasOrigin:      false,
		Remotes:        []model.RemoteInfo{},
		Host:           nil,
		Owner:          nil,
		RepoSlug:       nil,
		CurrentBranch:  "",
		LocalBranches:  []string{},
		TrackingBranch: "",
		Dirty:          nil,
	}

	gitDir := filepath.Join(path, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		if logger != nil {
			logger.Debugf("Collecting Git info for: %s", path)
		}
		gitmeta.Enrich(path, &p, logger)
	}

	return p
}
