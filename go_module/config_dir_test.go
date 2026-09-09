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

func TestCopyDirMigratesReadOnlyNestedDirectory(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dst")
	nested := filepath.Join(src, "read-only")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "config.json"), []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(nested, 0500); err != nil {
		t.Skipf("directory modes unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(nested, 0700)
		_ = os.Chmod(filepath.Join(dst, "read-only"), 0700)
	})
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("read-only legacy directory could not migrate: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "read-only", "config.json"))
	if err != nil || string(got) != "preserved" {
		t.Fatalf("migrated nested configuration mismatch: %q err=%v", got, err)
	}
}


func TestMigrationPreservesUnownedPartialDirectory(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, "mbii-fa-creator")
	newDir := filepath.Join(base, "mbii-foundry")
	tmpDir := newDir + ".tmp"

	// Create old dir with a file
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", oldDir, err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "config.json"), []byte("test"), 0644); err != nil {
		t.Fatalf("write %s: %v", oldDir, err)
	}

	// Simulate an interrupted migration by creating the tmpDir with partial contents
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", tmpDir, err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "partial.json"), []byte("partial"), 0644); err != nil {
		t.Fatalf("write %s: %v", tmpDir, err)
	}

	// Call appConfigDirWithBase
	got, err := appConfigDirWithBase(base)
	if err != nil {
		t.Fatalf("appConfigDirWithBase returned error: %v", err)
	}

	// It should succeed and return newDir
	if got != newDir {
		t.Errorf("expected %s, got %s", newDir, got)
	}

	// The newDir should contain the contents from oldDir
	content, err := os.ReadFile(filepath.Join(newDir, "config.json"))
	if err != nil {
		t.Errorf("failed to read migrated file: %v", err)
	} else if string(content) != "test" {
		t.Errorf("expected 'test', got %q", string(content))
	}

	// The partial file should not exist in newDir
	if _, err := os.Stat(filepath.Join(newDir, "partial.json")); !os.IsNotExist(err) {
		t.Errorf("expected partial.json to be missing, but it was found")
	}

	partial, err := os.ReadFile(filepath.Join(tmpDir, "partial.json"))
	if err != nil || string(partial) != "partial" {
		t.Fatal("migration modified an unrelated existing staging directory")
	}
}

func TestMigrationFailureRetainsOriginalAndRetries(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, "mbii-fa-creator")
	newDir := filepath.Join(base, "mbii-foundry")
	if err := os.MkdirAll(oldDir, 0700); err != nil { t.Fatal(err) }
	original := []byte(`{"theme":"gold"}`)
	if err := os.WriteFile(filepath.Join(oldDir, "config.json"), original, 0600); err != nil { t.Fatal(err) }
	link := filepath.Join(oldDir, "linked.json")
	if err := os.Symlink("config.json", link); err != nil { t.Skipf("symlink unavailable: %v", err) }
	got, err := appConfigDirWithBase(base)
	if err == nil || got != oldDir { t.Fatal("incomplete migration was not reported with the intact legacy fallback") }
	if _, err := os.Stat(newDir); !os.IsNotExist(err) { t.Fatal("failed migration published a partial configuration") }
	staging, globErr := filepath.Glob(filepath.Join(base, "mbii-foundry-migration-*"))
	if globErr != nil || len(staging) != 0 {
		t.Fatalf("failed migration left owned staging data behind: paths=%v err=%v", staging, globErr)
	}
	retained, err := os.ReadFile(filepath.Join(oldDir, "config.json"))
	if err != nil || string(retained) != string(original) {
		t.Fatal("failed migration changed the original")
	}
	fallbackWrite := []byte("written while migration warning was visible")
	if err := os.WriteFile(filepath.Join(got, "fallback-state.json"), fallbackWrite, 0600); err != nil {
		t.Fatalf("returned legacy fallback was not usable: %v", err)
	}
	if err := os.Remove(link); err != nil { t.Fatal(err) }
	got, err = appConfigDirWithBase(base)
	if err != nil || got != newDir { t.Fatalf("migration could not retry: %v", err) }
	migrated, err := os.ReadFile(filepath.Join(newDir, "config.json"))
	if err != nil || string(migrated) != string(original) { t.Fatal("retry did not preserve the original contents") }
	migratedFallback, err := os.ReadFile(filepath.Join(newDir, "fallback-state.json"))
	if err != nil || string(migratedFallback) != string(fallbackWrite) {
		t.Fatal("retry did not migrate state written through the usable fallback")
	}
}

func TestConfigDirectoryCannotBeFile(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "mbii-foundry")
	original := []byte("unrelated file")
	if err := os.WriteFile(path, original, 0600); err != nil { t.Fatal(err) }
	directory, err := appConfigDirWithBase(base)
	if err == nil || directory != "" {
		t.Fatal("non-directory configuration location was accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(original) {
		t.Fatal("unrelated file at configuration location was modified")
	}
}

func TestConfigDirectoryRejectsEmptyBase(t *testing.T) {
	directory, err := appConfigDirWithBase("")
	if err == nil || directory != "" {
		t.Fatal("empty configuration base was joined into the working directory")
	}
}

func TestCopyDirRejectsEmptyPaths(t *testing.T) {
	if err := copyDir("", t.TempDir()); err == nil {
		t.Fatal("empty migration source was accepted")
	}
	if err := copyDir(t.TempDir(), ""); err == nil {
		t.Fatal("empty migration destination was accepted")
	}
}
