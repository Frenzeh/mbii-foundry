package main

// App-config directory resolution + one-time migration from the
// pre-rebrand location.
//
// Old:  <UserConfigDir>/mbii-fa-creator/
// New:  <UserConfigDir>/mbii-foundry/
//
// On first launch after upgrade, if the old dir exists and the new
// doesn't, we copy contents over. The old dir is left in place as a
// safety net — users can delete it after confirming the new location
// works. Migration failures fall back to the old dir so nobody loses
// their config.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AppConfigDir returns the absolute path to the app's config directory,
// creating it and migrating from the old brand name if necessary.
// A nonempty directory with an error is a usable legacy fallback; empty means unavailable.
func AppConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return appConfigDirWithBase(base)
}

func appConfigDirWithBase(base string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("configuration base directory is unavailable")
	}
	newDir := filepath.Join(base, "mbii-foundry")
	oldDir := filepath.Join(base, "mbii-fa-creator")

	// Already migrated (or fresh install that's already written once).
	if info, err := os.Stat(newDir); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("configuration location is not a directory")
		}
		return newDir, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("configuration directory cannot be accessed: %w", err)
	}

	// Migration path: old exists, new doesn't.
	if info, err := os.Stat(oldDir); err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("legacy configuration location is not a directory")
		}
		// Use unique sibling staging
		tmpDir, err := os.MkdirTemp(base, "mbii-foundry-migration-")
		if err != nil {
			wrapped := fmt.Errorf("configuration migration could not create staging: %w", err)
			LogError("%v; staying on old path", wrapped)
			return oldDir, wrapped
		}
		
		// Ensure we clean up the temp dir on failure or if rename fails
		defer os.RemoveAll(tmpDir)

		if copyErr := copyDir(oldDir, tmpDir); copyErr != nil {
			wrapped := fmt.Errorf("configuration migration copy failed: %w", copyErr)
			LogError("%v; staying on old path", wrapped)
			return oldDir, wrapped
		}

		if renameErr := os.Rename(tmpDir, newDir); renameErr != nil {
			wrapped := fmt.Errorf("configuration migration commit failed: %w", renameErr)
			LogError("%v; staying on old path", wrapped)
			return oldDir, wrapped
		}

		LogInfo("Migrated config dir: %s -> %s (old dir left as backup)", oldDir, newDir)
		return newDir, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("legacy configuration directory cannot be accessed: %w", err)
	}

	// Fresh install.
	if err := os.MkdirAll(newDir, 0700); err != nil {
		LogError("Failed to create config dir %s: %v", newDir, err)
		return "", err
	}
	return newDir, nil
}

// copyDir copies regular files and directories into an owned staging directory.
// Unsupported entries fail migration rather than silently dropping configuration.
func copyDir(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("configuration migration requires nonempty source and destination directories")
	}
	type copiedDirectory struct {
		path string
		mode os.FileMode
	}
	var directories []copiedDirectory
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			// Keep staging directories owner-writable until every child is copied.
			// Read-only legacy directories receive their original mode at the end.
			if err := os.MkdirAll(target, 0700); err != nil {
				return err
			}
			directories = append(directories, copiedDirectory{path: target, mode: info.Mode().Perm()})
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported entry in legacy configuration: %s", rel)
		}
		return copyFileWithMode(path, target, info.Mode())
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		mode := directories[i].mode
		if directories[i].path == dst {
			mode = 0700
		}
		if err := os.Chmod(directories[i].path, mode); err != nil {
			return err
		}
	}
	return nil
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if err := out.Chmod(mode.Perm()); err != nil {
		out.Close()
		return err
	}
	_, err = io.Copy(out, in)
	if err != nil {
		out.Close()
		return err
	}

	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}
