// Package filter provides directory exclusion helpers for filesystem scans.
package filter

// DirSet builds a lookup set from directory base names to skip during traversal.
func DirSet(dirNames []string) map[string]bool {
	m := make(map[string]bool, len(dirNames))
	for _, d := range dirNames {
		m[d] = true
	}
	return m
}

// DefaultExcludeDirNames lists common dependency and tooling directories to skip by default.
var DefaultExcludeDirNames = []string{
	".git", "node_modules", ".venv", "venv", "env", "virtualenv",
	"__pycache__", "vendor", ".idea", ".vscode", "Lib", "site-packages",
}
