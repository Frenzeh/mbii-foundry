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
	"io"
	"os"
	"path/filepath"
)

// AppConfigDir returns the absolute path to the app's config directory,
// creating it and migrating from the old brand name if necessary.
// Returns an empty string only if UserConfigDir itself fails (rare).
func AppConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	newDir := filepath.Join(base, "mbii-foundry")
	oldDir := filepath.Join(base, "mbii-fa-creator")

	// Already migrated (or fresh install that's already written once).
	if _, err := os.Stat(newDir); err == nil {
		return newDir
	}

	// Migration path: old exists, new doesn't.
	if _, err := os.Stat(oldDir); err == nil {
		if copyErr := copyDir(oldDir, newDir); copyErr != nil {
			LogError("Config migration %s -> %s failed: %v; staying on old path", oldDir, newDir, copyErr)
			return oldDir
		}
		LogInfo("Migrated config dir: %s -> %s (old dir left as backup)", oldDir, newDir)
		return newDir
	}

	// Fresh install.
	if err := os.MkdirAll(newDir, 0755); err != nil {
		LogError("Failed to create config dir %s: %v", newDir, err)
		return ""
	}
	return newDir
}

// copyDir recursively copies src into dst. dst must not already exist.
// Creates parent directories as needed. Regular files only — symlinks
// and device nodes are skipped (none live in config dirs).
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if !info.Mode().IsRegular() {
			return nil // skip symlinks, sockets, devices
		}
		return copyFileWithMode(path, target, info.Mode())
	})
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
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
