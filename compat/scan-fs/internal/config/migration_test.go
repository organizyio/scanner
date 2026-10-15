package config

import (
	"errors"
	"flag"
	"reflect"
	"testing"
)

func TestMigrationFlagMappingAndDefaults(t *testing.T) {
	defaults, err := Parse([]string{"--root", "root"})
	if err != nil || defaults.Format != "plain" || defaults.Workers != 0 || defaults.Hash {
		t.Fatalf("default %+v %v", defaults, err)
	}
	c, err := Parse([]string{"--root", "a", "--root", "b", "--exclude", "*.tmp", "--hash", "--hash-partial", "--sha256", "--sniff-format", "--metadata", "--video", "--phash", "--format", "JSON", "--workers", "2"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--root", "a", "--root", "b", "--exclude", "*.tmp", "--hash", "--hash-partial", "--sha256", "--format", "--metadata", "--video", "--phash"}
	if !reflect.DeepEqual(c.EngineArgs(), want) || c.Format != "json" {
		t.Fatalf("mapping %+v", c)
	}
	if _, err := Parse([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"--bad"}, {"--root", "a", "--format", "nope"}} {
		if _, err := Parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
