package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	Definitions     = make(map[string]string)
	DefinitionsLock sync.RWMutex
)

// InitDefinitions loads per-enum markdown docs from the first install
// layout that has them. Same probing approach as resolveDataPath() in
// data_path.go — relative paths don't reach into the macOS .app
// bundle's Contents/Resources/, so we probe binary-relative layouts
// explicitly.
func InitDefinitions() {
	DefinitionsLock.Lock()
	defer DefinitionsLock.Unlock()
	Definitions = make(map[string]string)

	found := resolveDefinitionsPath()
	if found == "" {
		LogInfo("Warning: definitions folder not found — hover docs will be unavailable")
		return
	}
	LogInfo("Found definitions at: %s", found)
	loadDefinitionsFromPath(found)
}

func resolveDefinitionsPath() string {
	var candidates []string
	if ex, err := os.Executable(); err == nil {
		exDir := filepath.Dir(ex)
		candidates = append(candidates,
			filepath.Join(exDir, "definitions"),                    // release zip/tarball
			filepath.Join(exDir, "..", "definitions"),              // go_module build
			filepath.Join(exDir, "..", "Resources", "definitions"), // macOS .app bundle
			filepath.Join(exDir, "..", "..", "definitions"),        // nested build dir
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "definitions"),
			filepath.Join(cwd, "..", "definitions"),
			filepath.Join(cwd, "mbii-foundry", "definitions"),
			// Legacy: Foundry mounted inside mbii-holocron as a submodule
			filepath.Join(cwd, "fa_creator_app", "definitions"),
		)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			// Quick sanity: a valid definitions dir has subcategories.
			if _, err := os.Stat(filepath.Join(c, "attributes")); err == nil {
				abs, _ := filepath.Abs(c)
				if abs == "" {
					abs = c
				}
				return abs
			}
		}
	}
	return ""
}

func loadDefinitionsFromPath(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			// Index by bare filename (MB_ATT_PUSH) so the info-panel
			// lookup can resolve by enum ID. Also index by the relative
			// path to support fuzzy lookups the info panel does
			// elsewhere.
			key := strings.TrimSuffix(info.Name(), ".md")
			Definitions[key] = string(content)
			if rel, relErr := filepath.Rel(root, path); relErr == nil {
				relKey := strings.TrimSuffix(rel, ".md")
				Definitions[relKey] = string(content)
			}
		}
		return nil
	})
}

// GetDefinition returns the markdown body for a key, if known.
func GetDefinition(key string) (string, bool) {
	DefinitionsLock.RLock()
	defer DefinitionsLock.RUnlock()
	v, ok := Definitions[key]
	return v, ok
}
