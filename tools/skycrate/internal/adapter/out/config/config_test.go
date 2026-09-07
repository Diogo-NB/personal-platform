package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `bucket: skycrate-storage
categories:
  "Back Ups":
    tier: GLACIER
  "Back Ups / University":
    tier: DEEP_ARCHIVE
`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Bucket != "skycrate-storage" || len(got.Categories) != 2 {
		t.Fatalf("Load() = %#v", got)
	}
	if got.Categories["back ups"] != "GLACIER" {
		t.Errorf("raw category mapping was changed: %#v", got.Categories)
	}
}

func TestLoadRejectsInvalidConfigBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "missing bucket", contents: "categories:\n  backup:\n    tier: GLACIER\n", want: "bucket must not be empty"},
		{name: "bucket whitespace", contents: "bucket: ' bucket '\ncategories:\n  backup:\n    tier: GLACIER\n", want: "bucket must not contain surrounding whitespace"},
		{name: "missing categories", contents: "bucket: bucket\n", want: "categories must not be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(writeConfig(t, test.contents))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadDefaultPath(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	directory := filepath.Join(configHome, "skycrate")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	path := filepath.Join(directory, "config.yaml")
	if err := os.WriteFile(path, []byte("bucket: bucket\ncategories:\n  backup:\n    tier: GLACIER\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	gotPath, err := DefaultPath()
	if err != nil || gotPath != path {
		t.Fatalf("DefaultPath() = %q, %v", gotPath, err)
	}
	if _, err := Load(""); err != nil {
		t.Fatalf("Load(default) error: %v", err)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
