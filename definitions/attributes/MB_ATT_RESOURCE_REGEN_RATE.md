# Resource Regen Rate

`MB_ATT_RESOURCE_REGEN_RATE`

> Tick interval for the class's resource regen, in milliseconds.

## What it does

Selects the tick interval for resource regen. Used together with `MB_ATT_RESOURCE_REGEN_AMOUNT` to set the resource-per-second value.

## Per level

- **Level 1** — slow ticks.
- **Level 2** — medium ticks.
- **Level 3** — fast ticks.

## Notes

- Set 0 to disable resource regen.
- Affects whichever resource the class uses (Battery, Energy, Stamina, Fuel — see class definition).
- Sibling of `MB_ATT_RESOURCE_REGEN_AMOUNT` and `MB_ATT_RESOURCE_REGEN_CAP`.

---

`regen` · `resource` · `passive`

<!-- icon-suggestion: regen-rate -->
