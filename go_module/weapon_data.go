package main

import "image/color"

type WeaponDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"` // "Sidearms", "Rifles", "Heavy", "Melee/Force"

	// Rich Documentation
	Overview string            `json:"overview,omitempty"`
	Tips     []string          `json:"tips,omitempty"`
	Tags     []string          `json:"tags,omitempty"`
	Stats    map[string]string `json:"stats,omitempty"` // New field for numerical stats

	// Hidden marks a weapon that's defined in the enum but not live
	// in the current build (behind an #ifdef in bg_weapons.h, or a
	// commented-out line). Hidden entries are filtered from
	// GetWeapons but remain visible via GetAllWeapons so loaded
	// files referencing custom/experimental weapons still display
	// something instead of silently dropping the line. Populated
	// from hiddenWeaponIDs in data_loader.go at load time.
	Hidden bool `json:"-"`
}

var MBIIWeapons = []WeaponDef{
	// Melee / Special
	{ID: "WP_MELEE", Name: "Melee", Category: "Melee/Force", Description: "Fists and kicks. Always equipped."},
	{ID: "WP_SABER", Name: "Lightsaber", Category: "Melee/Force", Description: "Jedi/Sith weapon. Requires Saber Style attributes."},

	// Sidearms
	{ID: "WP_BLASTER_PISTOL", Name: "DL-44 Pistol", Category: "Sidearms", Description: "Standard blaster pistol. accurate and reliable."},
	{ID: "WP_BRYAR_PISTOL", Name: "Bryar Pistol", Category: "Sidearms", Description: "Chargeable pistol with high damage."},
	{ID: "WP_BRYAR_OLD", Name: "Old Bryar", Category: "Sidearms", Description: "Classic DF2 pistol."},

	// Rifles
	{ID: "WP_BLASTER", Name: "E-11 Blaster", Category: "Rifles", Description: "Standard stormtrooper rifle. Good fire rate."},
	{ID: "WP_DISRUPTOR", Name: "Disruptor Rifle", Category: "Rifles", Description: "Sniper rifle. Vaporizes targets at full charge."},
	{ID: "WP_BOWCASTER", Name: "Bowcaster", Category: "Rifles", Description: "Wookiee crossbow. Fires spread shot or charged bolt."},
	{ID: "WP_REPEATER", Name: "Imperial Repeater", Category: "Rifles", Description: "Fast firing heavy rifle with concussion launcher."},
	{ID: "WP_DEMP2", Name: "DEMP 2", Category: "Rifles", Description: "Ion gun. Effective against droids and shields."},
	{ID: "WP_FLECHETTE", Name: "Golan Arms", Category: "Rifles", Description: "Flechette launcher with mine secondary."},
	{ID: "WP_A280", Name: "A280", Category: "Rifles", Description: "Heavy blaster rifle. High damage, slower fire rate."},

	// Heavy
	{ID: "WP_ROCKET_LAUNCHER", Name: "PLX-1 Launcher", Category: "Heavy", Description: "Fires explosive rockets or smart tracking missiles."},
	{ID: "WP_THERMAL", Name: "Thermal Detonator", Category: "Heavy", Description: "Timed explosive grenade."},
	{ID: "WP_TRIP_MINE", Name: "Trip Mines", Category: "Heavy", Description: "Laser-trip explosives."},
	{ID: "WP_DET_PACK", Name: "Det Packs", Category: "Heavy", Description: "Remote controlled explosives."},

	// Grenades
	{ID: "WP_FIRE_NADE", Name: "Fire Grenade", Category: "Heavy", Description: "Creates a patch of fire."},
	{ID: "WP_PULSE_NADE", Name: "Pulse Grenade", Category: "Heavy", Description: "EMP grenade, drains ammo and force."},
}

// AccentColor returns the per-category accent for tile chrome.
// Used by the Inventory cards so the eye can scan rifles vs heavy at
// a glance. Colors are tuned against the dark theme — avoid pure
// saturated channels (they read as "warning" against the bg).
func (w WeaponDef) AccentColor() color.Color {
	switch w.Category {
	case "Melee/Force":
		return color.NRGBA{R: 180, G: 130, B: 220, A: 255} // violet
	case "Sidearms":
		return color.NRGBA{R: 110, G: 180, B: 220, A: 255} // teal-blue
	case "Rifles":
		return color.NRGBA{R: 110, G: 200, B: 130, A: 255} // green
	case "Heavy":
		return color.NRGBA{R: 220, G: 160, B: 90, A: 255} // amber
	}
	return color.NRGBA{R: 160, G: 160, B: 170, A: 255}
}
