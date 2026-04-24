# Health Regen Amount

`MB_ATT_HEALTH_REGEN_AMOUNT`

> Per-tick HP restored by passive health regeneration.

## What it does

Sets how much health is added each regen tick. Pulls from the class's `rankHealthRegenAmount` table — the rank index here picks one of the configured amounts (e.g. rank 1 might add 1 HP/tick, rank 3 might add 5 HP/tick).

## Per level

- **Level 1** — small amount per tick (configured in MBCH `rankHealthRegenAmount[1]`).
- **Level 2** — medium amount per tick.
- **Level 3** — large amount per tick.

## Notes

- Pairs with `MB_ATT_HEALTH_REGEN_RATE` (tick interval) and `MB_ATT_HEALTH_REGEN_CAP` (ceiling).
- All three regen attributes must be set for passive heal to work — amount alone with rate=0 does nothing.
- Stops applying once HP reaches `MB_ATT_HEALTH_REGEN_CAP`.

---

`regen` · `health` · `passive`

<!-- icon-suggestion: regen-health -->
