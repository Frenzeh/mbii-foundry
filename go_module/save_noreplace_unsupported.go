//go:build !darwin && !linux && !windows

package main

import (
	"errors"
	"fmt"
)

var errNoReplaceRenameUnsupported = errors.New("atomic no-replace rename is unsupported on this platform")

func renameNoReplace(oldPath, newPath string) error {
	return fmt.Errorf("rename %q to %q: %w", oldPath, newPath, errNoReplaceRenameUnsupported)
}
