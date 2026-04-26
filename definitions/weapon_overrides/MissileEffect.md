# MissileEffect

`MissileEffect effects/foo/projectile.efx`

> Effect attached to in-flight projectiles.

## What it does

Replaces the trail / aura effect on the weapon's projectile while it's in the air. Often paired with `missileModel` to fully reskin the projectile.

## Valid values

Path to an `.efx` effect file.

## Notes

- `AltMissileEffect` covers alt-fire projectiles. `Missile3Effect` is used by weapons that fire a third projectile variant.
- For hitscan weapons, see `primHitscanShot` / `primHitscanTracer` instead.

---

`mbch` · `weapon` · `override` · `effect`
