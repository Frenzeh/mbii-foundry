package main

// Helpers for locating a user's Jedi Academy GameData folder across
// the usual install paths (LucasArts retail, Steam, GoG, Linux Steam,
// macOS Wine/OpenJK). The goal is that 80% of users never open the
// folder picker — auto-detect hands them a working path on first run.
//
// Detection rules:
//   - A valid GameData folder contains BOTH a `base/` subfolder
//     (stock JKA assets) and an `MBII/` subfolder (Movie Battles II
//     install) — both as real directories. Beta testers may have
//     MBIITest/ or MBIIRelease/ instead of MBII/. One without the
//     other is not a usable install and validation fails with a
//     targeted message.
//   - Steam installs are discovered through the bounded
//     steamapps/libraryfolders.vdf parse at the well-known Steam
//     roots — never a home-directory walk.
//   - TextAssets are paired with GameData via sibling relationships
//     (../TextAssets, ../mbii/TextAssets, …) — a handful of stat
//     calls, not a scan — and a candidate must carry recognized
//     content (ext_data/, models/, maps/ or a non-empty .git) before
//     it autofills.
//   - Auto-detected paths never overwrite a config field that already
//     validates (see ShouldReplaceGamedataPath / ShouldFillTextAssetsPath).

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DetectGamedataPath scans common install locations (plus Steam
// libraries from libraryfolders.vdf) and returns the first one that
// validates. Empty string if nothing matched.
func DetectGamedataPath() string {
	for _, c := range gamedataCandidates() {
		if ValidateGamedataPath(c) == nil {
			return c
		}
	}
	return ""
}

// DetectTextAssetsPath proposes a TextAssets checkout paired with the
// given GameData folder, checking only well-known sibling layouts:
//
//	GameData/../TextAssets
//	GameData/../mbii/TextAssets      (mbii-foundry checkouts nest under mbii/)
//	GameData/../MBII/TextAssets
//	GameData/../textassets/TextAssets
//
// Bounded by construction: at most a few os.Stat calls, never a
// directory walk. A candidate must pass ValidateTextAssetsPath — an
// empty or random sibling folder never autofills. Empty string when
// nothing matches.
func DetectTextAssetsPath(gamedata string) string {
	if gamedata == "" {
		return ""
	}
	parent := filepath.Dir(gamedata)
	rels := []string{
		"TextAssets",
		filepath.Join("mbii", "TextAssets"),
		filepath.Join("MBII", "TextAssets"),
		filepath.Join("textassets", "TextAssets"),
	}
	for _, rel := range rels {
		candidate := filepath.Join(parent, rel)
		if ValidateTextAssetsPath(candidate) == nil {
			return candidate
		}
	}
	return ""
}

// DetectTextAssetsStandalone discovers TextAssets when no usable
// GameData is known: it pairs against whatever path the user typed
// (even an invalid one — the sibling relationship still holds) and
// then against install candidates that exist on disk. Bounded: the
// candidate list is fixed, never a home walk.
func DetectTextAssetsStandalone(gamedata string) string {
	if gamedata != "" {
		if found := DetectTextAssetsPath(gamedata); found != "" {
			return found
		}
	}
	for _, c := range gamedataCandidates() {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		if found := DetectTextAssetsPath(c); found != "" {
			return found
		}
	}
	return ""
}

// ValidateTextAssetsPath accepts a directory that looks like the
// MBII TextAssets repo: either it carries recognized asset packages
// (ext_data/, models/, maps/) or it is a git checkout (.git/ with
// actual content). Empty and random folders are rejected so the
// wizard never autofills noise.
func ValidateTextAssetsPath(path string) error {
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
	if dirExists(filepath.Join(path, "ext_data")) ||
		dirExists(filepath.Join(path, "models")) ||
		dirExists(filepath.Join(path, "maps")) {
		return nil
	}
	// Git repo marker with content (a bare/empty .git is not a
	// checkout that ever produced assets).
	gitDir := filepath.Join(path, ".git")
	if entries, err := os.ReadDir(gitDir); err == nil && len(entries) > 0 {
		return nil
	}
	return &pathErr{"no TextAssets content found (need ext_data/, models/, maps/ or a git checkout)"}
}

// ShouldReplaceGamedataPath decides whether an auto-detected GameData
// path may replace the configured one. A field that already validates
// is never overwritten — detection only fills gaps and repairs broken
// values.
func ShouldReplaceGamedataPath(current, candidate string) bool {
	if candidate == "" {
		return false
	}
	if current == "" {
		return true
	}
	return ValidateGamedataPath(current) != nil
}

// ShouldFillTextAssetsPath decides whether a detected TextAssets path
// may be written into the field. TextAssets is optional and may point
// at any user-chosen checkout, so an existing value always wins.
func ShouldFillTextAssetsPath(current, candidate string) bool {
	return candidate != "" && strings.TrimSpace(current) == ""
}

// ValidateGamedataPath returns nil if the path looks like a usable
// JKA GameData folder: base/ AND an MBII family folder (MBII/,
// MBIITest/ or MBIIRelease/ for dev and beta installs) — both as
// real directories, not same-named files. Errors name exactly what
// is missing so the wizard's status line can act on it.
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

	// base/ must be a directory, not a same-named file.
	if !dirExists(filepath.Join(path, "base")) {
		return &pathErr{"no 'base' subfolder found — stock JKA assets are required"}
	}

	hasMBII := false
	for _, mbiiName := range []string{"MBII", "MBIITest", "MBIIRelease"} {
		if dirExists(filepath.Join(path, mbiiName)) {
			hasMBII = true
			break
		}
	}
	if !hasMBII {
		return &pathErr{"no 'MBII' subfolder found (MBIITest/MBIIRelease also accepted)"}
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
	for _, lib := range steamLibraryGameDirs() {
		parent := filepath.Dir(filepath.Dir(lib))
		if _, err := os.Stat(parent); err == nil {
			parents = append(parents, parent)
		}
	}
	return dedup(parents)
}

type pathErr struct{ msg string }

func (e *pathErr) Error() string { return e.msg }

// dirExists reports whether path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// steamRoots lists the well-known Steam install roots per platform.
// Detection reads libraryfolders.vdf only under these roots — a
// bounded, deterministic set.
func steamRoots() []string {
	home, _ := os.UserHomeDir()
	var roots []string
	switch runtime.GOOS {
	case "windows":
		for _, pf := range []string{
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramFiles"),
			`C:\Program Files (x86)`,
		} {
			if pf != "" {
				roots = append(roots, filepath.Join(pf, "Steam"))
			}
		}
	case "darwin":
		roots = append(roots, filepath.Join(home, "Library", "Application Support", "Steam"))
	default: // linux, freebsd, …
		roots = append(roots,
			filepath.Join(home, ".steam", "steam"),
			filepath.Join(home, ".local", "share", "Steam"),
		)
	}
	return dedup(roots)
}

// steamLibraryGameDirs returns `<root or library>/steamapps/common/
// Jedi Academy/GameData` for every Steam library it can find without
// walking the filesystem: the main root plus the libraries listed in
// each root's steamapps/libraryfolders.vdf (a tiny Valve-written
// file; reads are capped and failures are silently skipped).
func steamLibraryGameDirs() []string {
	var dirs []string
	seenLibs := map[string]bool{}

	addLib := func(libRoot string) {
		if seenLibs[libRoot] {
			return
		}
		seenLibs[libRoot] = true
		dirs = append(dirs, filepath.Join(libRoot, "steamapps", "common", "Jedi Academy", "GameData"))
	}

	for _, root := range steamRoots() {
		addLib(root)
		for _, lib := range steamLibraryPathsFromVDF(filepath.Join(root, "steamapps", "libraryfolders.vdf")) {
			addLib(lib)
		}
	}
	return dirs
}

// steamLibraryPathsFromVDF extracts the `path` values from a Steam
// libraryfolders.vdf. The file is a few KB; anything larger is
// treated as corrupt and skipped. Malformed escapes are left as-is —
// candidate paths are validated by existence checks anyway.
func steamLibraryPathsFromVDF(vdfPath string) []string {
	info, err := os.Stat(vdfPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil
	}
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return nil
	}

	var libs []string
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "\"path\"") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "\"path\""))
		if len(rest) < 2 || rest[0] != '"' {
			continue
		}
		end := strings.Index(rest[1:], "\"")
		if end < 0 {
			continue
		}
		value := rest[1 : 1+end]
		value = strings.ReplaceAll(value, "\\\\", "\\")
		if value == "" {
			continue
		}
		libs = append(libs, value)
	}
	return libs
}

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
		// Steam libraries from libraryfolders.vdf (bounded parse), then
		// the fixed drive-letter layouts for secondary libraries.
		candidates = append(candidates, steamLibraryGameDirs()...)
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
		// Application Support, or keep game assets in Synology Drive.
		return append([]string{
			filepath.Join(home, "Library", "CloudStorage", "SynologyDrive-mcp5", "MBII_GameData"),
			filepath.Join(home, "Library", "CloudStorage", "SynologyDrive", "MBII_GameData"),
			filepath.Join(home, "SynologyDrive", "mcp5", "MBII_GameData"),
			filepath.Join(home, "SynologyDrive", "MBII_GameData"),
			filepath.Join(home, "Library", "Application Support", "OpenJK"),
			filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Jedi Academy", "GameData"),
			"/Applications/Jedi Academy.app/Contents/Resources/GameData",
			"/Applications/Jedi Academy/GameData",
			filepath.Join(home, "Games", "Jedi Academy", "GameData"),
		}, steamLibraryGameDirs()...)

	default: // linux, freebsd, etc.
		return append([]string{
			filepath.Join(home, ".steam", "steam", "steamapps", "common", "Jedi Academy", "GameData"),
			filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "Jedi Academy", "GameData"),
			filepath.Join(home, "Games", "Jedi Academy", "GameData"),
			"/usr/local/share/jediacademy/GameData",
		}, steamLibraryGameDirs()...)
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
