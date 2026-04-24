# SBD Arm Blaster

`WP_SBD`

> Wrist-mounted blaster for Super Battle Droids. Sustained fire on battery.

## What it does

The SBD Arm Blaster is the integrated wrist-cannon every Super Battle Droid carries. Primary fires a fast sustained stream that drains the SBD's battery rather than ammo. Secondary uses `CHGE_SBD` for a heavier charged round at level 2+.

## Primary fire

- **Damage** — 34
- **Velocity** — 4600 units/sec
- **Ammo cost** — 1 (battery)
- **Fire rate** — 270ms (level 1) → 135ms (level 4)

## Secondary fire

- **Mode** — Charged shot (`CHGE_SBD`)
- **Ammo cost** — 50
- **Fire rate** — 800ms

## Notes

- Pairs with `MB_ATT_FIREPOWER` (SBD-specific).
- Drains battery instead of standard ammo — managed by `MB_RES_BATTERY`.
- Mode-cycle weapon (`swapMode`); SBD-only animation set.
- See `MB_ATT_SBD_*` attributes for paired upgrades (battery, wristrocket, dualblasters).

---

`sbd` · `wrist-mounted` · `battery`

<!-- icon-suggestion: w_icon_sbdarm -->
