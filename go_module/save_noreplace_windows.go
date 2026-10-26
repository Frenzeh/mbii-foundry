//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// renameNoReplace uses MoveFileEx without MOVEFILE_REPLACE_EXISTING. Same-volume
// moves are atomic and an existing destination makes the call fail.
func renameNoReplace(oldPath, newPath string) error {
	oldPtr, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: oldPath, New: newPath, Err: err}
	}
	newPtr, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: oldPath, New: newPath, Err: err}
	}
	if err := windows.MoveFileEx(oldPtr, newPtr, windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return &os.LinkError{Op: "rename-noreplace", Old: oldPath, New: newPath, Err: err}
	}
	return nil
}
