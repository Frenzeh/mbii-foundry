package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	Definitions     map[string]string
	DefinitionsLock sync.RWMutex
)

// InitDefinitions loads definitions from standard locations
func InitDefinitions() {
	DefinitionsLock.Lock()
	defer DefinitionsLock.Unlock()
	Definitions = make(map[string]string)

	// Potential paths for definitions
	paths := []string{
		"definitions",                  // relative to binary
		"../definitions",               // relative to go_module source
		"../../definitions",            // deeper nesting
		"fa_creator_app/definitions",   // from root
	}

	for _, p := range paths {
		absPath, _ := filepath.Abs(p)
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			LogInfo("Found definitions at: %s", absPath)
			loadDefinitionsFromPath(absPath)
			return // Stop after finding the first valid directory
		}
	}
	LogInfo("Warning: definitions folder not found")
}

func loadDefinitionsFromPath(root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, err := os.ReadFile(path)
			if err != nil { return nil }
			
			// Key is the filename without extension (e.g., MB_ATT_JETPACK)
			key := strings.TrimSuffix(info.Name(), ".md")
			Definitions[key] = string(content)
		}
		return nil
	})
}

func GetDefinition(key string) (string, bool) {
	DefinitionsLock.RLock()
	defer DefinitionsLock.RUnlock()
	val, ok := Definitions[key]
	return val, ok
}
