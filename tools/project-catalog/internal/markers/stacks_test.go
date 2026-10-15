package markers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSharedRegistryUnionAndAlphabeticalPrimary(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "Cargo.toml", "pom.xml", "build.gradle", "poetry.lock", "project.fsproj"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
		if _, ok := MarkerKind(name); !ok {
			t.Fatalf("missing %s", name)
		}
	}
	want := []string{"dotnet", "go", "java", "kotlin", "python", "rust"}
	if got := DetectStacks(root); !reflect.DeepEqual(got, want) {
		t.Fatalf("stacks %v", got)
	}
	if kind, _ := MarkerKind("build.gradle"); kind != "kotlin" {
		t.Fatalf("Gradle conflict: %s", kind)
	}
	if !HasProjectMarker(root) {
		t.Fatal("no project")
	}
}
