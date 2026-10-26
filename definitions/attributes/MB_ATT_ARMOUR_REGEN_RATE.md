# Armor Regen Rate

`MB_ATT_ARMOUR_REGEN_RATE`

> Tick interval for passive armor regen, in milliseconds.

## What it does

Selects the tick interval from the class's `rankArmorRegenRate` table. Combined with the armor regen amount to set the effective armor-per-second.

## Per level

- **Level 1** — slow ticks.
- **Level 2** — medium ticks.
- **Level 3** — fast ticks.

## Notes

- Set 0 to disable.
- Sibling of `MB_ATT_ARMOUR_REGEN_AMOUNT` and `MB_ATT_ARMOUR_REGEN_CAP`.

---

`regen` · `armor` · `passive`

<!-- icon-suggestion: regen-rate -->
