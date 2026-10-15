// Package gitmeta fills model.ProjectInfo from git subprocess output (no catalog/UI deps).
package gitmeta

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"scan-projects/internal/model"
)

// Logger receives optional debug lines when git commands fail.
type Logger interface {
	Debugf(format string, args ...interface{})
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args = append([]string{"-c", "core.fsmonitor=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.WaitDelay = 100 * time.Millisecond
	cmd.Dir = dir
	// Discovery must not refresh repository index caches while reading status.
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("git inspection failed: %w", err)
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
	EnrichContext(context.Background(), path, info, logger)
}

func EnrichContext(ctx context.Context, path string, info *model.ProjectInfo, logger Logger) {
	if logger != nil {
		v := reflect.ValueOf(logger)
		switch v.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
			if v.IsNil() {
				logger = nil
			}
		}
	}
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

	remotesOut, err := runGit(ctx, path, "remote", "-v")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Partial Git inspection (remotes): %s: %v\n", path, err)
		if logger != nil {
			logger.Debugf("Failed to get remotes for %s: %v", path, err)
		}
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
			url := SanitizeRemoteURL(fields[1])
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

	cur, err := runGit(ctx, path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Partial Git inspection (current branch): %s: %v\n", path, err)
		if logger != nil {
			logger.Debugf("Failed to get current branch for %s: %v", path, err)
		}
	}
	if err == nil && cur != "" {
		info.CurrentBranch = cur
	}

	branchesOut, err := runGit(ctx, path, "branch", "--format", "%(refname:short)")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Partial Git inspection (branches): %s: %v\n", path, err)
		if logger != nil {
			logger.Debugf("Failed to get branches for %s: %v", path, err)
		}
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
		tracking, err := runGit(ctx, path, "rev-parse", "--abbrev-ref", info.CurrentBranch+"@{upstream}")
		if err != nil && logger != nil {
			logger.Debugf("Failed to get tracking branch for %s: %v", path, err)
		}
		if err == nil && tracking != "" {
			info.TrackingBranch = tracking
		}
	}

	statusOut, err := runGit(ctx, path, "status", "--porcelain")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Partial Git inspection: %s: %v\n", path, err)
		if logger != nil {
			logger.Debugf("Failed to get status for %s: %v", path, err)
		}
	}
	if err == nil {
		d := strings.TrimSpace(statusOut) != ""
		info.Dirty = &d
	}
}

// IsRepository validates gitdir files without changing repository configuration.
func IsRepository(ctx context.Context, path string) bool {
	st, err := os.Stat(path + string(os.PathSeparator) + ".git")
	if err != nil {
		return false
	}
	if st.IsDir() {
		return true
	}
	if !st.Mode().IsRegular() {
		return false
	}
	out, err := runGit(ctx, path, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// SanitizeRemoteURL removes secrets without echoing malformed URL input.
func SanitizeRemoteURL(raw string) string {
	if !strings.Contains(raw, "://") {
		// Windows drive paths are local paths, not a host/password separator.
		if len(raw) >= 3 && ((raw[0] >= 'a' && raw[0] <= 'z') || (raw[0] >= 'A' && raw[0] <= 'Z')) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') && !strings.ContainsAny(raw, "@?#") {
			return raw
		}
		clean := raw
		if i := strings.IndexAny(clean, "?#"); i >= 0 {
			clean = clean[:i]
		}
		if colon := strings.IndexByte(clean, ':'); colon > 0 && colon < len(clean)-1 {
			left, right := clean[:colon], clean[colon+1:]
			// SCP forms support optional SSH usernames, never user:password@host.
			if !strings.Contains(right, "@") && !strings.ContainsAny(left, "/\\") && strings.Count(left, "@") <= 1 && !strings.ContainsAny(right, "\r\n") {
				switch strings.ToLower(left) {
				case "http", "https", "ssh", "file", "ftp":
					return "[redacted remote]"
				}
				return clean
			}
		}
		if !strings.ContainsAny(raw, "@?#:") {
			return raw
		}
		return "[redacted remote]"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Host == "" && !(strings.EqualFold(u.Scheme, "file") && u.Path != "" && strings.HasPrefix(u.Path, "/"))) {
		return "[redacted remote]"
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	if strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https") {
		u.User = nil
	} else if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}
