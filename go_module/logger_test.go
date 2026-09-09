package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitLoggerAtCreatesPrivateAppendOnlyLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foundry.log")
	if err := os.WriteFile(path, []byte("existing line\n"), 0666); err != nil {
		t.Fatal(err)
	}

	previousFile := LogFile
	previousOutput := log.Writer()
	LogFile = nil
	t.Cleanup(func() {
		if LogFile != nil {
			_ = LogFile.Close()
		}
		LogFile = previousFile
		log.SetOutput(previousOutput)
	})

	if err := initLoggerAt(path); err != nil {
		t.Fatal(err)
	}
	log.Print("fixture diagnostic")
	if err := LogFile.Sync(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("log path is not a regular file: %v", info.Mode())
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("log permissions are %o, want 600", info.Mode().Perm())
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "existing line") || !strings.Contains(text, "fixture diagnostic") {
		t.Fatalf("logger did not append useful diagnostics: %q", text)
	}
}

func TestInitLoggerAtRejectsSymlinkAndPreservesTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "operator-file")
	path := filepath.Join(dir, "foundry.log")
	original := []byte("must remain unchanged")
	if err := os.WriteFile(target, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	before := LogFile
	if err := initLoggerAt(path); err == nil {
		t.Fatal("symlink log destination was accepted")
	}
	if LogFile != before {
		t.Fatal("failed logger initialization replaced the active logger")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("rejected log path changed its symlink target")
	}
}

func TestInitLoggerAtRejectsEmptyPath(t *testing.T) {
	if err := initLoggerAt(""); err == nil {
		t.Fatal("empty log path was accepted")
	}
}
