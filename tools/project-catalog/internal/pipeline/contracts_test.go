package pipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"scan-projects/internal/scan"
	"sort"
	"testing"
)

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err = json.Unmarshal(b, &object); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(object))
	for k := range object {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func TestCatalogAndDeepGoldenShapes(t *testing.T) {
	b, err := os.ReadFile("../../../../docs/follow-up/baseline-contracts.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Catalog []map[string]any `json:"catalog"`
		Deep    struct {
			Files   []map[string]any          `json:"files"`
			Folders map[string]map[string]any `json:"folders"`
		} `json:"deep"`
	}
	if err = json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	put(t, filepath.Join(root, "node/package.json"), "{}")
	put(t, filepath.Join(root, "hello.txt"), "abc")
	catalog := CatalogInventory(CatalogInput{Roots: []string{root}})
	if len(catalog) != 1 || !reflect.DeepEqual(jsonKeys(t, catalog[0]), jsonKeys(t, golden.Catalog[0])) {
		t.Fatalf("catalog shape changed: %+v", catalog)
	}
	deep, err := DeepScan(context.Background(), DeepScanInput{Roots: []string{root}, Options: scan.ScanOptions{ProjectDetection: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(deep.Files) != 2 || !reflect.DeepEqual(jsonKeys(t, deep.Files[0]), jsonKeys(t, golden.Deep.Files[0])) {
		t.Fatalf("deep file shape changed: %+v", deep.Files)
	}
	for _, folder := range deep.Folders {
		for _, expected := range golden.Deep.Folders {
			if !reflect.DeepEqual(jsonKeys(t, folder), jsonKeys(t, expected)) {
				t.Fatal("folder shape changed")
			}
			break
		}
	}
}
