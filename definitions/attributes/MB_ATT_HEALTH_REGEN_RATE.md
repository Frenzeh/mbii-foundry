# Health Regen Rate

`MB_ATT_HEALTH_REGEN_RATE`

> Tick interval for passive HP regen, in milliseconds.

## What it does

Selects the tick interval from the class's `rankHealthRegenRate` table. Lower number means faster ticks; higher means slower. Combined with `MB_ATT_HEALTH_REGEN_AMOUNT` to control the effective HPS.

## Per level

- **Level 1** — slow ticks (e.g. 2000 ms).
- **Level 2** — medium ticks (e.g. 1000 ms).
- **Level 3** — fast ticks (e.g. 500 ms).

## Notes

- A rate of 0 disables regen even if amount is set.
- Effective HPS = `amount / (rate / 1000)`.
- Sibling of `MB_ATT_HEALTH_REGEN_AMOUNT` and `MB_ATT_HEALTH_REGEN_CAP` — all three travel together.

---

`regen` · `health` · `passive`

<!-- icon-suggestion: regen-rate -->
