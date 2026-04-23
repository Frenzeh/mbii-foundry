package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// GitHub raw content URLs for data files
	GitHubDataBaseURL = "https://raw.githubusercontent.com/Frenzeh/mbii-foundry/main/data"
)

var (
	// Global data stores
	LoadedAttributes  []AttributeDef
	LoadedWeapons     []WeaponDef
	LoadedClasses     []ClassDef
	LoadedClassFlags  []ClassFlagDef
	LoadedSaberStyles []SaberStyleDef
	LoadedGlossary    []GlossaryDef
	DataLock          sync.RWMutex
	DataPath          string // Store the data path for updates
)

// LoadExternalData loads data files from the specified directory.
func LoadExternalData(dataPath string) error {
	DataLock.Lock()
	defer DataLock.Unlock()

	DataPath = dataPath // Store for GitHub update caching

	// --- Classes ---
	classPath := filepath.Join(dataPath, "classes.json")
	classData, err := os.ReadFile(classPath)

	LoadedClasses = make([]ClassDef, len(MBIIClasses))
	copy(LoadedClasses, MBIIClasses)

	if err == nil {
		var classes []ClassDef
		if err := json.Unmarshal(classData, &classes); err == nil {
			// Merge logic
			classMap := make(map[string]int)
			for i, c := range LoadedClasses {
				classMap[c.ID] = i
			}

			for _, ext := range classes {
				if idx, ok := classMap[ext.ID]; ok {
					if ext.Name != "" {
						LoadedClasses[idx].Name = ext.Name
					}
					if ext.Description != "" {
						LoadedClasses[idx].Description = ext.Description
					}
				} else {
					LoadedClasses = append(LoadedClasses, ext)
					classMap[ext.ID] = len(LoadedClasses) - 1
				}
			}
			LogInfo("Loaded and merged %d classes from %s", len(classes), classPath)
		} else {
			LogError("Failed to parse classes.json: %v", err)
		}
	} else {
		LogInfo("classes.json not found, using defaults.")
	}

	// Load Attributes
	attPath := filepath.Join(dataPath, "attributes.json")
	attData, err := os.ReadFile(attPath)

	// Initialize with defaults
	LoadedAttributes = make([]AttributeDef, len(MBIIAttributes))
	copy(LoadedAttributes, MBIIAttributes)

	if err == nil {
		var atts []AttributeDef
		if err := json.Unmarshal(attData, &atts); err == nil {
			// Merge logic
			attMap := make(map[string]int)
			for i, a := range LoadedAttributes {
				attMap[a.ID] = i
			}

			for _, ext := range atts {
				if idx, ok := attMap[ext.ID]; ok {
					// Update existing
					if ext.Name != "" {
						LoadedAttributes[idx].Name = ext.Name
					}
					if ext.Description != "" {
						LoadedAttributes[idx].Description = ext.Description
					}
					if ext.Category != "" {
						LoadedAttributes[idx].Category = ext.Category
					}
					if ext.MaxLevel > 0 {
						LoadedAttributes[idx].MaxLevel = ext.MaxLevel
					}

					// Rich Docs - overwrite
					if ext.Overview != "" {
						LoadedAttributes[idx].Overview = ext.Overview
					}
					if len(ext.Levels) > 0 {
						LoadedAttributes[idx].Levels = ext.Levels
					}
					if len(ext.Tips) > 0 {
						LoadedAttributes[idx].Tips = ext.Tips
					}
					if len(ext.Tags) > 0 {
						LoadedAttributes[idx].Tags = ext.Tags
					}
				} else {
					// Add new
					LoadedAttributes = append(LoadedAttributes, ext)
					attMap[ext.ID] = len(LoadedAttributes) - 1
				}
			}
			LogInfo("Loaded and merged %d attributes from %s", len(atts), attPath)
		} else {
			LogError("Failed to parse attributes.json: %v", err)
		}
	} else {
		LogInfo("attributes.json not found at %s, using defaults.", attPath)
	}

	// Load Weapons
	weapPath := filepath.Join(dataPath, "weapons.json")
	weapData, err := os.ReadFile(weapPath)

	// Initialize with defaults
	LoadedWeapons = make([]WeaponDef, len(MBIIWeapons))
	copy(LoadedWeapons, MBIIWeapons)

	if err == nil {
		var weaps []WeaponDef
		if err := json.Unmarshal(weapData, &weaps); err == nil {
			// Merge logic
			weapMap := make(map[string]int)
			for i, w := range LoadedWeapons {
				weapMap[w.ID] = i
			}

			for _, ext := range weaps {
				if idx, ok := weapMap[ext.ID]; ok {
					if ext.Name != "" {
						LoadedWeapons[idx].Name = ext.Name
					}
					if ext.Description != "" {
						LoadedWeapons[idx].Description = ext.Description
					}
					if ext.Category != "" {
						LoadedWeapons[idx].Category = ext.Category
					}

					if ext.Overview != "" {
						LoadedWeapons[idx].Overview = ext.Overview
					}
					if len(ext.Tips) > 0 {
						LoadedWeapons[idx].Tips = ext.Tips
					}
					if len(ext.Tags) > 0 {
						LoadedWeapons[idx].Tags = ext.Tags
					}
				} else {
					LoadedWeapons = append(LoadedWeapons, ext)
					weapMap[ext.ID] = len(LoadedWeapons) - 1
				}
			}
			LogInfo("Loaded and merged %d weapons from %s", len(weaps), weapPath)
		} else {
			LogError("Failed to parse weapons.json: %v", err)
		}
	} else {
		LogInfo("weapons.json not found at %s, using defaults.", weapPath)
	}

	// --- Class Flags ---
	cfPath := filepath.Join(dataPath, "class_flags.json")
	cfData, err := os.ReadFile(cfPath)
	LoadedClassFlags = make([]ClassFlagDef, len(MBIIClassFlags))
	copy(LoadedClassFlags, MBIIClassFlags)
	if err == nil {
		var flags []ClassFlagDef
		if err := json.Unmarshal(cfData, &flags); err == nil {
			fMap := make(map[string]int)
			for i, f := range LoadedClassFlags {
				fMap[f.ID] = i
			}
			for _, ext := range flags {
				if idx, ok := fMap[ext.ID]; ok {
					if ext.Name != "" {
						LoadedClassFlags[idx].Name = ext.Name
					}
					if ext.Description != "" {
						LoadedClassFlags[idx].Description = ext.Description
					}
					if ext.Overview != "" {
						LoadedClassFlags[idx].Overview = ext.Overview
					}
				} else {
					LoadedClassFlags = append(LoadedClassFlags, ext)
					fMap[ext.ID] = len(LoadedClassFlags) - 1
				}
			}
			LogInfo("Loaded %d class flags from %s", len(flags), cfPath)
		}
	}

	// --- Saber Styles ---
	ssPath := filepath.Join(dataPath, "saber_styles.json")
	ssData, err := os.ReadFile(ssPath)
	LoadedSaberStyles = make([]SaberStyleDef, len(MBIISaberStyles))
	copy(LoadedSaberStyles, MBIISaberStyles)
	if err == nil {
		var styles []SaberStyleDef
		if err := json.Unmarshal(ssData, &styles); err == nil {
			sMap := make(map[string]int)
			for i, s := range LoadedSaberStyles {
				sMap[s.ID] = i
			}
			for _, ext := range styles {
				if idx, ok := sMap[ext.ID]; ok {
					if ext.Name != "" {
						LoadedSaberStyles[idx].Name = ext.Name
					}
					if ext.Description != "" {
						LoadedSaberStyles[idx].Description = ext.Description
					}
					if ext.Overview != "" {
						LoadedSaberStyles[idx].Overview = ext.Overview
					}
				} else {
					LoadedSaberStyles = append(LoadedSaberStyles, ext)
					sMap[ext.ID] = len(LoadedSaberStyles) - 1
				}
			}
			LogInfo("Loaded %d saber styles from %s", len(styles), ssPath)
		}
	}

	// --- Glossary ---
	glossPath := filepath.Join(dataPath, "glossary.json")
	glossData, err := os.ReadFile(glossPath)
	if err == nil {
		var gloss []GlossaryDef
		if err := json.Unmarshal(glossData, &gloss); err == nil {
			LoadedGlossary = gloss
			LogInfo("Loaded %d glossary terms from %s", len(gloss), glossPath)
		}
	}

	// Flip the Hidden flag on IDs from hidden_content.go's curated
	// sets (#ifdef-guarded or commented-out in the game headers). The
	// JSON files are flat lists that don't know about live-vs-hidden
	// status — that's a property of the build, not the content — so
	// we apply it here after all loading is done.
	markHiddenClasses(LoadedClasses)
	markHiddenWeapons(LoadedWeapons)
	markHiddenAttributes(LoadedAttributes)

	// Strip decorative emoji prefixes from weapon/attribute names.
	// Historically the JSON hand-crafted names like "💣 Pulse Grenade"
	// as a cheap stand-in for real icons. Now that Foundry embeds the
	// actual game HUD icons and renders them alongside the label,
	// those emojis are visual noise fighting the real art. The strip
	// is conservative — only removes leading non-word runes (emojis,
	// combining marks, surrounding whitespace) up to the first real
	// letter/digit, so names like "E-11 Blaster" or "(Old) Bryar"
	// keep their natural opening punctuation.
	for i := range LoadedWeapons {
		LoadedWeapons[i].Name = stripLeadingNonWord(LoadedWeapons[i].Name)
	}
	for i := range LoadedAttributes {
		LoadedAttributes[i].Name = stripLeadingNonWord(LoadedAttributes[i].Name)
	}

	return nil
}

// GetAttributes returns loaded attributes with non-live entries
// filtered out. See hidden_content.go for what counts as hidden and
// why — short version: #ifdef-guarded or commented-out entries from
// the game's bg_public.h / bg_weapons.h that shouldn't appear in
// editor pickers. Use GetAllAttributes if you need the full set
// (e.g. rendering a loaded file that references a custom attribute).
func GetAttributes() []AttributeDef {
	return filterVisibleAttributes(GetAllAttributes())
}

// GetAllAttributes returns every loaded attribute, including ones
// marked Hidden. Falls back to the hardcoded defaults if external
// data hasn't loaded.
func GetAllAttributes() []AttributeDef {
	DataLock.RLock()
	defer DataLock.RUnlock()
	if len(LoadedAttributes) > 0 {
		return LoadedAttributes
	}
	return MBIIAttributes
}

// GetWeapons returns loaded weapons with non-live entries filtered out.
func GetWeapons() []WeaponDef {
	return filterVisibleWeapons(GetAllWeapons())
}

// GetAllWeapons returns every loaded weapon, including Hidden ones.
func GetAllWeapons() []WeaponDef {
	DataLock.RLock()
	defer DataLock.RUnlock()
	if len(LoadedWeapons) > 0 {
		return LoadedWeapons
	}
	return MBIIWeapons
}

// GetClasses returns loaded classes with non-live entries filtered out.
func GetClasses() []ClassDef {
	return filterVisibleClasses(GetAllClasses())
}

// GetAllClasses returns every loaded class, including Hidden ones.
func GetAllClasses() []ClassDef {
	DataLock.RLock()
	defer DataLock.RUnlock()
	if len(LoadedClasses) > 0 {
		return LoadedClasses
	}
	return MBIIClasses
}

func GetClassFlags() []ClassFlagDef {
	DataLock.RLock()
	defer DataLock.RUnlock()
	if len(LoadedClassFlags) > 0 {
		return LoadedClassFlags
	}
	return MBIIClassFlags
}

func GetSaberStyles() []SaberStyleDef {
	DataLock.RLock()
	defer DataLock.RUnlock()
	if len(LoadedSaberStyles) > 0 {
		return LoadedSaberStyles
	}
	return MBIISaberStyles
}

func GetGlossary() []GlossaryDef {
	DataLock.RLock()
	defer DataLock.RUnlock()
	return LoadedGlossary
}

// FetchDataFromGitHub downloads a JSON file from the GitHub repository.
// Returns the downloaded content and any error encountered.
func FetchDataFromGitHub(filename string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", GitHubDataBaseURL, filename)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %v", filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: HTTP %d", filename, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %v", filename, err)
	}

	return data, nil
}

// UpdateDataFromGitHub fetches the latest data files from GitHub and updates local cache.
// If dataPath is empty, only in-memory data is updated.
// Returns a summary message of what was updated.
func UpdateDataFromGitHub() (string, error) {
	var updated []string
	var errors []string

	// Fetch and update attributes
	attData, err := FetchDataFromGitHub("attributes.json")
	if err != nil {
		errors = append(errors, fmt.Sprintf("attributes: %v", err))
	} else {
		var atts []AttributeDef
		if err := json.Unmarshal(attData, &atts); err != nil {
			errors = append(errors, fmt.Sprintf("attributes parse: %v", err))
		} else {
			DataLock.Lock()
			LoadedAttributes = atts
			DataLock.Unlock()
			updated = append(updated, fmt.Sprintf("attributes (%d items)", len(atts)))

			// Save to local file if path is set
			if DataPath != "" {
				localPath := filepath.Join(DataPath, "attributes.json")
				if err := os.WriteFile(localPath, attData, 0644); err != nil {
					LogError("Failed to save attributes.json locally: %v", err)
				} else {
					LogInfo("Saved attributes.json to %s", localPath)
				}
			}
		}
	}

	// Fetch and update weapons
	weapData, err := FetchDataFromGitHub("weapons.json")
	if err != nil {
		errors = append(errors, fmt.Sprintf("weapons: %v", err))
	} else {
		var weaps []WeaponDef
		if err := json.Unmarshal(weapData, &weaps); err != nil {
			errors = append(errors, fmt.Sprintf("weapons parse: %v", err))
		} else {
			DataLock.Lock()
			LoadedWeapons = weaps
			DataLock.Unlock()
			updated = append(updated, fmt.Sprintf("weapons (%d items)", len(weaps)))

			// Save to local file if path is set
			if DataPath != "" {
				localPath := filepath.Join(DataPath, "weapons.json")
				if err := os.WriteFile(localPath, weapData, 0644); err != nil {
					LogError("Failed to save weapons.json locally: %v", err)
				} else {
					LogInfo("Saved weapons.json to %s", localPath)
				}
			}
		}
	}

	// Build result message
	var result string
	if len(updated) > 0 {
		result = fmt.Sprintf("Updated: %v", updated)
	}
	if len(errors) > 0 {
		if result != "" {
			result += "; "
		}
		result += fmt.Sprintf("Errors: %v", errors)
		return result, fmt.Errorf("some updates failed")
	}

	if result == "" {
		return "No updates performed", nil
	}
	return result, nil
}
