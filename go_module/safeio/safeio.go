package safeio

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// AtomicWrite writes data to a file atomically by first writing to a temporary file
// in the same directory, then renaming it to the target path.
// It preserves the existing file mode if the file already exists,
// rejects writing to non-regular files (e.g. symlinks, directories),
// and ensures the file is synced to disk.
func AtomicWrite(target string, mode fs.FileMode, write func(io.Writer) error) error {
	if target == "" {
		return fmt.Errorf("safeio: target path is empty")
	}
	if write == nil {
		return fmt.Errorf("safeio: write callback is nil")
	}
	// Reject non-regular targets
	info, err := os.Lstat(target)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("safeio: target is not a regular file (symlink/dir/device): %s", target)
		}
		// Preserve existing mode
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}

	dir := filepath.Dir(target)
	// Create temporary file beside destination
	f, err := os.CreateTemp(dir, filepath.Base(target)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	// Ensure cleanup on failure
	var success bool
	defer func() {
		if !success {
			f.Close()
			os.Remove(tmpName)
		}
	}()

	// Write data
	if err := write(f); err != nil {
		return err
	}

	// Finalize mode before Sync so metadata is included
	if err := f.Chmod(mode); err != nil {
		return err
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		return err
	}

	// Close file
	if err := f.Close(); err != nil {
		return err
	}

	// Atomic replace
	// Go's os.Rename on Windows uses MoveFileEx(MOVEFILE_REPLACE_EXISTING) which safely replaces
	// without needing a manual delete beforehand.
	if err := os.Rename(tmpName, target); err != nil {
		return err
	}

	success = true
	return nil
}

// WriteFile is a convenience wrapper around AtomicWrite for byte slices.
func WriteFile(target string, data []byte, mode fs.FileMode) error {
	return AtomicWrite(target, mode, func(w io.Writer) error {
		n, err := w.Write(data)
		if err == nil && n < len(data) {
			return io.ErrShortWrite
		}
		return err
	})
}
