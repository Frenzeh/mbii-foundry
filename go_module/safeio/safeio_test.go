package safeio

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWrite_Success(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")

	err := AtomicWrite(target, 0644, func(w io.Writer) error {
		_, err := w.Write([]byte("success"))
		return err
	})
	if err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "success" {
		t.Errorf("expected 'success', got '%s'", string(content))
	}
}

func TestAtomicWrite_Failure(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	os.WriteFile(target, []byte("original"), 0644)

	expectedErr := errors.New("simulated error")
	err := AtomicWrite(target, 0644, func(w io.Writer) error {
		w.Write([]byte("new content"))
		return expectedErr
	})
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "original" {
		t.Errorf("expected 'original', got '%s'", string(content))
	}
}

func TestAtomicWrite_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	os.WriteFile(target, []byte("original"), 0755) // executable

	err := AtomicWrite(target, 0600, func(w io.Writer) error {
		_, err := w.Write([]byte("new"))
		return err
	})
	if err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("atomic replacement is not a regular file: %v", info.Mode())
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0755 {
		t.Errorf("expected mode 0755, got %v", info.Mode().Perm())
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("atomic replacement content = %q, want %q", content, "new")
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")

	err := WriteFile(target, []byte("data"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "data" {
		t.Errorf("expected 'data', got '%s'", string(content))
	}
}

func TestAtomicWriteRejectsEmptyTargetAndNilWriter(t *testing.T) {
	if err := AtomicWrite("", 0600, func(io.Writer) error { return nil }); err == nil {
		t.Fatal("empty target path was accepted")
	}
	if err := AtomicWrite(filepath.Join(t.TempDir(), "target"), 0600, nil); err == nil {
		t.Fatal("nil write callback was accepted")
	}
}

func TestAtomicWriteRejectsSymlinkWithoutChangingOriginal(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(original, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(original, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteFile(link, []byte("replacement"), 0600); err == nil {
		t.Fatal("symlink destination was accepted")
	}
	got, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatal("rejected symlink write changed the original")
	}
}
