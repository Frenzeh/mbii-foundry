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
	"WP_WELD_PULSE":    "w_icon_blaster_pistol",
	"WP_WELD_BEAM":     "w_icon_blaster_pistol",
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

// ResolveAttributeIcon finds the icon for an MB_ATT_ ID
func (ir *IconResolver) ResolveAttributeIcon(attID string) string {
	// Patterns vary widely.
	// Force powers: gfx/forcepowers/{name} usually? Or gfx/hud/force_{name}?
	// Actually most attributes don't have HUD icons unless they are abilities.

	// Common mapping based on observation
	suffix := strings.ToLower(strings.TrimPrefix(attID, "MB_ATT_"))

	// Try ability icon pattern
	candidates := []string{
		fmt.Sprintf("gfx/hud/chk_%s", suffix),
		fmt.Sprintf("gfx/hud/i_icon_%s", suffix),
		fmt.Sprintf("gfx/2d/forcepowers/%s", suffix),
	}

	// Handle FP_ prefix (e.g. MB_ATT_FP_PUSH -> fp_push -> push)
	if strings.HasPrefix(suffix, "fp_") {
		short := strings.TrimPrefix(suffix, "fp_")
		candidates = append(candidates, fmt.Sprintf("gfx/2d/forcepowers/%s", short))
		candidates = append(candidates, fmt.Sprintf("gfx/hud/force_%s", short))
		candidates = append(candidates, fmt.Sprintf("gfx/menus/forcepowers/%s", short))
	}

	if ir.vfs != nil {
		for _, c := range candidates {
			// Check if exists (with extensions)
			if ir.checkExists(c) {
				return c
			}
		}
	}

	return candidates[0] // Return best guess
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
