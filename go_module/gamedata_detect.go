package main

// Helpers for locating a user's Jedi Academy GameData folder across
// the usual install paths (LucasArts retail, Steam, GoG, Linux Steam,
// macOS Wine/OpenJK). The goal is that 80% of users never open the
// folder picker — auto-detect hands them a working path on first run.
//
// A valid GameData folder contains a `base/` subfolder (stock JKA
// assets) and an `MBII/` subfolder (Movie Battles II install). Both
// are required for MBII Foundry to be useful; we check both.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DetectGamedataPath scans common install locations and returns the
// first one that validates. Empty string if nothing matched.
func DetectGamedataPath() string {
	for _, c := range gamedataCandidates() {
		if ValidateGamedataPath(c) == nil {
			return c
		}
	}
	return ""
}

// ValidateGamedataPath returns nil if the path looks like a usable
// JKA GameData folder (has both base/ and MBII/), otherwise a
// descriptive error explaining what's missing.
func ValidateGamedataPath(path string) error {
	if path == "" {
		return &pathErr{"path is empty"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return &pathErr{"folder does not exist: " + path}
	}
	if !info.IsDir() {
		return &pathErr{"not a directory: " + path}
	}
	if _, err := os.Stat(filepath.Join(path, "base")); err != nil {
		return &pathErr{"no 'base' subfolder — this doesn't look like a Jedi Academy install"}
	}
	if _, err := os.Stat(filepath.Join(path, "MBII")); err != nil {
		return &pathErr{"no 'MBII' subfolder — JKA is installed here, but Movie Battles II isn't"}
	}
	return nil
}

// CommonGamedataParents returns directories worth opening a folder
// picker at (parents of likely install locations). Useful for setting
// the initial location of a Browse... dialog so users don't land at
// some arbitrary default.
func CommonGamedataParents() []string {
	var parents []string
	for _, c := range gamedataCandidates() {
		parent := filepath.Dir(c)
		if _, err := os.Stat(parent); err == nil {
			parents = append(parents, parent)
		}
	}
	return dedup(parents)
}

type pathErr struct{ msg string }

func (e *pathErr) Error() string { return e.msg }

func gamedataCandidates() []string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		// Check every common Windows install layout. Order matters —
		// the first match wins, so more-specific paths come first.
		var candidates []string
		programFiles := []string{
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramFiles"),
			`C:\Program Files (x86)`,
			`C:\Program Files`,
		}
		for _, pf := range programFiles {
			if pf == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(pf, "LucasArts", "Star Wars Jedi Knight Jedi Academy", "GameData"),
				filepath.Join(pf, "Steam", "steamapps", "common", "Jedi Academy", "GameData"),
			)
		}
		// Drive-letter sweep for secondary Steam libraries — users
		// routinely park games on D: or E:. We only check the common
		// library layouts, not random folders.
		for _, drive := range []string{"C:", "D:", "E:", "F:"} {
			candidates = append(candidates,
				filepath.Join(drive+`\`, "SteamLibrary", "steamapps", "common", "Jedi Academy", "GameData"),
				filepath.Join(drive+`\`, "Steam", "steamapps", "common", "Jedi Academy", "GameData"),
				filepath.Join(drive+`\`, "Games", "Star Wars Jedi Knight - Jedi Academy", "GameData"),
				filepath.Join(drive+`\`, "GOG Games", "Star Wars Jedi Knight - Jedi Academy", "GameData"),
			)
		}
		return candidates

	case "darwin":
		// macOS: JKA doesn't have a native mac build. Users run it
		// through Wine/CrossOver, or via OpenJK which installs to
		// Application Support.
		return []string{
			filepath.Join(home, "Library", "Application Support", "OpenJK"),
			filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Jedi Academy", "GameData"),
			"/Applications/Jedi Academy.app/Contents/Resources/GameData",
			"/Applications/Jedi Academy/GameData",
			filepath.Join(home, "Games", "Jedi Academy", "GameData"),
		}

	default: // linux, freebsd, etc.
		return []string{
			filepath.Join(home, ".steam", "steam", "steamapps", "common", "Jedi Academy", "GameData"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "Jedi Academy", "GameData"),
			filepath.Join(home, "Games", "Jedi Academy", "GameData"),
			"/usr/local/share/jediacademy/GameData",
		}
	}
}

func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimRight(s, string(os.PathSeparator))
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
