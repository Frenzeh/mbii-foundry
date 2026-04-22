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

// ResolveWeaponIcon finds the icon for a WP_ ID
func (ir *IconResolver) ResolveWeaponIcon(wpID string) string {
	// Standard pattern: gfx/hud/w_icon_{name}
	// e.g. WP_BLASTER_PISTOL -> w_icon_blaster_pistol

	// Special overrides
	overrides := map[string]string{
		"WP_MELEE": "gfx/hud/w_icon_melee",
		"WP_SABER": "gfx/hud/w_icon_lightsaber",
	}
	if val, ok := overrides[wpID]; ok {
		return val
	}

	// General case
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
