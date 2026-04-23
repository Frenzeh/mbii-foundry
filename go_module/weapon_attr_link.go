package main

// weaponAttributeLink encodes the canonical WP_* ↔ MB_ATT_* pairing
// MBII's own wiki + real Legends content use. Most weapons have an
// attribute that controls their level (0 = off, 1–3 = rank / ammo /
// variant); the name doesn't always match the weapon's enum (e.g.
// WP_BLASTER_PISTOL ↔ MB_ATT_PISTOL, WP_FRAG_NADE ↔ MB_ATT_FRAGS).
//
// Foundry uses this to:
//   1. Show the paired attribute as a subtitle under each weapon
//      row, teaching the user which attribute controls which weapon
//      without them having to know the enum by heart.
//   2. Future: auto-suggest adding the paired MB_ATT at rank 1 when
//      the user checks a weapon. Task #38 left that out of the
//      first pass to avoid surprising edits.
//
// A handful of weapons (WP_MELEE, WP_SABER, WP_NONE) genuinely have
// no paired attribute — their "level" is controlled elsewhere
// (saber styles, force powers, etc.). Those map to "" and the UI
// shows no subtitle for them.
//
// Source: game/bg_misc.c weapon_* item defs + mbii wiki page
// "MBCH Guide" + legends .mbch corpus survey (task #37 report).
var weaponAttributeLink = map[string]string{
	"WP_STUN_BATON":     "MB_ATT_STUN_BATON",
	"WP_BRYAR_PISTOL":   "MB_ATT_PISTOL",
	"WP_CLONE_PISTOL":   "MB_ATT_CLONE_PISTOL",
	"WP_MANDO_PISTOL":   "MB_ATT_MANDO_PISTOL",
	"WP_BLASTER":        "MB_ATT_BLASTER",
	"WP_DC_CARBINE":     "MB_ATT_DC_CARBINE",
	"WP_CR2":            "MB_ATT_CR2",
	"WP_E_22":           "MB_ATT_E_22",
	"WP_HEAVY_PISTOL":   "MB_ATT_HEAVY_PISTOL",
	"WP_DLT19":          "MB_ATT_DLT19",
	"WP_TRAD_BOWCASTER": "MB_ATT_TRAD_BOWCASTER",
	"WP_DISRUPTOR":      "MB_ATT_DISRUPTOR",
	"WP_BOWCASTER":      "MB_ATT_BOWCASTER",
	"WP_REPEATER":       "MB_ATT_REPEATER",
	"WP_CLONE_RIFLE":    "MB_ATT_CLONERIFLE",
	"WP_THROWER":        "MB_ATT_THROWER",
	"WP_MINIGUN":        "MB_ATT_MINIGUN",
	"WP_DEMP2":          "MB_ATT_DEMP2",
	"WP_SHOTGUN":        "MB_ATT_SHOTGUN",
	"WP_FLECHETTE":      "MB_ATT_FLECHETTE",
	"WP_A280":           "MB_ATT_A280",
	"WP_DLT20A":         "MB_ATT_DLT20A",
	"WP_M5":             "MB_ATT_WESTARM5",
	"WP_T21":            "MB_ATT_T21",
	"WP_ROCKET_LAUNCHER": "MB_ATT_ROCKET_LAUNCHER",
	"WP_PLX1":           "MB_ATT_PLX1",
	"WP_THERMAL":        "MB_ATT_BASE_TD",
	"WP_FRAG_NADE":      "MB_ATT_FRAGS",
	"WP_REAL_TD":        "MB_ATT_THERMALS",
	"WP_TRIP_MINE":      "MB_ATT_TRIP_MINES",
	"WP_PULSE_NADE":     "MB_ATT_PULSE_GRENADES",
	"WP_FIRE_NADE":      "MB_ATT_FIRE_GRENADES",
	"WP_SONIC_NADE":     "MB_ATT_SONIC_DETONATOR",
	"WP_CRYO_NADE":      "MB_ATT_CRYOBAN_GRENADES",
	"WP_CONC_NADE":      "MB_ATT_MICRO_GRENADES",
	"WP_DET_PACK":       "MB_ATT_DET_PACK",
	"WP_CONCUSSION":     "MB_ATT_CONCUSSION",
	"WP_SBD":            "MB_ATT_FIREPOWER",
	"WP_BRYAR_OLD":      "MB_ATT_BRYAR_OLD",
	"WP_EE3":            "MB_ATT_EE3",
	"WP_EE4":            "MB_ATT_EE4",
	"WP_AMBAN":          "MB_ATT_AMBAN",
	"WP_PROJ":           "MB_ATT_PROJECTILE_RIFLE",
	"WP_UGL":            "MB_ATT_UGL",
	"WP_MGL":            "MB_ATT_MGL",

	// Weapons with no paired attribute — the empty mapping signals
	// "no subtitle, not a config-by-rank weapon".
	"WP_MELEE": "",
	"WP_SABER": "",
	"WP_NONE":  "",
}

// CanonicalAttributeFor returns the paired MB_ATT_* for a weapon ID,
// or "" if the weapon has no linked attribute (WP_MELEE, WP_SABER,
// or an ID not in the table yet). Stable public function so future
// features (linked-card editor, auto-suggest) can rely on it.
func CanonicalAttributeFor(wpID string) string {
	return weaponAttributeLink[wpID]
}
