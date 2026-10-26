# Wrist Flamethrower

`EAS_HI_FLAME`

> Fires the wrist flamethrower without switching weapons.

## What it does

Dispatches `Skill_ShootThrower` with the flame element (1). Spits a short cone of fire in front of the player, igniting targets briefly. Mandalorians and similar wrist-armed classes use this to layer a DoT over their primary weapon's burst.

## Notes

- Cost and cooldown live in the EAS table (`w_force.c`) — base 2000 ms cooldown.
- Mandalorian gauntlet toggle (`SMBF_GAUNTLET_MODE`) swaps this binding for `EAS_HI_WRIST` (laser) on the same key.
- Damage and ignition duration come from the underlying `MB_ATT_FLAMETHROWER` / `MB_ATT_THROWER_FLAME` attribute, not this special.

---

`special` · `mandalorian` · `wrist` · `fire`

<!-- icon-suggestion: flamethrower -->
