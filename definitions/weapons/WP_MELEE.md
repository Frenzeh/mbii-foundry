# Melee

`WP_MELEE`

> Hand-to-hand combat. Punches and kicks; the Wookiee/SBD primary attack.

## What it does

`WP_MELEE` is the bare-fists weapon — punches as primary, kicks as secondary if the class has the kick attribute. For most classes it's a fallback. For Wookiees with `MB_ATT_WOOKIE_STRENGTH` and SBDs in close quarters, it becomes a primary lethal weapon.

## Primary fire

- **Mode** — Punch
- **Fire rate** — 400ms

## Secondary fire

- **Mode** — Kick (if class allows; otherwise punch)
- **Fire rate** — 400ms

## Notes

- Pairs with `MB_ATT_CCTRAINING` (close-combat training upgrades).
- Wookiees deal lethal damage with bare fists when `MB_ATT_WOOKIE_STRENGTH` is bought.
- SBDs use this as a primary in close quarters.

---

`melee` · `unarmed` · `cqc`

<!-- icon-suggestion: w_icon_melee -->
