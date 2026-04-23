package main

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

// hiddenAttributeIDs — attributes tied 1:1 to weapons/classes that
// aren't live. Follows the REDACTED_FLAG_02 + WIP-class pattern: if
// the backing weapon/class is hidden, the attribute that unlocks it
// should be too.
var hiddenAttributeIDs = map[string]bool{
	// Correspond to hidden WP_*_NADE weapons (REDACTED_FLAG_02).
	"MB_ATT_REDACTED_14":   true,
	"MB_ATT_REDACTED_15":    true,
	"MB_ATT_REDACTED_16":      true,
	"MB_ATT_REDACTED_17":    true,
	"MB_ATT_REDACTED_18":     true,
	"MB_ATT_REDACTED_19": true,
	"MB_ATT_REDACTED_20":    true,
	"MB_ATT_REDACTED_21":   true,
	"MB_ATT_REDACTED_22":   true,
	// Spy disguise — only meaningful if MB_CLASS_REDACTED_02 is live, which
	// it isn't (behind REDACTED_FLAG_04).
	"MB_ATT_REDACTED_23": true,
	"MB_ATT_REDACTED_24":   true,
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
