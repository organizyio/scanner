package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Checkpoint struct {
	LastPath  string `json:"last_path"`
	UpdatedAt string `json:"updated_at"`
}

func Load(path string) (Checkpoint, bool) {
	var cp Checkpoint
	b, err := os.ReadFile(path)
	if err != nil {
		return cp, false
	}
	if err := json.Unmarshal(b, &cp); err != nil {
		return cp, false
	}
	return cp, true
}

func Save(path, lastPath string) error {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	cp := Checkpoint{
		LastPath:  lastPath,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(cp, "", "  ")
	return os.WriteFile(path, b, 0644)
}
