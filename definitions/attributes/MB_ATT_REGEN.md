# Regen

`MB_ATT_REGEN`

> Sets the regen rate of the class's resource pool.

## What it does

Generic "resource regen rate" attribute. Controls how fast the class resource (Force, Energy, Battery, etc.) refills. Pairs with `MB_ATT_POWER` for max-pool sizing.

## Per level

- **Level 1** — slow regen.
- **Level 2** — medium regen.
- **Level 3** — fast regen.

## Notes

- Pairs with `MB_ATT_POWER` (pool max).
- For granular control use the `MB_ATT_RESOURCE_REGEN_*` family (amount, rate, cap).
- Resource pool affected depends on class (Force, Energy, Battery, Stamina, Fuel, Rage).

---

`resource` · `regen` · `multipliers`

<!-- icon-suggestion: regen -->
