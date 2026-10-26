# Thermal Detonator

`WP_THERMAL`

> Standard thermal detonator. 3-second fuse, hard splash damage, signature sci-fi grenade.

## What it does

The basic Class-A thermal detonator: timed fuse, throw with `CHGE_GRENADE` cook to shorten arc. Lethal in its blast radius, no special area-of-denial effect — pure single-blast damage. Common across Imperial and Mercenary loadouts.

## Primary fire

- **Mode** — Throw (`CHGE_GRENADE`)
- **Fire rate** — 1000ms
- **Timer** — ~3 seconds

## Secondary fire

- **Mode** — Lobbed throw
- **Fire rate** — 1000ms

## Notes

- Pairs with `MB_ATT_BASE_TD` (in `weaponData[]`) and `MB_ATT_THERMALS`.
- Cook the fuse by holding fire to shorten detonation time.
- Distinct from `WP_REAL_TD` (the canon-accurate nuke variant).

---

`grenade` · `thermal` · `base-td`

<!-- icon-suggestion: w_icon_thermal -->
