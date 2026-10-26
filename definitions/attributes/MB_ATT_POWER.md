# Power (Resource Pool)

`MB_ATT_POWER`

> Sets the maximum size of the class's resource pool.

## What it does

Generic "max resource" knob — controls the cap on the class's primary resource (Force, Energy, Battery, Stamina, Fuel, Rage). Used in tandem with `MB_ATT_REGEN` for a power/regen pair model.

## Per level

- **Level 1** — small pool.
- **Level 2** — medium pool.
- **Level 3** — large pool.

## Notes

- Resource type is determined by the class header (`RESOURCE_*`), not by this attribute.
- Pairs with `MB_ATT_REGEN` for full pool tuning.
- Stacks with `MB_ATT_STM_MULTIPLIER` on Stamina-based classes.

---

`resource` · `pool` · `multipliers`

<!-- icon-suggestion: power -->
