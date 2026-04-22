package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCopyDirBasic verifies the migration copy logic handles a typical
// config layout: a few files, nested directories, varied modes.
func TestCopyDirBasic(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")

	// Populate src with a mix of files and nested dirs.
	files := map[string]string{
		"config.json":                 `{"key":"value"}`,
		"favorites.json":              `["/path/one","/path/two"]`,
		"backups/older/file.bak.mbch": "old backup",
		"logs/app.log":                "log line",
	}
	for rel, content := range files {
		full := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Every file should be present with identical content in dst.
	for rel, expected := range files {
		got, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("missing %s after copy: %v", rel, err)
			continue
		}
		if string(got) != expected {
			t.Errorf("content mismatch for %s: got %q, want %q", rel, string(got), expected)
		}
	}
}

// TestCopyDirEmptyDir verifies the migration doesn't trip on empty
// directories (users who created the old dir but never saved anything).
func TestCopyDirEmptyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir on empty source failed: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("dst not created: %v", err)
	}
}
