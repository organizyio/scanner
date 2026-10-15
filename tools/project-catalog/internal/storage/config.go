package storage

import (
	"os"

	"gopkg.in/yaml.v3"
)

func LoadYAML[T any](path string) (T, error) {
	var v T
	b, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	err = yaml.Unmarshal(b, &v)
	return v, err
}

func SaveYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
