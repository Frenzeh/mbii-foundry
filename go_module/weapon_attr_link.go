package main

// weaponAttributeLink encodes the canonical WP_* ↔ MB_ATT_* pairing.
var weaponAttributeLink = map[string]string{
	"WP_STUN_BATON":      "MB_ATT_STUN_BATON",
	"WP_BRYAR_PISTOL":    "MB_ATT_PISTOL",
	"WP_BLASTER_PISTOL":  "MB_ATT_PISTOL",
	"WP_CLONE_PISTOL":    "MB_ATT_CLONE_PISTOL",
	"WP_MANDO_PISTOL":    "MB_ATT_MANDO_PISTOL",
	"WP_BLASTER":         "MB_ATT_BLASTER",
	"WP_DC_CARBINE":      "MB_ATT_DC_CARBINE",
	"WP_CR2":             "MB_ATT_CR2",
	"WP_E_22":            "MB_ATT_E_22",
	"WP_HEAVY_PISTOL":    "MB_ATT_HEAVY_PISTOL",
	"WP_DLT19":           "MB_ATT_DLT19",
	"WP_TRAD_BOWCASTER":  "MB_ATT_TRAD_BOWCASTER",
	"WP_DISRUPTOR":       "MB_ATT_DISRUPTOR",
	"WP_BOWCASTER":       "MB_ATT_BOWCASTER",
	"WP_REPEATER":        "MB_ATT_REPEATER",
	"WP_CLONE_RIFLE":     "MB_ATT_CLONERIFLE",
	"WP_THROWER":         "MB_ATT_THROWER",
	"WP_MINIGUN":         "MB_ATT_MINIGUN",
	"WP_DEMP2":           "MB_ATT_DEMP2",
	"WP_SHOTGUN":         "MB_ATT_SHOTGUN",
	"WP_FLECHETTE":       "MB_ATT_FLECHETTE",
	"WP_A280":            "MB_ATT_A280",
	"WP_DLT20A":          "MB_ATT_DLT20A",
	"WP_M5":              "MB_ATT_WESTARM5",
	"WP_T21":             "MB_ATT_T21",
	"WP_ROCKET_LAUNCHER": "MB_ATT_ROCKET_LAUNCHER",
	"WP_PLX1":            "MB_ATT_PLX1",
	"WP_THERMAL":         "MB_ATT_BASE_TD",
	"WP_FRAG_NADE":       "MB_ATT_FRAGS",
	"WP_REAL_TD":         "MB_ATT_THERMALS",
	"WP_TRIP_MINE":       "MB_ATT_TRIP_MINES",
	"WP_PULSE_NADE":      "MB_ATT_PULSE_GRENADES",
	"WP_FIRE_NADE":       "MB_ATT_FIRE_GRENADES",
	"WP_SONIC_NADE":      "MB_ATT_SONIC_DETONATOR",
	"WP_CRYO_NADE":       "MB_ATT_CRYOBAN_GRENADES",
	"WP_CONC_NADE":       "MB_ATT_MICRO_GRENADES",
	"WP_DET_PACK":        "MB_ATT_DET_PACK",
	"WP_CONCUSSION":      "MB_ATT_CONCUSSION",
	"WP_SBD":             "MB_ATT_FIREPOWER",
	"WP_BRYAR_OLD":       "MB_ATT_BRYAR_OLD",
	"WP_EE3":             "MB_ATT_EE3",
	"WP_EE4":             "MB_ATT_EE4",
	"WP_AMBAN":           "MB_ATT_AMBAN",
	"WP_PROJ":            "MB_ATT_PROJECTILE_RIFLE",
	"WP_UGL":             "MB_ATT_UGL",
	"WP_MGL":             "MB_ATT_MGL",
	"WP_EQUALIZER":       "MB_ATT_EQUALIZER",
	"WP_WELD_PULSE":      "MB_ATT_WELDING_LASER",
	"WP_WELD_BEAM":       "MB_ATT_WELDING_LASER",
	"WP_BACTA_BOMB":      "MB_ATT_BACTA_BOMB",
	"WP_SMOKE_NADE":      "MB_ATT_SMOKE_NADE",
	"WP_GAS_NADE":        "MB_ATT_GAS_NADE",
	"WP_ROCK_NADE":       "MB_ATT_ROCK_NADE",
	"WP_FLASH_NADE":      "MB_ATT_FLASH_NADE",
	"WP_IMPACT_NADE":     "MB_ATT_IMPACT_NADE",
	"WP_PROTON_NADE":     "MB_ATT_PROTON_NADE",
	"WP_GLOP_NADE":       "MB_ATT_GLOP_NADE",
	"WP_WELDER":          "MB_ATT_WELDER",
	"WP_MIRV":            "MB_ATT_MIRV",
	"WP_CYCLER":          "MB_ATT_CYCLER",
	"WP_FA3_FLECHETTE":   "MB_ATT_FA3_FLECHETTE",
	"WP_MELEE":           "",
	"WP_SABER":           "",
	"WP_NONE":            "",
}

// secondaryWeaponAttributes maps weapons to secondary attributes (blobs, alt-fire nades, etc.).
var secondaryWeaponAttributes = map[string][]string{
	"WP_CLONE_RIFLE": {"MB_ATT_CLONEBLOBS", "MB_ATT_STRONGBLOBS"},
	"WP_M5":          {"MB_ATT_ARC_RIFLE_SCOPE", "MB_ATT_ARC_RIFLE_GRENADELAUNCHER"},
	"WP_REPEATER":    {"MB_ATT_REPEATER_NADES"},
	"WP_FLECHETTE":   {"MB_ATT_FLECHETTE_NADES", "MB_ATT_FLECHETTE_ALT_NUM"},
	"WP_DEMP2":       {"MB_ATT_DEMP2_BLASTS"},
	"WP_UGL":         {"MB_ATT_UGL_BURST", "MB_ATT_UGL_IMPACT", "MB_ATT_UGL_BURST_MIXED"},
	"WP_MGL":         {"MB_ATT_MGL_BURST", "MB_ATT_MGL_IMPACT", "MB_ATT_STICKY_BOMBS"},
	"WP_THROWER":     {"MB_ATT_THROWER_FLAME", "MB_ATT_THROWER_ICE", "MB_ATT_THROWER_LIGHTNING", "MB_ATT_THROWER_PLASMA", "MB_ATT_THROWER_POISON"},
}

// easSkillMap links EAS ability items to their required engine attributes.
var easSkillMap = map[string]string{
	"EAS_HI_GRAPPLEHOOK":      "MB_ATT_GRAPPLE_HOOK",
	"EAS_HI_MEDPAC":           "MB_ATT_BACTA",
	"EAS_HI_MEDPAC_BIG":       "MB_ATT_BACTA_BIG",
	"EAS_HI_SEEKER":           "MB_ATT_DRONE",
	"EAS_HI_BASESEEKER":       "MB_ATT_BASESEEKER",
	"EAS_HI_SHIELD":           "MB_ATT_FORCEFIELD",
	"EAS_HI_PSHIELD":          "MB_ATT_PSHIELD",
	"EAS_HI_CLOAK":            "MB_ATT_CLOAK",
	"EAS_HI_EWEB":             "MB_ATT_EWEB",
	"EAS_HI_SENTRY_GUN":       "MB_ATT_SENTRY",
	"EAS_HI_OVERCLOCK":        "MB_ATT_CYBERNETICS",
	"EAS_HI_TEMP_CLOAK":       "MB_ATT_TEMP_CLOAK",
	"EAS_HI_TEMP_PSHIELD":     "MB_ATT_TEMP_PSHIELD",
	"EAS_HI_TEMP_GRAPPLEHOOK": "MB_ATT_TEMP_GRAP_HOOKS",
}

// CanonicalAttributeFor returns the primary paired MB_ATT_* for a weapon.
func CanonicalAttributeFor(wpID string) string {
	return weaponAttributeLink[wpID]
}

// RelatedAttributesForWeapon returns all companion attributes for a weapon (primary + secondaries).
func RelatedAttributesForWeapon(wpID string) []string {
	var results []string
	if primary := weaponAttributeLink[wpID]; primary != "" {
		results = append(results, primary)
	}
	if secondaries, ok := secondaryWeaponAttributes[wpID]; ok {
		results = append(results, secondaries...)
	}
	return results
}

// RequiredAttributeForEAS returns the attribute required by an EAS action skill.
func RequiredAttributeForEAS(easID string) string {
	return easSkillMap[easID]
}

// weaponFlagKeys mirrors the explicit G_SetHeldFlag keys in bg_saga.c.
// They cannot be derived from weapon names (notably Bryar and FA3).
var weaponFlagKeys = map[string]string{
	"WP_MELEE":           "WP_MeleeFlags",
	"WP_STUN_BATON":      "WP_StunBatonFlags",
	"WP_SABER":           "WP_SaberFlags",
	"WP_BRYAR_PISTOL":    "WP_BlasterPistolFlags",
	"WP_BLASTER_PISTOL":  "WP_BlasterPistolFlags",
	"WP_CLONE_PISTOL":    "WP_ClonePistolFlags",
	"WP_MANDO_PISTOL":    "WP_MandoPistolFlags",
	"WP_BLASTER":         "WP_BlasterFlags",
	"WP_DISRUPTOR":       "WP_DisruptorFlags",
	"WP_BOWCASTER":       "WP_BowcasterFlags",
	"WP_REPEATER":        "WP_RepeaterFlags",
	"WP_CLONE_RIFLE":     "WP_CloneRifleFlags",
	"WP_THROWER":         "WP_ThrowerFlags",
	"WP_MINIGUN":         "WP_MinigunFlags",
	"WP_DEMP2":           "WP_DEMP2Flags",
	"WP_SHOTGUN":         "WP_ShotgunFlags",
	"WP_FLECHETTE":       "WP_FlechetteFlags",
	"WP_A280":            "WP_a280Flags",
	"WP_DLT20A":          "WP_dlt20aFlags",
	"WP_M5":              "WP_m5Flags",
	"WP_T21":             "WP_t21Flags",
	"WP_EE3":             "WP_ee3Flags",
	"WP_EE4":             "WP_ee4Flags",
	"WP_AMBAN":           "WP_AmbanFlags",
	"WP_CR2":             "WP_cr2Flags",
	"WP_DC_CARBINE":      "WP_DCCarbineFlags",
	"WP_E_22":            "WP_E22Flags",
	"WP_HEAVY_PISTOL":    "WP_HeavyPistolFlags",
	"WP_DLT19":           "WP_DLT19Flags",
	"WP_TRAD_BOWCASTER":  "WP_TradBowcasterFlags",
	"WP_ROCKET_LAUNCHER": "WP_RocketLauncherFlags",
	"WP_PLX1":            "WP_plx1Flags",
	"WP_THERMAL":         "WP_ThermalFlags",
	"WP_FRAG_NADE":       "WP_FragNadeFlags",
	"WP_TRIP_MINE":       "WP_TripMineFlags",
	"WP_PULSE_NADE":      "WP_PulseNadeFlags",
	"WP_DET_PACK":        "WP_DetPackFlags",
	"WP_CONCUSSION":      "WP_ConcussionFlags",
	"WP_SBD":             "WP_sbdFlags",
	"WP_WELD_PULSE":      "WP_WeldPulseFlags",
	"WP_WELD_BEAM":       "WP_WeldBeamFlags",
	"WP_BRYAR_OLD":       "WP_BryarOldFlags",
	"WP_REAL_TD":         "WP_RealTDFlags",
	"WP_FIRE_NADE":       "WP_FireNadeFlags",
	"WP_SONIC_NADE":      "WP_SonicNadeFlags",
	"WP_CRYO_NADE":       "WP_CryoNadeFlags",
	"WP_CONC_NADE":       "WP_ConcNadeFlags",
	"WP_PROJ":            "WP_projFlags",
	"WP_BACTA_BOMB":      "WP_BactaBombFlags",
	"WP_SMOKE_NADE":      "WP_SmokeNadeFlags",
	"WP_GAS_NADE":        "WP_GasNadeFlags",
	"WP_ROCK_NADE":       "WP_RockNadeFlags",
	"WP_FLASH_NADE":      "WP_FlashNadeFlags",
	"WP_IMPACT_NADE":     "WP_ImpactNadeFlags",
	"WP_PROTON_NADE":     "WP_ProtonNadeFlags",
	"WP_GLOP_NADE":       "WP_GlopNadeFlags",
	"WP_UGL":             "WP_UGLFlags",
	"WP_MGL":             "WP_MGLFlags",
	"WP_EQUALIZER":       "WP_EqualizerFlags",
	"WP_WELDER":          "WP_WelderFlags",
	"WP_MIRV":            "WP_MIRVFlags",
	"WP_CYCLER":          "WP_cyclerFlags",
	"WP_FA3_FLECHETTE":   "WP_fa3Flags",
}

// WeaponFlagKeyForWeapon returns the parser's exact key, or empty if unsupported.
func WeaponFlagKeyForWeapon(wpID string) string {
	return weaponFlagKeys[wpID]
}
