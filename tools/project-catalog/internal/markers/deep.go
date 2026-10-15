// Package markers holds shared filename conventions for project detection across scan and catalog.
package markers

// DeepFileKind maps lowercase file or directory names to a coarse technology kind.
// Scan records marker filenames only; internal/analysis infers ProjectKind from MarkerFiles.
var DeepFileKind = map[string]string{
	".git":             "git",
	"go.mod":           "go",
	"package.json":     "node",
	"pyproject.toml":   "python",
	"requirements.txt": "python",
	"poetry.lock":      "python",
	"cargo.toml":       "rust",
	"pom.xml":          "java",
	"build.gradle":     "java",
	"build.gradle.kts": "java",
}
