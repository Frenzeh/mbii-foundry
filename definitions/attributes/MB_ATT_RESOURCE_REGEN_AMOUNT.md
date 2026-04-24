# Resource Regen Amount

`MB_ATT_RESOURCE_REGEN_AMOUNT`

> Per-tick amount for the class's resource (Battery, Energy, Stamina, Fuel, Rage).

## What it does

Sets how much of the active class resource (`RESOURCE_BATTERY` for SBD, `RESOURCE_ENERGY` for Hero, `RESOURCE_STAMINA` for ARC, etc.) is restored each tick. The exact resource is determined by the class definition — this attribute scales the regen on whichever pool the class uses.

## Per level

- **Level 1** — small amount per tick.
- **Level 2** — medium amount per tick.
- **Level 3** — large amount per tick.

## Notes

- Pairs with `MB_ATT_RESOURCE_REGEN_RATE` and `MB_ATT_RESOURCE_REGEN_CAP`.
- Single attribute covers all class resources — class header determines which pool fills.
- Rage is the exception — it builds from damage dealt, not from regen.

---

`regen` · `resource` · `passive`

<!-- icon-suggestion: regen-resource -->
