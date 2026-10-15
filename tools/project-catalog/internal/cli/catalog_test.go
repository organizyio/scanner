package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCanceledCatalogWritesPartialJSONAndReturnsInterrupt(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "partial.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := runCatalog(ctx, []string{"--root", root, "--output", output}); code != 130 {
		t.Fatalf("interrupted catalog returned %d", code)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var projects []json.RawMessage
	if err := json.Unmarshal(data, &projects); err != nil {
		t.Fatalf("partial inventory is not JSON: %v", err)
	}
	if len(projects) != 0 {
		t.Fatal("catalog continued scanning after cancellation")
	}
}

func TestCompletedCatalogKeepsSuccessExit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "complete.json")
	if code := runCatalog(context.Background(), []string{"--root", root, "--output", output}); code != 0 {
		t.Fatalf("completed catalog returned %d", code)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var projects []json.RawMessage
	if err := json.Unmarshal(data, &projects); err != nil || len(projects) != 1 {
		t.Fatalf("complete inventory: %v %s", err, data)
	}
}
