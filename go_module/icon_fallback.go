package main

// Boxicon-style fallback iconography for attributes that don't have
// an in-game HUD icon. Maps an attribute ID (or its display name) to
// one of the embedded SVGs in assets/boxicons/, picked by keyword
// matching against the row's identity.
//
// Resolution order in resolveIconResource (mbch_editor.go):
//  1. Embedded MBII HUD icon (assets/icons/attributes/, weapons/, …)
//  2. VFS-backed game asset (PK3, loose file)
//  3. Boxicon fallback (this file)
//  4. nil — caller falls back to theme.QuestionIcon()
//
// The set is intentionally small (~25 icons) covering archetypes
// rather than 1:1 mapping. A "shield" boxicon stands in for any
// defensive attribute (Saber Defense, Force Protect, Beskar, etc.)
// when no specific HUD art exists. This keeps rows readable without
// pretending we have authoritative iconography for every attribute.

import (
	"fyne.io/fyne/v2"
	"strings"
)

// boxiconKeyword maps a fragment of an attribute ID/name (in lowercase)
// to a boxicon basename. Order matters — the first match wins, so put
// more-specific keywords ahead of generic ones (e.g. "darkrage" before
// "rage", "saber_defense" before "saber").
//
// Slice-of-pairs (not map) for stable iteration order.
var boxiconKeyword = []struct {
	needle string
	icon   string
}{
	// Movement / mobility — checked before generic "force" matches so
	// MB_ATT_DASH doesn't get caught by something later.
	{"jetpack", "jetpack"},
	{"jet_jumps", "jetpack"},
	{"fuel", "fuel"},
	{"dash", "dash"},
	{"speedlunge", "dash"},
	{"barge", "dash"},
	{"levitation", "wave"},
	{"speed", "footstep"},
	{"agility", "footstep"},
	{"footwork", "footstep"},

	// Defense
	{"shield", "shield"},
	{"defense", "shield"},
	{"absorb", "shield"},
	{"protect", "shield"},
	{"beskar", "shield"},
	{"armour", "shield"},
	{"armor", "shield"},
	{"block", "shield"},

	// Health / regen
	{"health", "heart"},
	{"heal", "heart"},
	{"bacta", "droplet"},
	{"medpac", "heart"},
	{"medkit", "heart"},
	{"stim", "droplet"},
	{"regen", "refresh"},

	// Force pool / energy
	{"forcepool", "atom"},
	{"force_pool", "atom"},
	{"fp_battery", "battery"},
	{"battery", "battery"},
	{"stamina", "battery"},
	{"energy", "battery"},
	{"resource_regen", "refresh"},

	// Force powers — bolt for offensive force
	{"lightning", "bolt"},
	{"darkrage", "flame"},
	{"rage", "flame"},
	{"grip", "fist"},
	{"drain", "droplet"},
	{"destruction", "explosion"},
	{"deadlysight", "eye"},
	{"stasis", "snowflake"},
	{"telepathy", "wave"},
	{"push", "wave"},
	{"pull", "swap"},
	{"repulse", "wave"},
	{"projection", "wave"},
	{"fp_see", "eye"},
	{"fp_sense", "eye"},
	{"force", "atom"},

	// Saber
	{"saberthrow", "saber"},
	{"saber", "saber"},

	// Weapons / ammo
	{"sniper", "target"},
	{"disruptor", "target"},
	{"trad_bowcaster", "target"},
	{"projectile_rifle", "target"},
	{"flechette", "explosion"},
	{"rocket", "explosion"},
	{"plx", "explosion"},
	{"frag", "explosion"},
	{"thermal", "explosion"},
	{"micro_grenade", "explosion"},
	{"grenade", "explosion"},
	{"trip_mine", "box"},
	{"sticky_bomb", "box"},
	{"det_pack", "box"},
	{"pulse", "bolt"},
	{"sonic", "wave"},
	{"cryo", "snowflake"},
	{"fire", "flame"},
	{"flame", "flame"},
	{"flamethrower", "flame"},

	// Misc utility
	{"poison", "skull"},
	{"hack", "wrench"},
	{"weld", "wrench"},
	{"hull_repair", "wrench"},
	{"data_spike", "wrench"},
	{"security_interface", "wrench"},
	{"binoculars", "eye"},
	{"goggles", "eye"},
	{"cloak", "eye"},
	{"disguise", "eye"},
	{"ammo", "bag"},
	{"firepower", "bag"},
	{"gear", "gear"},
	{"shock", "bolt"},
	{"viewbaseddrain", "eye"},
	{"baseseeker", "target"},

	// Multipliers + cooldowns + scaling
	{"multiplier", "swap"},
	{"cooldown", "timer"},
	{"rate_of_fire", "timer"},
	{"rof", "timer"},

	// Weapon-attribute archetypes — anything that maps a class's
	// access to a specific weapon. Keywords match the attribute ID's
	// suffix (MB_ATT_BLASTER / MB_ATT_REPEATER / …) so weapon rows in
	// the Attributes tab pick up a glyph even when no MBII HUD art
	// is wired. Pistols → bolt, rifles/snipers → target, launchers
	// → explosion, melee → fist/saber/box.
	{"_pistol", "bolt"},
	{"pistol", "bolt"},
	{"_blaster", "bolt"},
	{"blaster", "bolt"},
	{"_repeater", "target"},
	{"repeater", "target"},
	{"_carbine", "target"},
	{"carbine", "target"},
	{"_rifle", "target"},
	{"rifle", "target"},
	{"a280", "target"},
	{"dlt", "target"},
	{"dc_carbine", "target"},
	{"westar", "target"},
	{"e_22", "target"},
	{"e22", "target"},
	{"ee3", "target"},
	{"ee4", "target"},
	{"amban", "target"},
	{"t21", "target"},
	{"shotgun", "target"},
	{"minigun", "target"},
	{"clonerifle", "target"},
	{"ionrifle", "bolt"},
	{"concussion", "wave"},
	{"demp2", "bolt"},
	{"bowcaster", "bolt"},
	{"thrower", "flame"},
	{"ugl", "explosion"},
	{"mgl", "explosion"},
	{"plx", "explosion"},
	{"rocket_launcher", "explosion"},
	{"launcher", "explosion"},
	{"sticky_bombs", "box"},
	{"det_pack", "box"},
	{"trip_mines", "box"},
	{"base_td", "explosion"},
	{"thermals", "explosion"},
	{"frags", "explosion"},
	{"micro_grenades", "explosion"},
	{"pulse_grenades", "bolt"},
	{"fire_grenades", "flame"},
	{"sonic_detonator", "wave"},
	{"cryoban_grenades", "snowflake"},
	{"flechette_nades", "explosion"},
	{"repeater_nades", "explosion"},
	{"whistlingbird", "wave"},
	{"electro_staff", "bolt"},
	{"stun_baton", "bolt"},
	{"knife", "saber"},
	{"sword", "saber"},
	{"poison_dart", "skull"},
	{"quickthrow", "swap"},
	{"quickdraw", "swap"},
	{"drone", "atom"},
	{"firepower", "bolt"},

	// Generic family fallbacks last — caught only if specifics miss.
	{"weapon", "target"},
}

// keywordIcon returns the boxicon basename whose keyword first matches
// the given haystack (lowercased ID + name concatenation). Returns ""
// if nothing matches — caller should treat that as "no fallback."
func keywordIcon(id, name string) string {
	hay := strings.ToLower(id + " " + name)
	for _, m := range boxiconKeyword {
		if strings.Contains(hay, m.needle) {
			return m.icon
		}
	}
	return ""
}

// loadBoxiconResource reads a boxicon SVG from the embedded FS and
// wraps it as a fyne.StaticResource. SVGs are tiny (~200 bytes) so
// the read cost is negligible; the returned resource is suitable for
// use with widget.NewIcon / canvas.NewImageFromResource.
func loadBoxiconResource(basename string) fyne.Resource {
	if basename == "" {
		return nil
	}
	path := "assets/boxicons/" + basename + ".svg"
	data, err := embedBoxicons.ReadFile(path)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(basename+".svg", data)
}

// FallbackIconForAttribute returns a boxicon resource keyed off an
// attribute's ID + name, or nil if no keyword matches. Public so the
// resolveIconResource path in mbch_editor.go can fall through to it
// after the embedded HUD lookup misses.
func FallbackIconForAttribute(id, name string) fyne.Resource {
	icon := keywordIcon(id, name)
	if icon == "" {
		return nil
	}
	return loadBoxiconResource(icon)
}
