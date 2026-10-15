// Package gitmeta fills model.ProjectInfo from git subprocess output (no catalog/UI deps).
package gitmeta

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"scan-projects/internal/model"
)

// Logger receives optional debug lines when git commands fail.
type Logger interface {
	Debugf(format string, args ...interface{})
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Discovery must not refresh repository index caches while reading status.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func parseRemoteURL(url string) (host, owner, repoSlug *string) {
	if url == "" {
		return nil, nil, nil
	}

	url = strings.TrimSuffix(url, "/")

	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, "@", 2)
		if len(parts) != 2 {
			return nil, nil, nil
		}
		right := parts[1]

		lastColon := strings.LastIndex(right, ":")
		if lastColon == -1 {
			return nil, nil, nil
		}

		hostPart := right[:lastColon]
		path := right[lastColon+1:]

		hostParts := strings.SplitN(hostPart, ":", 2)
		h := hostParts[0]

		path = strings.TrimPrefix(path, "/")
		p := strings.SplitN(path, "/", 2)
		if len(p) != 2 {
			return &h, nil, nil
		}
		o := p[0]
		r := strings.TrimSuffix(p[1], ".git")
		return &h, &o, &r
	}

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		noProto := strings.SplitN(url, "://", 2)
		if len(noProto) != 2 {
			return nil, nil, nil
		}

		parts := strings.SplitN(noProto[1], "/", 2)
		if len(parts) < 1 {
			return nil, nil, nil
		}

		hostParts := strings.SplitN(parts[0], ":", 2)
		h := hostParts[0]

		if len(parts) < 2 {
			return &h, nil, nil
		}

		path := strings.TrimPrefix(parts[1], "/")
		segments := strings.Split(path, "/")
		if len(segments) < 2 {
			return &h, nil, nil
		}
		o := segments[0]
		r := strings.TrimSuffix(segments[1], ".git")
		return &h, &o, &r
	}

	return nil, nil, nil
}

// Enrich populates Git-related fields on info for a repository at path.
func Enrich(path string, info *model.ProjectInfo, logger Logger) {
	info.IsGit = true
	info.HasOrigin = false
	info.Remotes = []model.RemoteInfo{}
	info.Host = nil
	info.Owner = nil
	info.RepoSlug = nil
	info.CurrentBranch = ""
	info.LocalBranches = []string{}
	info.TrackingBranch = ""
	info.Dirty = nil

	remotesOut, err := runGit(path, "remote", "-v")
	if err != nil && logger != nil {
		logger.Debugf("Failed to get remotes for %s: %v", path, err)
	}
	if err == nil && remotesOut != "" {
		lines := strings.Split(remotesOut, "\n")
		seen := make(map[string]struct{})

		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			name := fields[0]
			url := fields[1]
			key := name + "|" + url
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			info.Remotes = append(info.Remotes, model.RemoteInfo{Name: name, URL: url})

			if name == "origin" {
				info.HasOrigin = true
				h, o, r := parseRemoteURL(url)
				if h != nil {
					info.Host = h
				}
				if o != nil {
					info.Owner = o
				}
				if r != nil {
					info.RepoSlug = r
				}
			}
		}
	}

	cur, err := runGit(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil && logger != nil {
		logger.Debugf("Failed to get current branch for %s: %v", path, err)
	}
	if err == nil && cur != "" {
		info.CurrentBranch = cur
	}

	branchesOut, err := runGit(path, "branch", "--format", "%(refname:short)")
	if err != nil && logger != nil {
		logger.Debugf("Failed to get branches for %s: %v", path, err)
	}
	if err == nil && branchesOut != "" {
		lineScan := bufio.NewScanner(strings.NewReader(branchesOut))
		for lineScan.Scan() {
			line := strings.TrimSpace(lineScan.Text())
			if line != "" {
				info.LocalBranches = append(info.LocalBranches, line)
			}
		}
	}

	if info.CurrentBranch != "" {
		tracking, err := runGit(path, "rev-parse", "--abbrev-ref", info.CurrentBranch+"@{upstream}")
		if err != nil && logger != nil {
			logger.Debugf("Failed to get tracking branch for %s: %v", path, err)
		}
		if err == nil && tracking != "" {
			info.TrackingBranch = tracking
		}
	}

	statusOut, err := runGit(path, "status", "--porcelain")
	if err != nil && logger != nil {
		logger.Debugf("Failed to get status for %s: %v", path, err)
	}
	d := strings.TrimSpace(statusOut) != ""
	info.Dirty = &d
}
