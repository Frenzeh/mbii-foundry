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
	// Melee / Saber
	{ID: "WP_MELEE", Name: "Melee", Category: "Melee/Force", Description: "Hand-to-hand combat using fists and kicks. Always available."},
	{ID: "WP_SABER", Name: "Lightsaber", Category: "Melee/Force", Description: "The elegant weapon of a Jedi or Sith. Requires saber style configuration."},
	{ID: "WP_STUN_BATON", Name: "Stun Baton", Category: "Melee/Force", Description: "Close-quarters electrified stun baton that disorients targets."},

	// Sidearms / Pistols
	{ID: "WP_BLASTER_PISTOL", Name: "DL-44 Blaster Pistol", Category: "Sidearms", Description: "Standard semi-automatic blaster pistol. High accuracy and moderate damage."},
	{ID: "WP_BRYAR_PISTOL", Name: "Bryar Blaster Pistol", Category: "Sidearms", Description: "Versatile sidearm with a high-damage chargeable secondary shot."},
	{ID: "WP_BRYAR_OLD", Name: "Old Bryar Pistol", Category: "Sidearms", Description: "Classic Dark Forces Bryar pistol with reliable baseline damage."},
	{ID: "WP_CLONE_PISTOL", Name: "DC-15S Clone Pistol", Category: "Sidearms", Description: "Dual-wieldable Clone Trooper sidearm capable of charging powerful shots."},
	{ID: "WP_MANDO_PISTOL", Name: "WESTAR-34 Pistols", Category: "Sidearms", Description: "Rapid-fire Mandalorian dual blaster pistols with tight hip-fire accuracy."},
	{ID: "WP_CR2", Name: "CR-2 Heavy Blaster", Category: "Sidearms", Description: "Compact, rapid-firing blaster pistol suited for close-range skirmishing."},
	{ID: "WP_HEAVY_PISTOL", Name: "Heavy Blaster Pistol", Category: "Sidearms", Description: "High-caliber sidearm delivering substantial stopping power."},

	// Rifles / Blasters
	{ID: "WP_BLASTER", Name: "E-11 Blaster", Category: "Rifles", Description: "Standard Imperial stormtrooper rifle with fast automatic rate of fire."},
	{ID: "WP_A280", Name: "A280 Rifle", Category: "Rifles", Description: "Rebel alliance assault rifle offering superior range and high single-shot damage."},
	{ID: "WP_CLONE_RIFLE", Name: "Clone Rifle", Category: "Rifles", Description: "Standard-issue heavy Clone blaster rifle with selectable Ion and Concussion blob attachments."},
	{ID: "WP_M5", Name: "Westar M5", Category: "Rifles", Description: "ARC Trooper multi-role assault weapon supporting zoom scope and grenade launcher attachments."},
	{ID: "WP_T21", Name: "T-21 Heavy Blaster", Category: "Rifles", Description: "Massive repeating blaster delivering tremendous sustained firepower."},
	{ID: "WP_E_22", Name: "E-22 Blaster", Category: "Rifles", Description: "Double-barrel reciprocating blaster rifle providing dense suppressing fire."},
	{ID: "WP_DLT19", Name: "DLT-19 Heavy Blaster", Category: "Rifles", Description: "Heavy support blaster with large power capacity and rapid automatic fire."},
	{ID: "WP_DLT20A", Name: "DLT-20A Blaster", Category: "Rifles", Description: "Long-barreled sniper blaster rifle offering excellent optics and precision at distance."},
	{ID: "WP_EE3", Name: "EE-3 Carbine", Category: "Rifles", Description: "Boba Fett's precision carbine featuring sniper zoom and high-velocity 3-round bursts."},
	{ID: "WP_EE4", Name: "EE-4 Carbine", Category: "Rifles", Description: "Short-range carbine firing widespread bursts ideal for room clearing."},
	{ID: "WP_DISRUPTOR", Name: "Disruptor Rifle", Category: "Rifles", Description: "Lethal sniper rifle that disintegrates unshielded targets on fully charged headshots."},
	{ID: "WP_BOWCASTER", Name: "Bowcaster", Category: "Rifles", Description: "Magnetic accelerator crossbow firing spreading salvos or concentrated charged bolts."},
	{ID: "WP_TRAD_BOWCASTER", Name: "Traditional Bowcaster", Category: "Rifles", Description: "Solid-slug heavy bowcaster with high kinetic impact and knockback."},
	{ID: "WP_REPEATER", Name: "Heavy Repeater", Category: "Rifles", Description: "Rapid-fire slugthrower with an explosive concussion alt-fire launcher."},
	{ID: "WP_DEMP2", Name: "DEMP 2", Category: "Rifles", Description: "Electromagnetic pulse rifle designed to strip shields and neutralize droids."},
	{ID: "WP_FLECHETTE", Name: "Golan Arms Flechette", Category: "Rifles", Description: "Canister launcher firing bouncing flechette darts or proximity explosive mines."},
	{ID: "WP_CONCUSSION", Name: "Concussion Rifle", Category: "Rifles", Description: "Fires high-impact sonic shockwaves that knock targets down on impact."},
	{ID: "WP_SHOTGUN", Name: "CP-50 Repeater", Category: "Rifles", Description: "Short-range spread shotgun delivering devastating close-quarters damage."},
	{ID: "WP_AMBAN", Name: "Amban Rifle", Category: "Rifles", Description: "Mandalorian phase-pulse disintegrator rifle with close-range melee shock taser."},
	{ID: "WP_PROJ", Name: "Projectile Rifle", Category: "Rifles", Description: "Ballistic slug sniper rifle with high bullet velocity and armor penetration."},

	// Heavy Weapons
	{ID: "WP_ROCKET_LAUNCHER", Name: "PLX-1 Rocket Launcher", Category: "Heavy", Description: "Shoulder-fired heavy missile launcher featuring unguided rockets and laser-guided homing."},
	{ID: "WP_PLX1", Name: "PLX-1 Missile Launcher", Category: "Heavy", Description: "Lightweight portable unguided rocket launcher."},
	{ID: "WP_THROWER", Name: "Flamethrower", Category: "Heavy", Description: "Stream flamethrower that ignites infantry and bypasses conventional blaster deflection."},
	{ID: "WP_MINIGUN", Name: "Rotary Cannon", Category: "Heavy", Description: "Multi-barrel heavy rotary blaster cannon with unmatched sustained rate of fire."},
	{ID: "WP_SBD", Name: "Arm Blaster", Category: "Heavy", Description: "Integrated Super Battle Droid dual-arm blasters and heavy suppression cannon."},
	{ID: "WP_UGL", Name: "Universal Grenade Launcher", Category: "Heavy", Description: "Universal grenade launcher firing high-explosive impact, timed, or burst rounds."},
	{ID: "WP_MGL", Name: "Micro Grenade Launcher", Category: "Heavy", Description: "Micro grenade launcher firing sticky bombs for area denial and explosive breaching."},
	{ID: "WP_EQUALIZER", Name: "Equalizer MiniMag", Category: "Heavy", Description: "Burst-fire micro-missile system providing concentrated explosive volleys."},

	// Explosives & Grenades
	{ID: "WP_THERMAL", Name: "Thermal Detonator", Category: "Heavy", Description: "Military-grade timed explosive grenade with a massive destructive blast radius."},
	{ID: "WP_REAL_TD", Name: "Thermal Detonator", Category: "Heavy", Description: "High-yield thermal detonator with heavy explosive payload."},
	{ID: "WP_FRAG_NADE", Name: "Frag Grenade", Category: "Heavy", Description: "Anti-personnel fragmentation grenade releasing deadly shrapnel."},
	{ID: "WP_FIRE_NADE", Name: "Fire Grenade", Category: "Heavy", Description: "Incendiary grenade coating the detonation zone with burning chemicals."},
	{ID: "WP_PULSE_NADE", Name: "Pulse Grenade", Category: "Heavy", Description: "EMP shock grenade that drains shields, battery energy, and ammunition."},
	{ID: "WP_SONIC_NADE", Name: "Sonic Detonator", Category: "Heavy", Description: "Acoustic shockwave grenade that deafens, disorients, and knocks down targets."},
	{ID: "WP_CRYO_NADE", Name: "Cryo Grenade", Category: "Heavy", Description: "Cryogenic freeze grenade that immobilizes and severely slows movement speed."},
	{ID: "WP_CONC_NADE", Name: "Concussion Grenade", Category: "Heavy", Description: "Shockwave grenade delivering concussive force to stagger and knockdown opponents."},
	{ID: "WP_TRIP_MINE", Name: "Trip Mine", Category: "Heavy", Description: "Laser-tripped stationary explosive that detonates upon beam interruption."},
	{ID: "WP_DET_PACK", Name: "Detonation Pack", Category: "Heavy", Description: "Plantable explosive charge triggered remotely with a radio detonator."},
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
