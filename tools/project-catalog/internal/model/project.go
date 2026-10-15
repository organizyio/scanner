// Package model defines data structures for project information.
package model

// RemoteInfo describes a git remote with its name and URL.
type RemoteInfo struct {
	Name string `json:"name"` // Remote name (e.g., "origin")
	URL  string `json:"url"`  // Remote URL
}

// ProjectInfo describes a detected project, which may or may not be a Git repository.
// It includes project metadata such as language stacks, Git information, and paths.
type ProjectInfo struct {
	Name           string       `json:"name"`            // Project directory name
	Path           string       `json:"path"`            // Full path to project directory
	Stacks         []string     `json:"stacks"`          // Detected language stacks (e.g., ["python", "node"])
	PrimaryStack   string       `json:"primary_stack"`   // Primary/dominant stack
	IsGit          bool         `json:"is_git"`          // Whether this is a Git repository
	HasOrigin      bool         `json:"has_origin"`      // Whether origin remote exists
	Remotes        []RemoteInfo `json:"remotes"`         // All Git remotes
	Host           *string      `json:"host"`            // Git host from origin (e.g., "github.com")
	Owner          *string      `json:"owner"`           // Repository owner from origin (e.g., "carrio-org")
	RepoSlug       *string      `json:"repo_slug"`       // Repository name from origin (e.g., "carrio-api")
	CurrentBranch  string       `json:"current_branch"`  // Current Git branch
	LocalBranches  []string     `json:"local_branches"`  // All local branch names
	TrackingBranch string       `json:"tracking_branch"` // Upstream tracking branch
	Dirty          *bool        `json:"dirty"`           // Whether working directory has uncommitted changes (nil if unknown)
}
