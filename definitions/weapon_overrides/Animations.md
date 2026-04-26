# Animation Overrides

`hasAnimOverrides 1`
`animReady    BOTH_STAND2`
`animAttack   BOTH_ATTACK2`
`animFire     BOTH_ATTACK_FIRE`
`animReload   BOTH_RELOAD1`
`animPutAway  BOTH_STAND_TO_KNEEL`
`animPullOut  BOTH_KNEEL_TO_STAND`

> Per-block animation override set.

## What it does

Lets the override block use animation sequences different from the base weapon. `hasAnimOverrides 1` flag turns the override system on; the individual `anim*` fields then map weapon states to anim names.

## Valid values

Animation names matching the engine's `BOTH_*` enum (defined in `animations.cfg`).

## Notes

- Set `hasAnimOverrides 0` (or omit) to inherit from `WeaponBasedOff`.
- Mismatched anim names silently fall back to the engine default — verify by walking the class in-game.

---

`mbch` · `weapon` · `override` · `animation`
