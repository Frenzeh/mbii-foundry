package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Curated visibility rules for classes, weapons, and attributes.
//
// The PUBLIC source carries only the baseline sentinel set (enum zero
// values, resource markers, etc.). Anything beyond that — dev-only
// toggles, experimental content, build variants — is supplied at
// runtime via the optional private overlay (see loadPrivateHidden*).
// Absent the overlay, the editor shows every public entry.
//
// Keep this file narrow: no inline feature names, no references to
// build flags, no roadmap hints. That content lives in the private
// overlay, which is not distributed with public releases.

// hiddenClassIDs — baseline set. Enum sentinels and spectator pseudo-
// classes only; real gameplay classes never appear here in the
// public source.
var hiddenClassIDs = map[string]bool{
	"MB_CLASS_NOCLASS":  true,
	"MB_CLASS_OBSERVER": true,
}

// hiddenWeaponIDs — baseline sentinels.
var hiddenWeaponIDs = map[string]bool{
	"WP_NONE": true,
}

// hiddenAttributeIDs — baseline sentinels + enum terminators.
var hiddenAttributeIDs = map[string]bool{
	"MB_ATT_INVALID":  true,
	"MB_ATT_FP_FINAL": true, // #define alias, not a real enum member
	"MB_RES_ENERGY":   true,
	"MB_RES_STAMINA":  true,
	"MB_RES_RAGE":     true,
	"MB_RES_BATTERY":  true,
}

// init pulls in optional runtime overlays (ID lists + extra ID→doc
// mappings). Any entry in the overlay supplements the baseline;
// nothing in the overlay is required for the editor to function.
func init() {
	loadPrivateHiddenOverlay()
}

// loadPrivateHiddenOverlay reads a JSON file of additional hidden
// IDs from the runtime `private/hidden.json` if present. The overlay
// schema is:
//
//	{
//	  "classes":    ["MB_CLASS_..."],
//	  "weapons":    ["WP_..."],
//	  "attributes": ["MB_ATT_..."]
//	}
//
// Missing file → no-op. Malformed file → logged + ignored (we'd
// rather ship a public-only view than crash).
func loadPrivateHiddenOverlay() {
	path := resolvePrivatePath("hidden.json")
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var doc struct {
		Classes    []string `json:"classes"`
		Weapons    []string `json:"weapons"`
		Attributes []string `json:"attributes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		LogInfo("hidden overlay: parse error, skipping: %v", err)
		return
	}
	for _, id := range doc.Classes {
		hiddenClassIDs[id] = true
	}
	for _, id := range doc.Weapons {
		hiddenWeaponIDs[id] = true
	}
	for _, id := range doc.Attributes {
		hiddenAttributeIDs[id] = true
	}
	LogInfo("hidden overlay applied: %d classes, %d weapons, %d attrs",
		len(doc.Classes), len(doc.Weapons), len(doc.Attributes))
}

// resolvePrivatePath locates a file inside the runtime `private/`
// directory if one exists next to the binary or in the working
// tree. Returns "" when no overlay is installed.
func resolvePrivatePath(name string) string {
	var candidates []string
	if ex, err := os.Executable(); err == nil {
		exDir := filepath.Dir(ex)
		candidates = append(candidates,
			filepath.Join(exDir, "private", name),
			filepath.Join(exDir, "..", "private", name),
			filepath.Join(exDir, "..", "Resources", "private", name),
			filepath.Join(exDir, "..", "..", "private", name),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "private", name),
			filepath.Join(cwd, "..", "private", name),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			if abs == "" {
				abs = c
			}
			return abs
		}
	}
	return ""
}

// markHiddenClasses flips the Hidden flag on classes whose IDs
// appear in hiddenClassIDs. Idempotent — safe to call multiple times.
func markHiddenClasses(classes []ClassDef) {
	for i := range classes {
		if hiddenClassIDs[classes[i].ID] {
			classes[i].Hidden = true
		}
	}
}

func markHiddenWeapons(weapons []WeaponDef) {
	for i := range weapons {
		if hiddenWeaponIDs[weapons[i].ID] {
			weapons[i].Hidden = true
		}
	}
}

func markHiddenAttributes(attrs []AttributeDef) {
	for i := range attrs {
		if hiddenAttributeIDs[attrs[i].ID] {
			attrs[i].Hidden = true
		}
	}
}

// recategorizeAttributes applies a consistent sub-categorization to
// the loaded attribute list. MBII's attribute enum is long enough
// that the legacy JSON dumps most of it into "General" — which then
// renders as a 60-row scroll of unrelated things. This pass re-
// buckets by ID patterns so the grid's category headers actually
// tell the user what's in each section.
//
// Buckets (matched top-to-bottom; first hit wins):
//   - Force:          MB_ATT_FP_*
//   - Saber:          MB_ATT_SABER*, MB_ATT_SS_*
//   - Regen:          *_REGEN_* family
//   - Multipliers:    *_MULTIPLIER
//   - Supply:         DISP_*, DROP_*, STIM_*, *_PACK, BACTA*, SUPPLYDROP, DISPENSER
//   - Class Specific: class-prefixed attributes (Wookiee, Deka, SBD, Clone, etc.)
//   - Weapons:        everything in the weapon-attribute allow-set
//                     (mirrors the WP_* ↔ MB_ATT_* relationships)
//   - Advanced:       engine-tuning / movement-tech attributes that
//                     are rarely bought directly. Collapsed by default
//                     in the grid so they don't drown out the bread-
//                     and-butter attribute buckets above.
//   - General:        fallback for anything unmatched
//
// Not exhaustive — a handful of utility attributes land in General
// intentionally. The goal is to split the old megabucket, not to
// force every attribute into a specific bucket at any cost.
func recategorizeAttributes(attrs []AttributeDef) {
	for i := range attrs {
		attrs[i].Category = categorizeAttribute(attrs[i].ID)
	}
}

func categorizeAttribute(id string) string {
	if strings.HasPrefix(id, "MB_ATT_FP_") {
		return "Force"
	}
	if strings.HasPrefix(id, "MB_ATT_SABER") || strings.HasPrefix(id, "MB_ATT_SS_") {
		return "Saber"
	}
	if strings.Contains(id, "_REGEN_") {
		return "Regen"
	}
	if strings.HasSuffix(id, "_MULTIPLIER") {
		return "Multipliers"
	}

	// Supply / consumables / dispensers — share visual weight so they
	// group into one logical tab.
	if strings.HasPrefix(id, "MB_ATT_DISP_") ||
		strings.HasPrefix(id, "MB_ATT_DROP_") ||
		strings.HasPrefix(id, "MB_ATT_STIM_") ||
		id == "MB_ATT_MEDI_PACK" ||
		id == "MB_ATT_AMMO_PACK" ||
		id == "MB_ATT_SUPPLYDROP" ||
		id == "MB_ATT_BACTA" ||
		id == "MB_ATT_BACTA_BIG" ||
		id == "MB_ATT_STIMPACK" ||
		id == "MB_ATT_SPAWNER" {
		return "Supply"
	}

	// Class-specific attributes — anything that only applies to one
	// specific class's kit.
	switch {
	case strings.HasPrefix(id, "MB_ATT_WOOKIE"),
		strings.HasPrefix(id, "MB_ATT_DEKA"),
		strings.HasPrefix(id, "MB_ATT_CLONE"),
		strings.HasPrefix(id, "MB_ATT_ARC_RIFLE_"),
		strings.HasPrefix(id, "MB_ATT_MANDO_"),
		strings.HasPrefix(id, "MB_ATT_IMP_"),
		id == "MB_ATT_CCTRAINING",
		id == "MB_ATT_ET_CCTRAINING",
		id == "MB_ATT_STRONGBLOBS",
		id == "MB_ATT_HULL_STRENGTH",
		id == "MB_ATT_HULL_REPAIR",
		id == "MB_ATT_SECURITY_INTERFACE",
		id == "MB_ATT_ASSEMBLE",
		id == "MB_ATT_RALLY",
		id == "MB_ATT_WRIST_AMMO",
		id == "MB_ATT_WRISTLASER",
		id == "MB_ATT_HEAT_DUMPS",
		id == "MB_ATT_WELDING_LASER",
		id == "MB_ATT_SHOCK_ARM",
		id == "MB_ATT_FIRE_EXTINGUISHER",
		id == "MB_ATT_DATA_SPIKES",
		id == "MB_ATT_WATER_BREATHING":
		return "Class Specific"
	}

	// Weapons — allow-list keyed on IDs that map 1:1 to a WP_* enum or
	// are clearly weapon-related (grenades, launchers, melee weapons).
	if weaponAttributeIDs[id] {
		return "Weapons"
	}

	// Advanced — engine tuning knobs, movement-tech flags, and internal
	// mechanics that rarely belong in a player-facing loadout. Still
	// editable, just tucked into a collapsed accordion section so it
	// doesn't crowd the main grid.
	if advancedAttributeIDs[id] {
		return "Advanced"
	}

	return "General"
}

// advancedAttributeIDs — attributes that are live in the enum but
// represent engine-level tuning rather than loadout picks.
var advancedAttributeIDs = map[string]bool{
	"MB_ATT_TURN_RATE":         true,
	"MB_ATT_USE_DISTANCE":      true,
	"MB_ATT_VIEWBASEDDRAIN":    true,
	"MB_ATT_INAIR_FORCE_REGEN": true,
	"MB_ATT_BUNNY_HOP":         true,
	"MB_ATT_FLOAT_HOP":         true,
	"MB_ATT_GRAPPLE_HOP":       true,
	"MB_ATT_GETUPS":            true,
	"MB_ATT_FUEL":              true,
	"MB_ATT_FUELREGEN":         true,
	"MB_ATT_WRIST_AMMO":        true,
	"MB_ATT_KNOCKDOWN_ROLL":    true,
	"MB_ATT_TRACKING_BEACON":   true,
	"MB_ATT_SHIELD_RECHARGE2":  true,
	"MB_ATT_SHIELD_PROJ":       true,
	"MB_ATT_FP_MIRALUKA":       true,
	"MB_ATT_FP_REPULSE":        true,
	"MB_ATT_GUNBASH":           true,
	"MB_ATT_FLIPKICK":          true,
	"MB_ATT_ROSHTAUNT":         true,
	"MB_ATT_LIGHTS_BEACON":     true,
	"MB_ATT_ANTI_MT":           true,
}

// weaponAttributeIDs is the allow-set for the "Weapons" bucket in
// categorizeAttribute. Kept as a separate map so the set is easy
// to scan in one place rather than a 50-case switch.
var weaponAttributeIDs = map[string]bool{
	"MB_ATT_PISTOL":            true,
	"MB_ATT_BLASTER":           true,
	"MB_ATT_DISRUPTOR":         true,
	"MB_ATT_BOWCASTER":         true,
	"MB_ATT_SWORD":             true,
	"MB_ATT_DRONE":             true,
	"MB_ATT_WPFLAMETHROWER":    true,
	"MB_ATT_CLONERIFLE":        true,
	"MB_ATT_PROJECTILE_RIFLE":  true,
	"MB_ATT_A280":              true,
	"MB_ATT_THERMALS":          true,
	"MB_ATT_THERMAL":           true,
	"MB_ATT_ROCKET":            true,
	"MB_ATT_ROCKET_LAUNCHER":   true,
	"MB_ATT_PLX1":              true,
	"MB_ATT_T21":               true,
	"MB_ATT_CLONE_PISTOL":      true,
	"MB_ATT_HEAVY_PISTOL":      true,
	"MB_ATT_KNIFE":             true,
	"MB_ATT_ELECTRO_STAFF":     true,
	"MB_ATT_SHOTGUN":           true,
	"MB_ATT_WESTARM5":          true,
	"MB_ATT_DLT20A":            true,
	"MB_ATT_DLT19":             true,
	"MB_ATT_TRAD_BOWCASTER":    true,
	"MB_ATT_IONRIFLE":          true,
	"MB_ATT_REPEATER":          true,
	"MB_ATT_FLECHETTE":         true,
	"MB_ATT_DEMP2":             true,
	"MB_ATT_THROWER":           true,
	"MB_ATT_THROWER_LIGHTNING": true,
	"MB_ATT_THROWER_ICE":       true,
	"MB_ATT_THROWER_POISON":    true,
	"MB_ATT_THROWER_PLASMA":    true,
	"MB_ATT_THROWER_FLAME":     true,
	"MB_ATT_FLAMETHROWER":      true,
	"MB_ATT_DET_PACK":          true,
	"MB_ATT_CONCUSSION":        true,
	"MB_ATT_BRYAR_OLD":         true,
	"MB_ATT_STUN_BATON":        true,
	"MB_ATT_BASE_TD":           true,
	"MB_ATT_UGL":               true,
	"MB_ATT_UGL_BURST":         true,
	"MB_ATT_UGL_IMPACT":        true,
	"MB_ATT_UGL_BURST_MIXED":   true,
	"MB_ATT_MGL":               true,
	"MB_ATT_MGL_IMPACT":        true,
	"MB_ATT_MGL_BURST":         true,
	"MB_ATT_STICKY_BOMBS":      true,
	"MB_ATT_MINIGUN":           true,
	"MB_ATT_PULSE_GRENADES":    true,
	"MB_ATT_FRAGS":             true,
	"MB_ATT_EE3":               true,
	"MB_ATT_EE4":               true,
	"MB_ATT_AMBAN":             true,
	"MB_ATT_BESKAR":            true,
	"MB_ATT_WHISTLINGBIRD":     true,
	"MB_ATT_CR2":               true,
	"MB_ATT_DC_CARBINE":        true,
	"MB_ATT_E_22":              true,
	"MB_ATT_FIRE_GRENADES":     true,
	"MB_ATT_CRYOBAN_GRENADES":  true,
	"MB_ATT_SONIC_DETONATOR":   true,
	"MB_ATT_MICRO_GRENADES":    true,
	"MB_ATT_TRACKING_DART":     true,
	"MB_ATT_POISON_DART":       true,
	"MB_ATT_TRIP_MINES":        true,
	"MB_ATT_REPEATER_NADES":    true,
	"MB_ATT_FLECHETTE_NADES":   true,
	"MB_ATT_REMOTE_DETONATE":   true,
	"MB_ATT_QUICKTHROW":        true,
	"MB_ATT_QUICKDRAW":         true,
}

// filterVisibleClasses / filterVisibleWeapons / filterVisibleAttributes
// return a new slice containing only non-hidden entries, preserving
// input order. Used by the Get*() accessors.
func filterVisibleClasses(in []ClassDef) []ClassDef {
	out := make([]ClassDef, 0, len(in))
	for _, c := range in {
		if !c.Hidden {
			out = append(out, c)
		}
	}
	return out
}

func filterVisibleWeapons(in []WeaponDef) []WeaponDef {
	out := make([]WeaponDef, 0, len(in))
	for _, w := range in {
		if !w.Hidden {
			out = append(out, w)
		}
	}
	return out
}

func filterVisibleAttributes(in []AttributeDef) []AttributeDef {
	out := make([]AttributeDef, 0, len(in))
	for _, a := range in {
		if !a.Hidden {
			out = append(out, a)
		}
	}
	return out
}
