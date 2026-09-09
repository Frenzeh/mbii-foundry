//go:build linux

package main

import "golang.org/x/sys/unix"

// renameNoReplace atomically renames oldPath to newPath only when newPath does
// not exist. Unsupported kernels and filesystems return their native error;
// there is deliberately no racy fallback.
func renameNoReplace(oldPath, newPath string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE)
}
