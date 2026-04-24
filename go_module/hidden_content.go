package main

import "strings"

// Curated lists of class, weapon, and attribute IDs that are defined
// in the game headers but are NOT live in the shipping build — they
// sit behind #ifdef guards (REDACTED_FLAG_04, REDACTED_FLAG_02)
// or are commented out. The editor needs to hide them so testers
// can't accidentally author files referencing content that doesn't
// exist at runtime.
//
// Source of truth: /Users/pj/Library/CloudStorage/SynologyDrive-mcp5/
//   mbii/moviebattles/game/bg_public.h  (classes_t enum)
//   mbii/moviebattles/game/bg_weapons.h (weapon_t enum)
//
// Keep these lists in sync when the game headers change. The public
// GetClasses/GetWeapons/GetAttributes getters filter Hidden==true
// entries; GetAllClasses/GetAllWeapons/GetAllAttributes return
// everything, for the rare case we still need to display a hidden
// entry (e.g. when a loaded .mbch file references one — we don't
// want it silently disappearing from the UI just because the current
// build is missing the feature).

// hiddenClassIDs — classes not live in the base build.
//
// #ifdef REDACTED_FLAG_04 guards #15-#24 (ENFORCER through
// REBMEDIC) — these are WIP content from GCJ's personal branch, not
// shipping. NOCLASS is the zero sentinel; OBSERVER is the spectator
// pseudo-class and doesn't make sense as an editor target.
var hiddenClassIDs = map[string]bool{
	"MB_CLASS_NOCLASS":       true,
	"MB_CLASS_OBSERVER":      true,
	"MB_CLASS_REDACTED_01":      true,
	"MB_CLASS_REDACTED_02":           true,
	"MB_CLASS_REDACTED_03":    true,
	"MB_CLASS_REDACTED_04":     true,
	"MB_CLASS_REDACTED_05":   true,
	"MB_CLASS_REDACTED_06":    true,
	"MB_CLASS_REDACTED_07": true,
	"MB_CLASS_REDACTED_08":     true,
	"MB_CLASS_REDACTED_09":      true,
	"MB_CLASS_REDACTED_10":      true,
}

// hiddenWeaponIDs — weapons not live in the base build.
//
// #ifdef REDACTED_FLAG_02 guards 9 experimental grenades. WP_NONE is
// the zero sentinel. WP_EMPLACED_GUN and WP_TURRET are level objects,
// not player weapons (explicitly above LAST_USEABLE_WEAPON). The
// commented-out classic weapons (WP_GAUNTLET, WP_MACHINEGUN,
// WP_LIGHTNING, WP_RAILGUN, WP_GRAPPLING_HOOK, WP_REDACTED_10)
// live in legacy header comments and aren't referenced by any live
// build — listed here in case any data file still carries them.
var hiddenWeaponIDs = map[string]bool{
	"WP_NONE":          true,
	"WP_REDACTED_01":   true,
	"WP_REDACTED_02":    true,
	"WP_REDACTED_03":      true,
	"WP_REDACTED_04":    true,
	"WP_REDACTED_05":     true,
	"WP_REDACTED_06": true,
	"WP_REDACTED_07":    true,
	"WP_REDACTED_08":   true,
	"WP_REDACTED_09":   true,
	"WP_EMPLACED_GUN":  true,
	"WP_TURRET":        true,
	// Legacy / never-implemented — keep in the hidden list so stale
	// data files don't resurface them in the picker.
	"WP_GAUNTLET":         true,
	"WP_MACHINEGUN":       true,
	"WP_GRENADE_LAUNCHER": true,
	"WP_LIGHTNING":        true,
	"WP_RAILGUN":          true,
	"WP_GRAPPLING_HOOK":   true,
	"WP_REDACTED_10":   true,
}

// hiddenAttributeIDs — attributes not live in the shipping build.
// Covers three groups:
//
//  1. Directly behind an #ifdef in bg_public.h's MB_ATT enum:
//     - REDACTED_FLAG_01 batch (6 force powers)
//     - REDACTED_FLAG_05 (MD_CCTRAINING)
//     - REDACTED_FLAG_06 (MANUALSABERTHROW)
//     - REDACTED_FLAG_07 batch (7 SBD-mode attributes)
//     - REDACTED_FLAG_02 batch (9 grenade attributes)
//  2. Tied to a hidden weapon/class (if the backing feature is
//     hidden, its attribute should be too):
//     - MB_ATT_SPY_* (depends on MB_CLASS_REDACTED_02 which is hidden)
//  3. Enum sentinels and resource markers that aren't user-facing:
//     - MB_ATT_INVALID (zero value), MB_RES_* (resource kinds)
//
// Keep in sync with the enum in game/bg_public.h whenever those
// ifdefs change. The in-app picker filters these out of GetAttributes;
// GetAllAttributes still returns them for loaded files that happen
// to reference a hidden ID.
var hiddenAttributeIDs = map[string]bool{
	// Enum sentinels / non-user-facing.
	"MB_ATT_INVALID":  true,
	"MB_RES_ENERGY":   true,
	"MB_RES_STAMINA":  true,
	"MB_RES_RAGE":     true,
	"MB_RES_BATTERY":  true,

	// #ifdef REDACTED_FLAG_01 — experimental force powers, not in
	// shipping build.
	"MB_ATT_FP_BLIND":       true,
	"MB_ATT_FP_DESTRUCTION": true,
	"MB_ATT_REDACTED_01":  true,
	"MB_ATT_FP_DEADLYSIGHT": true,
	"MB_ATT_REDACTED_02":      true,
	"MB_ATT_REDACTED_03":   true,

	// #ifdef REDACTED_FLAG_05
	"MB_ATT_REDACTED_05": true,

	// #ifdef REDACTED_FLAG_06
	"MB_ATT_REDACTED_06": true,

	// #ifdef REDACTED_FLAG_07 — WIP SBD mode rework.
	"MB_ATT_REDACTED_07":        true,
	"MB_ATT_REDACTED_08":     true,
	"MB_ATT_REDACTED_09":     true,
	"MB_ATT_REDACTED_10": true,
	"MB_ATT_REDACTED_11":  true,
	"MB_ATT_REDACTED_12":    true,
	"MB_ATT_REDACTED_13":   true,

	// #ifdef REDACTED_FLAG_02 — experimental grenade set, matching
	// WP_*_NADE weapons that are also hidden.
	"MB_ATT_REDACTED_14":   true,
	"MB_ATT_REDACTED_15":    true,
	"MB_ATT_REDACTED_16":      true,
	"MB_ATT_REDACTED_17":    true,
	"MB_ATT_REDACTED_18":     true,
	"MB_ATT_REDACTED_19": true,
	"MB_ATT_REDACTED_20":    true,
	"MB_ATT_REDACTED_21":   true,
	"MB_ATT_REDACTED_22":   true,

	// Spy disguise / pistol — only meaningful if MB_CLASS_REDACTED_02 is live,
	// which it isn't (behind REDACTED_FLAG_04).
	"MB_ATT_REDACTED_23": true,
	"MB_ATT_REDACTED_24":   true,

	// Attributes tied to hidden weapons — if the weapon isn't live, the
	// attribute that maps to it can't be usefully purchased either.
	"MB_ATT_GRAPPLE_HOOK":   true, // WP_GRAPPLE_HOOK is hidden
	"MB_ATT_REDACTED_25": true, // WP_REDACTED_10 is hidden

	// MB_ATT_FP_FINAL isn't a real enum member — it's a #define alias
	// for (MB_ATT_PISTOL-1) that marks the end of the force-power run.
	// Keep it in the hidden map as a defensive stop against any stale
	// data that happened to carry it forward.
	"MB_ATT_FP_FINAL": true,

	// MB_ATT_REDACTED_04 is live in the enum but per the header
	// comment only targets NPCs — not a user-facing loadout pick.
	"MB_ATT_REDACTED_04": true,
}

// markHiddenClasses flips the Hidden flag on classes whose IDs appear
// in hiddenClassIDs. Idempotent — safe to call multiple times.
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
// the loaded attribute list. MBII's attribute enum is ~200 entries
// long and the legacy JSON dumps most of them into "General" —
// which then renders as a 60-row scroll of unrelated things. This
// pass re-buckets by ID patterns so the grid's category headers
// actually tell the user what's in each section.
//
// Buckets (matched top-to-bottom; first hit wins):
//   - Force:          MB_ATT_FP_*
//   - Saber:          MB_ATT_SABER*, MB_ATT_SS_*
//   - Regen:          *_REGEN_* family
//   - Multipliers:    *_MULTIPLIER
//   - Supply:         DISP_*, DROP_*, STIM_*, *_PACK, BACTA*, SUPPLYDROP, DISPENSER
//   - Class Specific: WOOKIE*, DEKA*, SBD*, CLONE*, CCTRAINING*, ET_*, MD_*, ASTRO_*, SPY_*, ARC_RIFLE_*
//   - Weapons:        everything in the weapon-attribute allow-set
//                     (mirrors the WP_* ↔ MB_ATT_* relationships)
//   - Advanced:       engine-tuning / movement-tech attributes that
//                     are rarely bought directly — jetpack fuel, turn
//                     rate, hop mechanics, getup anim, etc. Collapsed
//                     by default in the grid so they don't drown out
//                     the bread-and-butter attribute buckets above.
//   - General:        fallback for anything unmatched
//
// Not exhaustive — a handful of utility attributes (STEALTH, DASH,
// RALLY, etc.) land in General intentionally. The goal is to split
// the old megabucket, not to force every attribute into a specific
// bucket at any cost.
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
	// group into one logical tab. Covers authored dispensers, drops,
	// stims, and ammo/medi packs.
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
		strings.HasPrefix(id, "MB_ATT_SBD"),
		strings.HasPrefix(id, "MB_ATT_CLONE"),
		strings.HasPrefix(id, "MB_ATT_SPY_"),
		strings.HasPrefix(id, "MB_ATT_ASTRO_"),
		strings.HasPrefix(id, "MB_ATT_ARC_RIFLE_"),
		strings.HasPrefix(id, "MB_ATT_MANDO_"),
		strings.HasPrefix(id, "MB_ATT_IMP_"),
		id == "MB_ATT_CCTRAINING",
		id == "MB_ATT_ET_CCTRAINING",
		id == "MB_ATT_REDACTED_05",
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
	// editable (FA authors sometimes touch them), just tucked into a
	// collapsed accordion section so they don't crowd the main grid.
	if advancedAttributeIDs[id] {
		return "Advanced"
	}

	return "General"
}

// advancedAttributeIDs — attributes that are live in the enum but
// represent engine-level tuning rather than loadout picks. Kept as
// its own map so the grid can render them in a collapsed "Advanced"
// section the author opens only when they need to touch one.
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
	"MB_ATT_SHIELD_RECHARGE2":  true, // duplicate-of-SHIELD_RECHARGE dev helper
	"MB_ATT_SHIELD_PROJ":       true,
	"MB_ATT_FP_MIRALUKA":       true, // niche force (MIRALUKA_MBATT)
	"MB_ATT_FP_REPULSE":        true, // niche force (NEW360PUSH)
	"MB_ATT_GUNBASH":           true, // melee mechanic, not loadout
	"MB_ATT_FLIPKICK":          true, // movement mechanic
	"MB_ATT_ROSHTAUNT":         true, // dev/test taunt binding
	"MB_ATT_LIGHTS_BEACON":     true,
	"MB_ATT_ANTI_MT":           true,
}

// weaponAttributeIDs is the allow-set for the "Weapons" bucket in
// categorizeAttribute. Kept as a separate map so the set is easy
// to scan in one place rather than a 50-case switch.
var weaponAttributeIDs = map[string]bool{
	"MB_ATT_PISTOL":                    true,
	"MB_ATT_BLASTER":                   true,
	"MB_ATT_DISRUPTOR":                 true,
	"MB_ATT_BOWCASTER":                 true,
	"MB_ATT_SWORD":                     true,
	"MB_ATT_DRONE":                     true,
	"MB_ATT_WPFLAMETHROWER":            true,
	"MB_ATT_CLONERIFLE":                true,
	"MB_ATT_PROJECTILE_RIFLE":          true,
	"MB_ATT_A280":                      true,
	"MB_ATT_THERMALS":                  true,
	"MB_ATT_THERMAL":                   true,
	"MB_ATT_ROCKET":                    true,
	"MB_ATT_ROCKET_LAUNCHER":           true,
	"MB_ATT_PLX1":                      true,
	"MB_ATT_T21":                       true,
	"MB_ATT_CLONE_PISTOL":              true,
	"MB_ATT_HEAVY_PISTOL":              true,
	"MB_ATT_KNIFE":                     true,
	"MB_ATT_ELECTRO_STAFF":             true,
	"MB_ATT_SHOTGUN":                   true,
	"MB_ATT_WESTARM5":                  true,
	"MB_ATT_DLT20A":                    true,
	"MB_ATT_DLT19":                     true,
	"MB_ATT_TRAD_BOWCASTER":            true,
	"MB_ATT_IONRIFLE":                  true,
	"MB_ATT_REPEATER":                  true,
	"MB_ATT_FLECHETTE":                 true,
	"MB_ATT_DEMP2":                     true,
	"MB_ATT_THROWER":                   true,
	"MB_ATT_THROWER_LIGHTNING":         true,
	"MB_ATT_THROWER_ICE":               true,
	"MB_ATT_THROWER_POISON":            true,
	"MB_ATT_THROWER_PLASMA":            true,
	"MB_ATT_THROWER_FLAME":             true,
	"MB_ATT_FLAMETHROWER":              true,
	"MB_ATT_DET_PACK":                  true,
	"MB_ATT_CONCUSSION":                true,
	"MB_ATT_BRYAR_OLD":                 true,
	"MB_ATT_STUN_BATON":                true,
	"MB_ATT_BASE_TD":                   true,
	"MB_ATT_UGL":                       true,
	"MB_ATT_UGL_BURST":                 true,
	"MB_ATT_UGL_IMPACT":                true,
	"MB_ATT_UGL_BURST_MIXED":           true,
	"MB_ATT_MGL":                       true,
	"MB_ATT_MGL_IMPACT":                true,
	"MB_ATT_MGL_BURST":                 true,
	"MB_ATT_STICKY_BOMBS":              true,
	"MB_ATT_MINIGUN":                   true,
	"MB_ATT_PULSE_GRENADES":            true,
	"MB_ATT_FRAGS":                     true,
	"MB_ATT_EE3":                       true,
	"MB_ATT_EE4":                       true,
	"MB_ATT_AMBAN":                     true,
	"MB_ATT_BESKAR":                    true,
	"MB_ATT_WHISTLINGBIRD":             true,
	"MB_ATT_CR2":                       true,
	"MB_ATT_DC_CARBINE":                true,
	"MB_ATT_E_22":                      true,
	"MB_ATT_FIRE_GRENADES":             true,
	"MB_ATT_CRYOBAN_GRENADES":          true,
	"MB_ATT_SONIC_DETONATOR":           true,
	"MB_ATT_MICRO_GRENADES":            true,
	"MB_ATT_TRACKING_DART":             true,
	"MB_ATT_POISON_DART":               true,
	"MB_ATT_TRIP_MINES":                true,
	"MB_ATT_REPEATER_NADES":            true,
	"MB_ATT_FLECHETTE_NADES":           true,
	"MB_ATT_REMOTE_DETONATE":           true,
	"MB_ATT_QUICKTHROW":                true,
	"MB_ATT_QUICKDRAW":                 true,
	"MB_ATT_SABER_NOCAT":               false, // not a weapon; gets Saber category
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
