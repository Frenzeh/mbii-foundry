# Class-Specific Weapon Overrides

`WeaponInfo0 { ... }`, `WeaponInfo1 { ... }`, etc.

> Inside a `ClassInfo` block, an MBCH can declare per-class weapon overrides
> that swap models, icons, sounds, effects, ammo, animations, and damage on
> top of any base WP_*. The block name suffix is just an index — the actual
> target is set by `WeaponToReplace`.

## Block syntax

```
WeaponInfo0
{
  WeaponToReplace WP_REPEATER
  WeaponBasedOff  WP_REPEATER
  WeaponName      "Snowball Launcher"
  Icon            gfx/hud/w_icon_snowball
  NewWorldModel   models/weapons2/snowball/snowball_w.glm
  NewViewModel    models/weapons2/snowball/snowball.md3
  MuzzleEffect    effects/wookiee/snowball_muzzle.efx
  FlashSound      sound/weapons/snowball/fire1.wav
  ReloadTimeModifier 0.75
  DamageMod       0.6
  RateMod         1.2
  VelocityMod     0.85
}
```

Up to 16 override blocks per class (engine cap). Each block must include
`WeaponToReplace`; everything else is optional.

## Field families

| Family | Fields |
|--------|--------|
| Identity | `WeaponToReplace`, `WeaponBasedOff`, `WeaponName`, `Icon` |
| Visuals | `NewWorldModel`, `NewViewModel`, `NewHandsModel`, `NewBarrelModel`, `CorrectedWorldModel`, `missileModel`, `altMissileModel` |
| Effects | `MuzzleEffect`, `AltMuzzleEffect`, `MissileEffect`, `AltMissileEffect`, `Missile3Effect`, `ChargeEffect`, `OverchargeEffect` |
| Hitscan | `primHitscanShot`, `altHitscanShot`, `primHitscanTracer`, `altHitscanTracer`, `primGore`, `altGore`, `disruptorBeam1`, `disruptorBeam2` |
| Sounds | `FlashSound0..3`, `AltFlashSound0..3`, `ChargeSound`, `SelectSound` |
| Ammo / Reload | `customAmmo`, `clipSize`, `ReloadTimeModifier`, `primFireEnabled`, `altFireEnabled` |
| Damage / Rate | `DamageMod`, `RateMod`, `VelocityMod` |
| Force | `FPMult`, `FPChargeMult`, `FPBlockMinMult`, `FPBlockMaxMult`, `FPNoBlockMinMult`, `FPNoBlockMaxMult` |
| Animation | `hasAnimOverrides`, `animReady`, `animAttack`, `animFire`, `animReload`, `animPutAway`, `animPullOut` |

## Notes

- Fields not listed in an override block fall through to the base weapon
  (the engine-defined `WP_*` defaults).
- `WeaponBasedOff` controls inheritance for stats not explicitly overridden;
  if omitted, defaults to `WeaponToReplace`.
- Per-block size budget: keep each `WeaponInfoN { }` under ~4096 bytes.
- The MBCH file as a whole is capped at 16384 bytes (R22.0.00).

---

`mbch` · `weapon` · `override` · `reference`
