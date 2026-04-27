package main

import (
	"fmt"
	"strings"
)

// IconResolver handles the logic for finding the correct icon path
// based on game entity types (Attributes, Weapons, Models).
type IconResolver struct {
	vfs *VirtualFileSystem
}

func NewIconResolver(vfs *VirtualFileSystem) *IconResolver {
	return &IconResolver{vfs: vfs}
}

// weaponIconAliases maps WP_* enum values to the actual HUD icon
// filename MBII uses. Source of truth: item defs in moviebattles/
// game/bg_misc.c, where each weapon_* entry ships with its in-HUD
// icon path. MBII's filenames are *not* derivable from the enum
// (e.g. WP_CLONE_PISTOL → "clonepistol", WP_THROWER → "cr-24_flamerifle")
// so we keep an explicit map. When bg_misc.c changes, regenerate by
// grep'ing `"gfx/hud/w_icon_"` lines paired with the WP_* at the
// end of each row.
//
// Keys are the full WP_* enum name. Values are the basename without
// extension — ResolveWeaponIcon prepends "gfx/hud/" so embedded-icon
// lookup (which keys on basename) still works.
var weaponIconAliases = map[string]string{
	// Default picks from bg_misc.c's weapon_* item defs.
	"WP_NONE":          "w_icon_melee",
	"WP_STUN_BATON":    "w_icon_stunbaton",
	"WP_MELEE":         "w_icon_melee",
	"WP_SABER":         "w_icon_lightsaber",
	"WP_BRYAR_PISTOL":  "w_icon_blaster_pistol",
	"WP_CLONE_PISTOL":  "w_icon_clonepistol",
	"WP_MANDO_PISTOL":  "w_icon_westar",
	"WP_BLASTER":       "w_icon_e11",
	"WP_DC_CARBINE":    "w_icon_dc-15s",
	"WP_CR2":           "w_icon_cr2pistol",
	"WP_E_22":          "w_icon_e_22",
	"WP_HEAVY_PISTOL":  "w_icon_imp_pistol",
	"WP_DLT19":         "w_icon_dlt19scoped",
	"WP_TRAD_BOWCASTER": "w_icon_wbowcaster1",
	"WP_DISRUPTOR":     "w_icon_disruptor",
	"WP_BOWCASTER":     "w_icon_bowcaster",
	"WP_REPEATER":      "w_icon_repeater",
	"WP_CLONE_RIFLE":   "w_icon_clonerifle",
	"WP_THROWER":       "w_icon_cr-24_flamerifle",
	"WP_MINIGUN":       "w_icon_rotary_cannon",
	"WP_DEMP2":         "w_icon_demp2",
	"WP_SHOTGUN":       "w_icon_cp-50_repeater",
	"WP_FLECHETTE":     "w_icon_flechette",
	"WP_A280":          "w_icon_a280",
	"WP_DLT20A":        "w_icon_dlt20a",
	"WP_M5":            "w_icon_cw-w5",
	"WP_T21":           "w_icon_t-21",
	"WP_ROCKET_LAUNCHER": "w_icon_merrsonn",
	"WP_PLX1":          "w_icon_plx-1",
	"WP_THERMAL":       "w_icon_thermal",
	"WP_FRAG_NADE":     "w_icon_fraggrenade",
	"WP_REAL_TD":       "w_icon_realtd",
	"WP_TRIP_MINE":     "w_icon_tripmine",
	"WP_PULSE_NADE":    "w_icon_rpgren",
	"WP_FIRE_NADE":     "w_icon_plasma",
	"WP_SONIC_NADE":    "w_icon_sonic_det",
	"WP_CRYO_NADE":     "w_icon_cryobangrenade",
	"WP_CONC_NADE":     "w_icon_v-59_conc",
	"WP_DET_PACK":      "w_icon_detpack",
	"WP_CONCUSSION":    "w_icon_c_rifle",
	"WP_SBD":           "w_icon_sbdarm",
	"WP_BRYAR_OLD":     "w_icon_briar",
	"WP_EE3":           "w_icon_ee-3",
	"WP_EE4":           "w_icon_ee-4",
	"WP_AMBAN":         "w_icon_mandorifle",
	"WP_PROJ":          "w_icon_proj_rifle",
	"WP_UGL":           "w_icon_relby_v10",
	"WP_MGL":           "w_icon_upl",
}

// ResolveWeaponIcon finds the HUD icon path for a WP_ ID. Looks up
// the alias map first (authoritative — MBII's filenames rarely match
// the enum suffix) and falls back to the naive "gfx/hud/w_icon_<lower>"
// pattern for IDs that aren't in the table yet (new custom weapons,
// experimental enums).
func (ir *IconResolver) ResolveWeaponIcon(wpID string) string {
	if alias, ok := weaponIconAliases[wpID]; ok {
		return "gfx/hud/" + alias
	}
	suffix := strings.ToLower(strings.TrimPrefix(wpID, "WP_"))
	return fmt.Sprintf("gfx/hud/w_icon_%s", suffix)
}

// attributeIconAliases maps MB_ATT_* enum values to icon basenames.
// Unlike weapons, MBII doesn't ship a canonical 1:1 attribute→icon
// table — content authors pick from the icon_stats_* set ad-hoc in
// their .mbch customSpecIcon fields. This table is a best-effort
// mapping curated from the icon_stats_* pool in MBAssets2.pk3 so
// the attribute grid renders meaningful art instead of blank rows.
// Multiple attributes deliberately reuse the same icon (e.g. all
// armor-style attributes → icon_stats_armor) — there's no harm,
// and having a thematically-appropriate icon beats none.
var attributeIconAliases = map[string]string{
	// Direct weapon counterparts.
	"MB_ATT_A280":             "icon_stats_a280",
	"MB_ATT_BLASTER":          "icon_stats_e11",
	"MB_ATT_BOWCASTER":        "icon_stats_bowcaster",
	"MB_ATT_CLONERIFLE":       "icon_stats_clonerifle",
	"MB_ATT_CLONE_PISTOL":     "icon_stats_clonepistol",
	"MB_ATT_CONCUSSION":       "icon_stats_conc",
	"MB_ATT_DEMP2":            "icon_stats_emp",
	"MB_ATT_DISRUPTOR":        "icon_stats_disruptor",
	"MB_ATT_DLT20A":           "icon_stats_dlt20a",
	"MB_ATT_EE3":              "icon_stats_ee3",
	"MB_ATT_EE4":              "icon_stats_ee3", // no dedicated EE4 icon; EE3 is closest
	"MB_ATT_PISTOL":           "icon_stats_pistol",
	"MB_ATT_HEAVY_PISTOL":     "icon_stats_pistol",
	"MB_ATT_MANDO_PISTOL":     "icon_stats_pistol",
	"MB_ATT_IMP_PISTOL":       "icon_stats_pistol",
	"MB_ATT_PLX1":             "icon_stats_plx",
	"MB_ATT_PROJECTILE_RIFLE": "icon_stats_proj",
	"MB_ATT_ROCKET":           "icon_stats_rocket",
	"MB_ATT_ROCKET_LAUNCHER":  "icon_stats_rocket",
	"MB_ATT_T21":              "icon_stats_t21",
	"MB_ATT_WESTARM5":         "icon_stats_westarm5",

	// Grenades / explosives.
	"MB_ATT_FIRE_GRENADES":   "icon_stats_fire",
	"MB_ATT_FRAGS":           "icon_stats_frag",
	"MB_ATT_CRYOBAN_GRENADES": "icon_stats_sonic", // no cryo icon; sonic is a close thematic match
	"MB_ATT_SONIC_DETONATOR": "icon_stats_sonic",
	"MB_ATT_THERMALS":        "icon_stats_thermal",
	"MB_ATT_PULSE_GRENADES":  "icon_stats_concblob",
	"MB_ATT_BASE_TD":         "icon_stats_thermal",

	// Darts.
	"MB_ATT_POISON_DART":   "icon_stats_pdart",
	"MB_ATT_TRACKING_DART": "icon_stats_tdart",

	// Armor / durability — all share the armor icon since MBII doesn't
	// ship dedicated variants.
	"MB_ATT_ARMOUR":          "icon_stats_armor",
	"MB_ATT_BLAST_ARMOUR":    "icon_stats_armor",
	"MB_ATT_MAGNETIC_PLATING": "icon_stats_armor",
	"MB_ATT_CORTOSIS":        "icon_stats_armor",
	"MB_ATT_DURABILITY":      "icon_stats_armor",
	"MB_ATT_HULL_STRENGTH":   "icon_stats_armor",
	"MB_ATT_DEKA_SHIELD":     "icon_stats_armor",
	"MB_ATT_DEKA_HULL":       "icon_stats_armor",
	"MB_ATT_WOOKIE_HEALTH":   "icon_stats_health",

	// Health / bacta / healing.
	"MB_ATT_HEALTH":     "icon_stats_health",
	"MB_ATT_HEALING":    "i_icon_bacta",
	"MB_ATT_MEDI_PACK":  "i_icon_medkit",
	"MB_ATT_STIMPACK":   "i_icon_medkit",
	"MB_ATT_AMMO_PACK":  "i_icon_medkit",

	// Speed / movement.
	"MB_ATT_BASESPEED": "icon_stats_movespeed",
	"MB_ATT_ACROBACY":  "icon_stats_movespeed",
	"MB_ATT_DEXTERITY": "icon_stats_movespeed",

	// Jetpack / fuel.
	"MB_ATT_FUEL":         "icon_stats_fuel",
	"MB_ATT_FUELREGEN":    "icon_stats_fuel",
	"MB_ATT_JETPACK":      "icon_stats_fuel",
	"MB_ATT_JET_JUMPS":    "i_icon_jetpack",
	"MB_ATT_ASTRO_JUMPJETS": "i_icon_jetpack",

	// Pool / battery / energy.
	"MB_ATT_BATTERY":           "i_icon_battery",
	"MB_ATT_FP_BATTERY":        "i_icon_battery",
	"MB_ATT_SBD_BATTERY":       "i_icon_battery",
	"MB_ATT_FORCEPOOL":         "i_icon_battery",
	"MB_ATT_FORCE_REGEN":       "i_icon_battery",
	"MB_ATT_RESOURCE_REGEN_AMOUNT": "i_icon_battery",
	"MB_ATT_RESOURCE_REGEN_RATE":   "i_icon_battery",
	"MB_ATT_RESOURCE_REGEN_CAP":    "i_icon_battery",

	// Regen rates — health regen → health icon, armor regen →
	// armor icon, block → armor (closest match in the embed set).
	"MB_ATT_HEALTH_REGEN_AMOUNT": "icon_stats_health",
	"MB_ATT_HEALTH_REGEN_RATE":   "icon_stats_health",
	"MB_ATT_HEALTH_REGEN_CAP":    "icon_stats_health",
	"MB_ATT_ARMOUR_REGEN_AMOUNT": "icon_stats_armor",
	"MB_ATT_ARMOUR_REGEN_RATE":   "icon_stats_armor",
	"MB_ATT_ARMOUR_REGEN_CAP":    "icon_stats_armor",
	"MB_ATT_BLOCK_REGEN_AMOUNT":  "icon_stats_armor",
	"MB_ATT_BLOCK_REGEN_RATE":    "icon_stats_armor",
	"MB_ATT_BLOCK_REGEN_CAP":     "icon_stats_armor",

	// Inventory items / class specials with embedded icons.
	"MB_ATT_CLOAK":             "i_icon_cloak",
	"MB_ATT_SPY_DISGUISE":      "i_icon_cloak",
	"MB_ATT_BINOCULARS":        "i_icon_goggles",
	"MB_ATT_SBD_ZOOM":          "i_icon_zoom",
	"MB_ATT_SHIELD":            "i_icon_shieldwall",
	"MB_ATT_SHIELD_PROJ":       "i_icon_shieldwall",
	"MB_ATT_SHIELD_NADE":       "i_icon_shieldwall",
	"MB_ATT_FORCEFIELD":        "i_icon_shieldwall",
	"MB_ATT_PSHIELD":           "i_icon_shieldwall",
	"MB_ATT_PERSONAL_SHIELD":   "i_icon_shieldwall",
	"MB_ATT_SEEKER":            "i_icon_seeker",
	"MB_ATT_BASESEEKER":        "i_icon_seeker",
	"MB_ATT_SENTRY_GUN":        "i_icon_sentrygun",
	"MB_ATT_EWEB":              "i_icon_eweb",
	"MB_ATT_BACTA":             "i_icon_bacta",
	"MB_ATT_BACTA_BOMB":        "i_icon_bacta",
	"MB_ATT_PSD":               "i_icon_psd",
	"MB_ATT_PERSONAL_DEFENSE_SHIELD": "i_icon_psd",
	"MB_ATT_BIG_BACTA":         "i_icon_big_bacta",
	"MB_ATT_GOODIE_KEY":        "i_icon_goodie_key",
	"MB_ATT_SECURITY_KEY":      "i_icon_security_key",
	"MB_ATT_SECURITY_INTERFACE": "i_icon_security_key",

	// More weapon-attribute mappings the original table missed.
	"MB_ATT_REPEATER":         "icon_stats_concblob",
	"MB_ATT_REPEATER_NADES":   "icon_stats_concblob",
	"MB_ATT_FLECHETTE":        "icon_stats_concblob",
	"MB_ATT_FLECHETTE_NADES":  "icon_stats_concblob",
	"MB_ATT_MICRO_GRENADES":   "icon_stats_concblob",
	"MB_ATT_AMBAN":            "icon_stats_t21",
	"MB_ATT_BRYAR_OLD":        "icon_stats_pistol",
	"MB_ATT_CR2":              "icon_stats_pistol",
	"MB_ATT_DLT19":            "icon_stats_t21",
	"MB_ATT_E_22":             "icon_stats_e11",
	"MB_ATT_DC_CARBINE":       "icon_stats_clonerifle",
	"MB_ATT_TRAD_BOWCASTER":   "icon_stats_bowcaster",
	"MB_ATT_THROWER":          "icon_stats_fire",
	"MB_ATT_MINIGUN":          "icon_stats_e11",
	"MB_ATT_SHOTGUN":          "icon_stats_e11",
	"MB_ATT_UGL":              "icon_stats_concblob",
	"MB_ATT_MGL":              "icon_stats_concblob",
	"MB_ATT_UGL_BURST":        "icon_stats_concblob",
	"MB_ATT_UGL_IMPACT":       "icon_stats_concblob",
	"MB_ATT_MGL_BURST":        "icon_stats_concblob",
	"MB_ATT_MGL_IMPACT":       "icon_stats_concblob",
	"MB_ATT_UGL_BURST_MIXED":  "icon_stats_concblob",
	"MB_ATT_DET_PACK":         "icon_stats_thermal",
	"MB_ATT_TRIP_MINES":       "icon_stats_thermal",
	"MB_ATT_STICKY_BOMBS":     "icon_stats_thermal",
	"MB_ATT_REMOTE_DETONATE":  "icon_stats_thermal",
	"MB_ATT_WHISTLINGBIRD":    "icon_stats_sonic",
	"MB_ATT_KNIFE":            "icon_stats_pdart",
	"MB_ATT_SWORD":            "icon_stats_pdart",
	"MB_ATT_ELECTRO_STAFF":    "icon_stats_emp",
	"MB_ATT_STUN_BATON":       "icon_stats_emp",
	"MB_ATT_DRONE":            "i_icon_seeker",
	"MB_ATT_FLAMETHROWER":     "icon_stats_fire",
	"MB_ATT_BESKAR":           "icon_stats_armor",
	"MB_ATT_FIREPOWER":        "icon_stats_e11",
	"MB_ATT_QUICKTHROW":       "icon_stats_thermal",
	"MB_ATT_QUICKDRAW":        "icon_stats_pistol",
}

// forceIconAliases maps MB_ATT_FP_* IDs to the basename of the icon
// MBII actually ships at `gfx/mp/` (per shaders/fp_icons.shader).
// Curated set mirrors the class-builder menu's force-power icons.
// Kept separate from attributeIconAliases because force icons live
// in a different directory (gfx/mp/) and the extracted PNGs land in
// assets/icons/force/ — LoadGameIcon's basename lookup still finds
// them, but callers benefit from a direct FP_* → basename map.
var forceIconAliases = map[string]string{
	"MB_ATT_FP_PUSH":          "new_f_icon_push",
	"MB_ATT_FP_PULL":          "new_f_icon_pull",
	"MB_ATT_FP_LEVITATION":    "new_f_icon_jump",
	"MB_ATT_FP_SPEED":         "new_f_icon_speed",
	"MB_ATT_FP_SEE":           "new_f_icon_sight",
	"MB_ATT_FP_HEAL":          "new_f_icon_lt_heal",
	"MB_ATT_FP_ABSORB":        "new_f_icon_lt_absorb",
	"MB_ATT_FP_PROTECT":       "new_f_icon_lt_protect",
	"MB_ATT_FP_TELEPATHY":     "new_f_icon_lt_mind_trick",
	"MB_ATT_FP_TEAM_HEAL":     "new_f_icon_lt_healother",
	"MB_ATT_FP_TEAM_FORCE":    "new_f_icon_dk_forceother",
	"MB_ATT_FP_DRAIN":         "new_f_icon_dk_drain",
	"MB_ATT_FP_GRIP":          "new_f_icon_dk_grip",
	"MB_ATT_FP_LIGHTNING":     "new_f_icon_dk_l1",
	"MB_ATT_FP_RAGE":          "new_f_icon_dk_rage",
	"MB_ATT_FP_BLIND":         "force_blind",
	"MB_ATT_FP_DESTRUCTION":   "force_destruction",
	"MB_ATT_FP_DEADLYSIGHT":   "deadly_sight",
	"MB_ATT_FP_SABER_OFFENSE": "new_f_icon_saber_attack",
	"MB_ATT_FP_SABER_DEFENSE": "new_f_icon_saber_defend",
	"MB_ATT_FP_SABERTHROW":    "new_f_icon_saber_throw",
	// Force-power bonuses / trainings — closest available embed match.
	"MB_ATT_FP_REPULSE":       "new_f_icon_360_push",
	"MB_ATT_FP_PROJECTION":    "new_f_icon_superpush",
	"MB_ATT_FP_TEAM_ENERGIZE": "new_f_icon_lt_healother",
	"MB_ATT_FP_BATTLEMED":     "new_f_icon_lt_heal",
	"MB_ATT_FP_DOMINATION":    "new_f_icon_lt_mind_trick",
	"MB_ATT_FP_ATTUNEMENT":    "new_f_icon_lt_absorb",
	"MB_ATT_FP_STASIS":        "new_f_icon_dk_grip",
	"MB_ATT_FP_DARKRAGE":      "new_f_icon_dk_rage",
	"MB_ATT_FP_BERSERK":       "new_f_icon_dk_rage",
	"MB_ATT_MANUALSABERTHROW": "new_f_icon_saber_throw",
}

// ResolveAttributeIcon returns the gfx path for an MB_ATT_* ID.
// Prefers the curated alias table (backed by extracted icon_stats_*
// and i_icon_* PNGs embedded in the binary); falls back to the
// legacy chk_/i_icon_/forcepowers pattern for IDs not in the table,
// letting the VFS path still find something when the user has PK3s
// indexed that Foundry hasn't explicitly mapped.
func (ir *IconResolver) ResolveAttributeIcon(attID string) string {
	if alias, ok := forceIconAliases[attID]; ok {
		return "gfx/mp/" + alias
	}
	if alias, ok := attributeIconAliases[attID]; ok {
		return "gfx/menus/alpha/" + alias
	}

	suffix := strings.ToLower(strings.TrimPrefix(attID, "MB_ATT_"))

	candidates := []string{
		fmt.Sprintf("gfx/hud/chk_%s", suffix),
		fmt.Sprintf("gfx/hud/i_icon_%s", suffix),
		fmt.Sprintf("gfx/2d/forcepowers/%s", suffix),
	}

	// Handle FP_ prefix (e.g. MB_ATT_FP_PUSH -> push in forcepowers dir)
	if strings.HasPrefix(suffix, "fp_") {
		short := strings.TrimPrefix(suffix, "fp_")
		candidates = append(candidates, fmt.Sprintf("gfx/2d/forcepowers/%s", short))
		candidates = append(candidates, fmt.Sprintf("gfx/hud/force_%s", short))
		candidates = append(candidates, fmt.Sprintf("gfx/menus/forcepowers/%s", short))
	}

	if ir.vfs != nil {
		for _, c := range candidates {
			if ir.checkExists(c) {
				return c
			}
		}
	}
	return candidates[0]
}

// ResolveClassIcon finds the icon for a Character definition
func (ir *IconResolver) ResolveClassIcon(model, skin, customShader string) string {
	// 1. Explicit UI Shader
	if customShader != "" && customShader != "default" {
		return customShader
	}

	// 2. Standard Model Icon Pattern
	// models/players/{model}/mb2_icon_{skin}
	if model == "" {
		model = "kyle"
	}
	if skin == "" {
		skin = "default"
	}

	return fmt.Sprintf("models/players/%s/mb2_icon_%s", model, skin)
}

func (ir *IconResolver) checkExists(basePath string) bool {
	extensions := []string{".jpg", ".tga", ".png", ".shader"}
	for _, ext := range extensions {
		// This requires VFS to support 'Exists' check efficiently
		// For now, we assume VFS Index has keys.
		// NOTE: VFS keys are usually lower case in our implementation
		path := strings.ToLower(basePath + ext)
		if _, ok := ir.vfs.Index[path]; ok {
			return true
		}
	}
	return false
}
