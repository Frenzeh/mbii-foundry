package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Frenzeh/mbii-foundry/safeio"
)

const (
	// Manual catalogs follow upstream main over HTTPS, not the signed app-update manifest.
	GitHubDataBaseURL    = "https://raw.githubusercontent.com/Frenzeh/mbii-foundry/main/data"
	maxCatalogFileBytes  = 4 << 20
	maxCatalogCacheBytes = 2 * maxCatalogFileBytes
	maxCatalogEntries    = 4096
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
	DataPath          string // Read-only installed data directory; never an update destination.
)

// LoadExternalData merges installed data and a valid user cache over embedded defaults.
func LoadExternalData(dataPath string) error {
	DataLock.Lock()
	defer DataLock.Unlock()

	DataPath = dataPath

	// --- Classes ---
	classPath := filepath.Join(dataPath, "classes.json")
	classData, err := readInstalledDataFile(dataPath, "classes.json")

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

	catalog := loadBaseCatalog(dataPath)
	if cachePath, err := catalogCachePath(); err != nil {
		LogError("Catalog cache is unavailable; using installed data and defaults: %v", err)
	} else if cached, err := readCatalogCache(cachePath); err == nil {
		catalog.Attributes = mergeCatalogAttributes(catalog.Attributes, cached.Attributes)
		catalog.Weapons = mergeCatalogWeapons(catalog.Weapons, cached.Weapons)
		LogInfo("Loaded catalog cache from %s", cachePath)
	} else if !os.IsNotExist(err) {
		LogError("Ignoring invalid catalog cache %s: %v", cachePath, err)
	}

	// --- Class Flags ---
	cfPath := filepath.Join(dataPath, "class_flags.json")
	cfData, err := readInstalledDataFile(dataPath, "class_flags.json")
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
	ssData, err := readInstalledDataFile(dataPath, "saber_styles.json")
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
	glossData, err := readInstalledDataFile(dataPath, "glossary.json")
	if err == nil {
		var gloss []GlossaryDef
		if err := json.Unmarshal(glossData, &gloss); err == nil {
			LoadedGlossary = gloss
			LogInfo("Loaded %d glossary terms from %s", len(gloss), glossPath)
		}
	}

	markHiddenClasses(LoadedClasses)
	applyCatalogPolicy(&catalog)
	LoadedAttributes, LoadedWeapons = catalog.Attributes, catalog.Weapons

	return nil
}

type catalogCache struct {
	Attributes []AttributeDef `json:"attributes"`
	Weapons    []WeaponDef    `json:"weapons"`
}

// CatalogUpdate holds a persisted, prepared catalog awaiting UI-thread publication.
type CatalogUpdate struct {
	Summary string
	catalog catalogCache
}

func readCatalogFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, readErr := readAllLimited(f, limit)
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, err
	}
	return data, nil
}

func readInstalledDataFile(dataPath, filename string) ([]byte, error) {
	if dataPath == "" {
		return nil, os.ErrNotExist
	}
	return readCatalogFile(filepath.Join(dataPath, filename), maxCatalogFileBytes)
}

func loadBaseCatalog(dataPath string) catalogCache {
	catalog := catalogCache{
		Attributes: append([]AttributeDef(nil), MBIIAttributes...),
		Weapons:    append([]WeaponDef(nil), MBIIWeapons...),
	}
	if data, err := readInstalledDataFile(dataPath, "attributes.json"); err == nil {
		var attrs []AttributeDef
		if err := json.Unmarshal(data, &attrs); err == nil {
			catalog.Attributes = mergeCatalogAttributes(catalog.Attributes, attrs)
		} else {
			LogError("Failed to parse installed attributes.json: %v", err)
		}
	} else if !os.IsNotExist(err) {
		LogError("Failed to read installed attributes.json: %v", err)
	}
	if data, err := readInstalledDataFile(dataPath, "weapons.json"); err == nil {
		var weapons []WeaponDef
		if err := json.Unmarshal(data, &weapons); err == nil {
			catalog.Weapons = mergeCatalogWeapons(catalog.Weapons, weapons)
		} else {
			LogError("Failed to parse installed weapons.json: %v", err)
		}
	} else if !os.IsNotExist(err) {
		LogError("Failed to read installed weapons.json: %v", err)
	}
	return catalog
}

func mergeCatalogAttributes(attrs, external []AttributeDef) []AttributeDef {
	byID := make(map[string]int, len(attrs))
	for i, attr := range attrs {
		byID[attr.ID] = i
	}
	for _, ext := range external {
		idx, ok := byID[ext.ID]
		if !ok {
			attrs = append(attrs, ext)
			byID[ext.ID] = len(attrs) - 1
			continue
		}
		if ext.Name != "" {
			attrs[idx].Name = ext.Name
		}
		if ext.Description != "" {
			attrs[idx].Description = ext.Description
		}
		if ext.Category != "" {
			attrs[idx].Category = ext.Category
		}
		if ext.MaxLevel > 0 {
			attrs[idx].MaxLevel = ext.MaxLevel
		}
		if ext.Overview != "" {
			attrs[idx].Overview = ext.Overview
		}
		if len(ext.Levels) > 0 {
			attrs[idx].Levels = ext.Levels
		}
		if len(ext.Tips) > 0 {
			attrs[idx].Tips = ext.Tips
		}
		if len(ext.Tags) > 0 {
			attrs[idx].Tags = ext.Tags
		}
	}
	return attrs
}

func mergeCatalogWeapons(weapons, external []WeaponDef) []WeaponDef {
	byID := make(map[string]int, len(weapons))
	for i, weapon := range weapons {
		byID[weapon.ID] = i
	}
	for _, ext := range external {
		idx, ok := byID[ext.ID]
		if !ok {
			weapons = append(weapons, ext)
			byID[ext.ID] = len(weapons) - 1
			continue
		}
		if ext.Name != "" {
			weapons[idx].Name = ext.Name
		}
		if ext.Description != "" {
			weapons[idx].Description = ext.Description
		}
		if ext.Category != "" {
			weapons[idx].Category = ext.Category
		}
		if ext.Overview != "" {
			weapons[idx].Overview = ext.Overview
		}
		if len(ext.Tips) > 0 {
			weapons[idx].Tips = ext.Tips
		}
		if len(ext.Tags) > 0 {
			weapons[idx].Tags = ext.Tags
		}
	}
	return weapons
}

func applyCatalogPolicy(catalog *catalogCache) {
	markHiddenAttributes(catalog.Attributes)
	markHiddenWeapons(catalog.Weapons)
	recategorizeAttributes(catalog.Attributes)
	for i := range catalog.Weapons {
		catalog.Weapons[i].Name = stripLeadingNonWord(catalog.Weapons[i].Name)
	}
	fallbacks := make(map[string]int, len(MBIIAttributes))
	for _, attr := range MBIIAttributes {
		fallbacks[attr.ID] = attr.MaxLevel
	}
	for i := range catalog.Attributes {
		attr := &catalog.Attributes[i]
		attr.Name = stripLeadingNonWord(attr.Name)
		if attr.MaxLevel <= 0 && fallbacks[attr.ID] > 0 {
			attr.MaxLevel = fallbacks[attr.ID]
		}
	}
}

func validateCatalogID(id string, seen map[string]bool) error {
	if len(id) == 0 || len(id) > 128 {
		return fmt.Errorf("catalog identifier must contain 1–128 bytes")
	}
	for _, r := range id {
		if r != '_' && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') {
			return fmt.Errorf("invalid catalog identifier %q", id)
		}
	}
	if seen[id] {
		return fmt.Errorf("duplicate catalog identifier %q", id)
	}
	seen[id] = true
	return nil
}

func validateCatalog(catalog catalogCache) error {
	if len(catalog.Attributes) == 0 || len(catalog.Attributes) > maxCatalogEntries {
		return fmt.Errorf("attributes must contain 1–%d entries", maxCatalogEntries)
	}
	if len(catalog.Weapons) == 0 || len(catalog.Weapons) > maxCatalogEntries {
		return fmt.Errorf("weapons must contain 1–%d entries", maxCatalogEntries)
	}
	seen := make(map[string]bool, len(catalog.Attributes))
	for _, attr := range catalog.Attributes {
		if err := validateCatalogID(attr.ID, seen); err != nil {
			return fmt.Errorf("attributes: %w", err)
		}
		if attr.MaxLevel < 0 || attr.MaxLevel > 255 {
			return fmt.Errorf("attribute %s max_level is outside 0–255", attr.ID)
		}
	}
	seen = make(map[string]bool, len(catalog.Weapons))
	for _, weapon := range catalog.Weapons {
		if err := validateCatalogID(weapon.ID, seen); err != nil {
			return fmt.Errorf("weapons: %w", err)
		}
	}
	return nil
}

func readCatalogCache(path string) (catalogCache, error) {
	data, err := readCatalogFile(path, maxCatalogCacheBytes)
	if err != nil {
		return catalogCache{}, err
	}
	var catalog catalogCache
	if err := json.Unmarshal(data, &catalog); err != nil {
		return catalogCache{}, err
	}
	if err := validateCatalog(catalog); err != nil {
		return catalogCache{}, err
	}
	return catalog, nil
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

// FetchDataFromGitHub downloads one supported catalog from upstream main over HTTPS.
func FetchDataFromGitHub(filename string) ([]byte, error) {
	if filename != "attributes.json" && filename != "weapons.json" {
		return nil, fmt.Errorf("unsupported catalog file %q", filename)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("catalog redirects are not allowed")
		},
	}
	resp, err := client.Get(GitHubDataBaseURL + "/" + filename)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", filename, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to fetch %s: HTTP %d", filename, resp.StatusCode)
	}
	data, readErr := readAllLimited(resp.Body, maxCatalogFileBytes)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}
	return data, nil
}

// UpdateDataFromGitHub fetches, validates and atomically persists both catalogs.
// It does not publish globals: call PublishCatalogUpdate on the UI thread after success.
func UpdateDataFromGitHub() (*CatalogUpdate, error) {
	attrData, err := FetchDataFromGitHub("attributes.json")
	if err != nil {
		return nil, err
	}
	var cached catalogCache
	if err := json.Unmarshal(attrData, &cached.Attributes); err != nil {
		return nil, fmt.Errorf("attributes parse: %w", err)
	}
	weaponData, err := FetchDataFromGitHub("weapons.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(weaponData, &cached.Weapons); err != nil {
		return nil, fmt.Errorf("weapons parse: %w", err)
	}
	if err := validateCatalog(cached); err != nil {
		return nil, err
	}
	cachePath, err := catalogCachePath()
	if err != nil {
		return nil, fmt.Errorf("catalog refresh was not saved: %w", err)
	}
	data, err := json.Marshal(cached)
	if err != nil {
		return nil, fmt.Errorf("encode catalog cache: %w", err)
	}
	if len(data) > maxCatalogCacheBytes {
		return nil, fmt.Errorf("catalog cache exceeds %d-byte limit", maxCatalogCacheBytes)
	}

	DataLock.RLock()
	dataPath := DataPath
	DataLock.RUnlock()
	prepared := loadBaseCatalog(dataPath)
	prepared.Attributes = mergeCatalogAttributes(prepared.Attributes, cached.Attributes)
	prepared.Weapons = mergeCatalogWeapons(prepared.Weapons, cached.Weapons)
	applyCatalogPolicy(&prepared)

	if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
		return nil, fmt.Errorf("catalog refresh was not saved: %w", err)
	}
	// Replace the fetched pair together; a failed refresh leaves the previous pair intact.
	if err := safeio.WriteFile(cachePath, data, 0600); err != nil {
		return nil, fmt.Errorf("catalog refresh was not saved: %w", err)
	}
	return &CatalogUpdate{
		Summary: fmt.Sprintf("Saved attributes (%d items) and weapons (%d items) to user cache", len(cached.Attributes), len(cached.Weapons)),
		catalog: prepared,
	}, nil
}

// PublishCatalogUpdate publishes a successful refresh without HTTP or filesystem work.
// The caller must run this on the UI thread so existing catalog views remain safe.
func PublishCatalogUpdate(update *CatalogUpdate) {
	DataLock.Lock()
	defer DataLock.Unlock()
	LoadedAttributes, LoadedWeapons = update.catalog.Attributes, update.catalog.Weapons
}
