package storage

import (
	"errors"
)

type ScanRoot struct {
	Path    string `yaml:"path"`
	Enabled bool   `yaml:"enabled"`
}

type ScanProfile struct {
	Name              string     `yaml:"name"`
	ScanRoots         []ScanRoot `yaml:"scan_roots"`
	ExcludeDirs       []string   `yaml:"exclude_dirs"`
	MaxFileSizeMB     int64      `yaml:"max_file_size_mb"`
	HashFiles         bool       `yaml:"hash_files"`
	HashMaxFileSizeMB int64      `yaml:"hash_max_file_size_mb"`
	ProjectDetection  bool       `yaml:"project_detection"`

	CheckpointEnabled bool   `yaml:"checkpoint_enabled"`
	CheckpointPath    string `yaml:"checkpoint_path"`
}

type AppConfig struct {
	ActiveProfile string                 `yaml:"active_profile"`
	Profiles      map[string]ScanProfile `yaml:"profiles"`
}

func DefaultConfig(checkpointPath string) AppConfig {
	return AppConfig{
		ActiveProfile: "Default",
		Profiles: map[string]ScanProfile{
			"Default": {
				Name: "Default",
				ScanRoots: []ScanRoot{
					{Path: "D:\\", Enabled: true},
					{Path: "E:\\", Enabled: true},
				},
				ExcludeDirs: []string{
					"$RECYCLE.BIN",
					"System Volume Information",
					".git",
					"node_modules",
					"__pycache__",
				},
				MaxFileSizeMB:     0,
				HashFiles:         true,
				HashMaxFileSizeMB: 200,
				ProjectDetection:  true,
				CheckpointEnabled: true,
				CheckpointPath:    checkpointPath,
			},
		},
	}
}

func (c *AppConfig) EnsureValid() error {
	if c.Profiles == nil || len(c.Profiles) == 0 {
		return errors.New("no profiles")
	}
	if _, ok := c.Profiles[c.ActiveProfile]; !ok {
		for name := range c.Profiles {
			c.ActiveProfile = name
			break
		}
	}
	return nil
}
