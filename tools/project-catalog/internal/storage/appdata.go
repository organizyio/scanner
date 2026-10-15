package storage

import (
	"os"
	"path/filepath"
)

const AppName = "ScannerApp"

// AppDataDir returns the per-user application data directory (no Fyne dependency).
func AppDataDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", AppName)
	}
	return filepath.Join(base, AppName)
}

// SettingsPath returns the path to the main settings YAML file.
func SettingsPath() string {
	return filepath.Join(AppDataDir(), "settings.yaml")
}
