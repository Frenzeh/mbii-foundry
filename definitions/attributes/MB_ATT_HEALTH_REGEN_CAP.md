# Health Regen Cap

`MB_ATT_HEALTH_REGEN_CAP`

> Ceiling above which passive health regen stops applying.

## What it does

Selects the cap from `rankHealthRegenCap`. Once HP reaches this value, regen pauses; regen only resumes after damage drops HP below the cap. If unset (0), the cap defaults to `STAT_MAX_HEALTH`.

## Per level

- **Level 1** — low cap (e.g. only regen up to 50 HP).
- **Level 2** — medium cap.
- **Level 3** — full max-health cap.

## Notes

- Set to 0 to use full max health.
- Useful for "first aid only" classes — you can self-stabilize but not fully heal.
- Sibling of `MB_ATT_HEALTH_REGEN_AMOUNT` and `MB_ATT_HEALTH_REGEN_RATE`.

---

`regen` · `health` · `passive` · `cap`

<!-- icon-suggestion: regen-cap -->
