package main

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
