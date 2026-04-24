# Shield Recharge

`MB_ATT_SHIELD_RECHARGE`

> Armor regen with the same caps as Health regen.

## What it does

Older armor-regen attribute that uses the Health regen ceiling (i.e. won't exceed `MB_ATT_HEALTH_REGEN_CAP`). Used on classes whose armor is meant to recharge alongside HP rather than independently.

## Per level

- **Level 1** — slow armor recharge.
- **Level 2** — medium recharge.
- **Level 3** — fast recharge.

## Notes

- Sibling of `MB_ATT_SHIELD_RECHARGE2` (uncapped variant).
- Uses Health regen bounds; for independent armor regen prefer `MB_ATT_ARMOUR_REGEN_*` family.

---

`regen` · `armor` · `legacy`

<!-- icon-suggestion: shield-recharge -->
