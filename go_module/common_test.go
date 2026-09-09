package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileManager_Backups(t *testing.T) {
	tmpDir := t.TempDir()
	fm := NewFileManager(tmpDir)

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(dir2, 0755)

	file1 := filepath.Join(dir1, "test.mbch")
	file2 := filepath.Join(dir2, "test.mbch")
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	// Test rapid same-path backup uniqueness
	// We force the maximum backups to ensure it cycles properly and tests uniqueness.
	var file1Backups []string
	for i := 0; i < 7; i++ {
		b, err := fm.CreateBackup(file1)
		if err != nil {
			t.Fatal(err)
		}
		file1Backups = append(file1Backups, b)
	}

	// Verify all returned backup paths from rapid loop are unique
	uniqueBackups := make(map[string]bool)
	for _, b := range file1Backups {
		if uniqueBackups[b] {
			t.Fatalf("Duplicate backup created: %s", b)
		}
		uniqueBackups[b] = true
	}

	// Because MaxBackupsPerFile is 5, there should only be 5 backups kept for file1
	list1 := fm.ListBackups(file1)
	if len(list1) != MaxBackupsPerFile {
		t.Errorf("expected %d backups for file1, got %d", MaxBackupsPerFile, len(list1))
	}

	// Test observable cross-project retention
	// We place a legacy backup for file2 (or just an unrelated file named test_legacy.mbch)
	// and ensure that creating backups for file1 doesn't prune them.
	legacyBackupPath := filepath.Join(fm.getBackupPath(), "test_20200101_120000.mbch")
	os.MkdirAll(fm.getBackupPath(), 0755)
	os.WriteFile(legacyBackupPath, []byte("legacy bytes"), 0644)

	// Another legacy/unrelated file
	otherLegacyPath := filepath.Join(fm.getBackupPath(), "test_20210101_120000.mbch")
	os.WriteFile(otherLegacyPath, []byte("more legacy"), 0644)

	// Create backups for file2, this should prune file2's backups but NOT legacy bytes
	for i := 0; i < 6; i++ {
		_, err := fm.CreateBackup(file2)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Legacy bytes must be 100% untouched
	legacyData, err := os.ReadFile(legacyBackupPath)
	if err != nil || string(legacyData) != "legacy bytes" {
		t.Errorf("Legacy backup was pruned or corrupted!")
	}
	otherLegacyData, err := os.ReadFile(otherLegacyPath)
	if err != nil || string(otherLegacyData) != "more legacy" {
		t.Errorf("Other legacy backup was pruned or corrupted!")
	}

	// Restore test
	restoreDst := filepath.Join(tmpDir, "restore.mbch")
	if err := fm.RestoreBackup(list1[0], restoreDst); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(restoreDst)
	if string(restored) != "content1" {
		t.Errorf("expected content1, got %s", string(restored))
	}

	// Test restore error
	err = fm.RestoreBackup("nonexistent", restoreDst)
	if err == nil {
		t.Errorf("expected error restoring nonexistent backup")
	}

	// Test no-replace final publication
	// Predict what the base name for file2's next backup would be (it uses current second)
	// We'll create a colliding file directly in the backup dir to force the EEXIST fallback loop.
	importPath := file2
	// We can't perfectly predict time.Now(), so we'll just create a file, then mock the baseName if possible,
	// BUT actually `CreateBackup` creates a backup right now. Let's just create a collision by reading the time!
	importBase := filepath.Base(importPath)
	importExt := filepath.Ext(importBase)
	importNameWithoutExt := importBase[:len(importBase)-len(importExt)]
	
	// Fast way to guarantee a collision: 
	// CreateBackup will generate something like: name_hash_timestamp.mbch
	// Let's create a backup, get its name, delete it, wait a bit? No, we can just pre-create a file that MATCHES the pattern!
	// Actually, just calling CreateBackup once gives us the exact timestamp it used if we do it fast.
	// But time could tick.
	// Better: Pre-create ALL possible timestamp files for the next 2 seconds!
	importHash := fm.getPathHash(importPath)
	now := time.Now()
	for i := 0; i <= 2; i++ {
		tStr := now.Add(time.Duration(i) * time.Second).Format("20060102_150405")
		predict := filepath.Join(fm.getBackupPath(), importNameWithoutExt+"_"+importHash+"_"+tStr+importExt)
		os.WriteFile(predict, []byte("pre-existing data"), 0644)
	}
	
	// Now CreateBackup MUST find a collision and use the _1 fallback!
	bPath, err := fm.CreateBackup(importPath)
	if err != nil {
		t.Fatalf("CreateBackup failed on collision: %v", err)
	}
	
	// Check that the returned path is the _1 version (or at least different from the base)
	// But more importantly, check that ALL the pre-existing files STILL contain "pre-existing data"!
	for i := 0; i <= 2; i++ {
		tStr := now.Add(time.Duration(i) * time.Second).Format("20060102_150405")
		predict := filepath.Join(fm.getBackupPath(), importNameWithoutExt+"_"+importHash+"_"+tStr+importExt)
		data, err := os.ReadFile(predict)
		if err == nil {
			if string(data) != "pre-existing data" {
				t.Errorf("Pre-existing backup %s was OVERWRITTEN! Data: %s", predict, string(data))
			}
		}
	}
	
	// Check the newly created backup actually contains the correct content2 data
	newData, err := os.ReadFile(bPath)
	if err != nil || string(newData) != "content2" {
		t.Errorf("New backup was not created correctly: %v", err)
	}
}

func TestFileManagerEmptyConfigStaysMemoryOnly(t *testing.T) {
	working := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(working); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	fm := NewFileManager("")
	fm.AddRecentFile("")
	if got := fm.GetRecentFiles(); len(got) != 0 {
		t.Fatalf("empty recent path was retained: %#v", got)
	}
	if _, err := fm.CreateBackup(filepath.Join(working, "document.mbch")); err == nil {
		t.Fatal("backup unexpectedly succeeded without a configuration directory")
	}
	if err := fm.RestoreBackup("", ""); err == nil {
		t.Fatal("restore accepted empty paths")
	}
	if _, err := os.Stat(filepath.Join(working, RecentFilesFile)); !os.IsNotExist(err) {
		t.Fatal("memory-only manager joined an empty config path into the working directory")
	}
}


