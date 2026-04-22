package main

// resolveDataPath locates the `data/` folder across the install layouts
// the app might be launched from:
//
//   - go build inside go_module/ → binary at go_module/mbii-foundry,
//     data at ../data
//   - Release binary in a tarball/zip → binary and data/ live side by side
//     at the extract root
//   - macOS .app bundle → binary at Contents/MacOS/, data at
//     Contents/Resources/data
//   - go run from tmp/go-build → CWD is go_module/, data at ../data
//
// Returns the absolute path of the first valid candidate, or "" if
// none exist. The data/ folder is validated by checking for at least
// one expected file (attributes.json) — an empty data/ dir doesn't
// count.

import (
	"os"
	"path/filepath"
)

func resolveDataPath() string {
	// Candidate generation — ordered most-specific to most-generic.
	var candidates []string

	if ex, err := os.Executable(); err == nil {
		exDir := filepath.Dir(ex)
		candidates = append(candidates,
			filepath.Join(exDir, "data"),                    // release zip/tarball
			filepath.Join(exDir, "..", "data"),              // go_module build
			filepath.Join(exDir, "..", "Resources", "data"), // macOS .app
			filepath.Join(exDir, "..", "..", "data"),        // nested build dir
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "data"),
			filepath.Join(cwd, "..", "data"),
		)
	}

	for _, c := range candidates {
		if hasValidDataDir(c) {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}

// hasValidDataDir returns true if the candidate path is a directory
// containing at least attributes.json. Shallow check — sufficient to
// reject empty dirs and false-positive siblings.
func hasValidDataDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(path, "attributes.json"))
	return err == nil
}
