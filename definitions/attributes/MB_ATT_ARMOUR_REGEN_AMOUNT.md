# Armor Regen Amount

`MB_ATT_ARMOUR_REGEN_AMOUNT`

> Per-tick armor restored by passive armor regen.

## What it does

Sets how much armor is added each regen tick, indexed into the class's `rankArmorRegenAmount` table. Useful for shielded units (Droids, SBDs) where armor functions like a regenerating overshield.

## Per level

- **Level 1** — small amount per tick.
- **Level 2** — medium amount per tick.
- **Level 3** — large amount per tick.

## Notes

- Pairs with `MB_ATT_ARMOUR_REGEN_RATE` and `MB_ATT_ARMOUR_REGEN_CAP`.
- Halo-style "shield recharge" effect: pause damage briefly and shields refill.
- Doesn't trigger if base `MB_ATT_ARMOUR` is 0 — needs an armor pool to fill.

---

`regen` · `armor` · `passive`

<!-- icon-suggestion: regen-armor -->
