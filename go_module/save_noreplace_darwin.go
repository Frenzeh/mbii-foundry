//go:build darwin

package main

import "golang.org/x/sys/unix"

// renameNoReplace atomically renames oldPath to newPath only when newPath does
// not exist. Both paths must be on the same filesystem.
func renameNoReplace(oldPath, newPath string) error {
	return unix.RenameatxNp(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_EXCL)
}
