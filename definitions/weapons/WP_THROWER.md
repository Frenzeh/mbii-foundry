# Flame/Element Thrower (CR-24)

`WP_THROWER`

> CR-24 flame rifle. Continuous-stream weapon; element variants per attribute.

## What it does

The Thrower is a sustained-stream weapon that fires at 50ms cadence in both modes — effectively a continuous beam. Multiple element variants exist via `MB_ATT_THROWER_*` (lightning, ice, poison, plasma, flame), each changing the damage type and visual effect. Drains battery while firing.

## Primary fire

- **Mode** — Continuous stream
- **Ammo cost** — 1 per tick
- **Fire rate** — 50ms (effectively continuous)

## Secondary fire

- **Mode** — Same continuous stream (alt visual/effect)
- **Ammo cost** — 1 per tick
- **Fire rate** — 50ms

## Notes

- Pairs with `MB_ATT_THROWER` (and elemental variants `MB_ATT_THROWER_LIGHTNING`, `_ICE`, `_POISON`, `_PLASMA`, `_FLAME`).
- Uses `THROWER_ANIM_SET`.
- See also `MB_ATT_FLAMETHROWER` and `MB_ATT_WPFLAMETHROWER` for the broader flame family.
- Heavy Mercenary / Sith Cultist signature.

---

`thrower` · `stream` · `elemental`

<!-- icon-suggestion: w_icon_cr-24_flamerifle -->
