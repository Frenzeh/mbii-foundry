package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildExportManifest(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source")
	os.Mkdir(source, 0755)

	os.WriteFile(filepath.Join(source, "file1.txt"), []byte("data"), 0644)
	os.Mkdir(filepath.Join(source, "subdir"), 0755)
	os.WriteFile(filepath.Join(source, "subdir", "file2.txt"), []byte("data2"), 0644)
	os.Mkdir(filepath.Join(source, ".hidden"), 0755)
	os.WriteFile(filepath.Join(source, ".hidden", "hidden.txt"), []byte("hidden"), 0644)
	os.WriteFile(filepath.Join(source, ".hiddenfile"), []byte("hidden"), 0644)

	dest := filepath.Join(source, "output.pk3")

	manifest, err := BuildExportManifest(source, dest)
	if err != nil {
		t.Fatalf("BuildExportManifest failed: %v", err)
	}

	if len(manifest) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(manifest))
	}

	for _, entry := range manifest {
		if entry.ArchivePath == "output.pk3" {
			t.Errorf("output.pk3 should be excluded")
		}
		if entry.ArchivePath == ".hiddenfile" || entry.ArchivePath == ".hidden/hidden.txt" {
			t.Errorf("hidden files should be excluded")
		}
	}
}

func TestWritePK3(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source")
	os.Mkdir(source, 0755)
	os.WriteFile(filepath.Join(source, "test.txt"), []byte("hello pk3"), 0644)

	dest := filepath.Join(tempDir, "out.pk3")
	manifest := []ExportEntry{
		{SourcePath: filepath.Join(source, "test.txt"), ArchivePath: "test.txt"},
	}

	if err := WritePK3(source, dest, manifest); err != nil {
		t.Fatalf("WritePK3 failed: %v", err)
	}

	r, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("failed to open pk3: %v", err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("expected 1 file in zip, got %d", len(r.File))
	}

	if r.File[0].Name != "test.txt" {
		t.Errorf("expected test.txt, got %s", r.File[0].Name)
	}

	rc, _ := r.File[0].Open()
	data, _ := io.ReadAll(rc)
	rc.Close()
	if string(data) != "hello pk3" {
		t.Errorf("expected 'hello pk3', got '%s'", string(data))
	}
}

func TestWritePK3Security(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source")
	os.MkdirAll(source, 0755)

	dest := filepath.Join(tempDir, "out.pk3")

	// Create a dummy file
	os.WriteFile(filepath.Join(source, "file.txt"), []byte("data"), 0644)

	// Test traversal in SourcePath
	t.Run("TraversalSource", func(t *testing.T) {
		manifest := []ExportEntry{
			{SourcePath: "../escape.txt", ArchivePath: "file.txt"},
		}
		err := WritePK3(source, dest, manifest)
		if err == nil {
			t.Errorf("expected error for traversal source")
		}
	})

	// Test absolute traversal out of root
	t.Run("AbsoluteEscapeSource", func(t *testing.T) {
		manifest := []ExportEntry{
			{SourcePath: filepath.Join(tempDir, "outside.txt"), ArchivePath: "file.txt"},
		}
		err := WritePK3(source, dest, manifest)
		if err == nil {
			t.Errorf("expected error for absolute escape source")
		}
	})

	// Test traversal in ArchivePath
	t.Run("TraversalArchive", func(t *testing.T) {
		manifest := []ExportEntry{
			{SourcePath: "file.txt", ArchivePath: "../escape.txt"},
		}
		err := WritePK3(source, dest, manifest)
		if err == nil {
			t.Errorf("expected error for traversal archive")
		}
	})

	// Test symlink rejection
	t.Run("SymlinkRejection", func(t *testing.T) {
		symlinkPath := filepath.Join(source, "symlink.txt")
		os.Symlink("file.txt", symlinkPath)
		manifest := []ExportEntry{
			{SourcePath: "symlink.txt", ArchivePath: "symlink.txt"},
		}
		err := WritePK3(source, dest, manifest)
		if err == nil {
			t.Errorf("expected error for symlink source")
		}
	})

	// Test self-output
	t.Run("SelfOutput", func(t *testing.T) {
		selfDest := filepath.Join(source, "self.pk3")
		os.WriteFile(selfDest, []byte("dummy"), 0644)
		manifest := []ExportEntry{
			{SourcePath: "self.pk3", ArchivePath: "self.pk3"},
		}
		err := WritePK3(source, selfDest, manifest)
		if err == nil {
			t.Errorf("expected error for self-output")
		}
	})
}

func TestGitPruning(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source")
	os.MkdirAll(source, 0755)

	gitDir := filepath.Join(source, ".git")
	os.MkdirAll(gitDir, 0755)
	os.WriteFile(filepath.Join(gitDir, "config"), []byte("data"), 0644)

	dest := filepath.Join(tempDir, "out.pk3")

	manifest, err := BuildExportManifest(source, dest)
	if err != nil {
		t.Fatalf("BuildExportManifest failed: %v", err)
	}

	for _, entry := range manifest {
		if strings.Contains(entry.SourcePath, ".git") || strings.Contains(entry.ArchivePath, ".git") {
			t.Errorf("expected .git to be pruned")
		}
	}
}

func TestWritePK3RejectsDuplicateArchivePath(t *testing.T) {
	tempDir := t.TempDir()
	source := filepath.Join(tempDir, "source")
	os.MkdirAll(source, 0755)
	os.WriteFile(filepath.Join(source, "a.txt"), []byte("A"), 0644)
	os.WriteFile(filepath.Join(source, "b.txt"), []byte("B"), 0644)

	dest := filepath.Join(tempDir, "out.pk3")
	manifest := []ExportEntry{
		{SourcePath: filepath.Join(source, "a.txt"), ArchivePath: "a.txt"},
		{SourcePath: filepath.Join(source, "b.txt"), ArchivePath: "a.txt"},
	}
	err := WritePK3(source, dest, manifest)
	if err == nil || !strings.Contains(err.Error(), "duplicate archive path") {
		t.Errorf("expected duplicate archive path rejection, got %v", err)
	}
}

func TestWritePK3RejectsEmptyManifestWithoutReplacingDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "existing.pk3")
	if err := os.WriteFile(dest, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WritePK3(t.TempDir(), dest, nil); err == nil || !strings.Contains(err.Error(), "empty manifest") {
		t.Fatalf("expected empty manifest rejection, got %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep me" {
		t.Fatalf("destination changed on validation failure: %q", got)
	}
}

func TestWritePK3RejectsNonCanonicalAndAliasedPaths(t *testing.T) {
	source := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	dest := filepath.Join(t.TempDir(), "out.pk3")
	badPaths := []string{
		"",
		"/absolute.txt",
		".",
		"./file.txt",
		"a/./file.txt",
		"a/../file.txt",
		"C:/Windows/system.ini",
		"a/../../escape.txt",
		"a//file.txt",
		"a/",
		`a\file.txt`,
		"a/\x01file.txt",
	}
	for _, archivePath := range badPaths {
		t.Run(fmt.Sprintf("%q", archivePath), func(t *testing.T) {
			err := WritePK3(source, dest, []ExportEntry{{
				SourcePath: "a.txt", ArchivePath: archivePath,
			}})
			if err == nil {
				t.Fatalf("unsafe archive path %q accepted", archivePath)
			}
		})
	}

	aliases := [][2]string{
		{"Readme.txt", "README.TXT"},
		{"Caf\u00e9.txt", "Cafe\u0301.txt"},
		{"Stra\u00dfe.txt", "STRASSE.TXT"},
	}
	for _, pair := range aliases {
		t.Run(pair[0], func(t *testing.T) {
			err := WritePK3(source, dest, []ExportEntry{
				{SourcePath: "a.txt", ArchivePath: pair[0]},
				{SourcePath: "b.txt", ArchivePath: pair[1]},
			})
			if err == nil || !strings.Contains(err.Error(), "alias") {
				t.Fatalf("expected alias collision for %q and %q, got %v", pair[0], pair[1], err)
			}
		})
	}
}

func TestWritePK3RejectsUnsafeSourcePathsAndOutputAliases(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real", "file.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "out.pk3")
	for _, sourcePath := range []string{
		"",
		".",
		"./real/file.txt",
		"real/../real/file.txt",
		"real//file.txt",
		`real\file.txt`,
		"real/\x7ffile.txt",
	} {
		t.Run(fmt.Sprintf("%q", sourcePath), func(t *testing.T) {
			err := WritePK3(root, dest, []ExportEntry{{
				SourcePath: sourcePath, ArchivePath: "file.txt",
			}})
			if err == nil {
				t.Fatalf("unsafe source path %q accepted", sourcePath)
			}
		})
	}

	if err := os.Symlink("real", filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	err := WritePK3(root, dest, []ExportEntry{{
		SourcePath: "linked/file.txt", ArchivePath: "file.txt",
	}})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("nested symlink source accepted: %v", err)
	}

	rootLink := filepath.Join(t.TempDir(), "linked-root")
	if err := os.Symlink(root, rootLink); err != nil {
		t.Fatal(err)
	}
	err = WritePK3(rootLink, dest, []ExportEntry{{
		SourcePath: "real/file.txt", ArchivePath: "file.txt",
	}})
	if err == nil || !strings.Contains(err.Error(), "source root") {
		t.Fatalf("symlink source root accepted: %v", err)
	}

	hardlink := filepath.Join(root, "destination-alias.pk3")
	if err := os.WriteFile(dest, []byte("existing destination"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(dest, hardlink); err != nil {
		t.Fatal(err)
	}
	err = WritePK3(root, dest, []ExportEntry{{
		SourcePath: "destination-alias.pk3", ArchivePath: "old.pk3",
	}})
	if err == nil || !strings.Contains(err.Error(), "output file") {
		t.Fatalf("hardlink alias of destination accepted: %v", err)
	}
}
