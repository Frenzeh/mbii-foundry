# Dash

`MB_ATT_DASH`

> Quick, evasive burst of movement.

## What it does

Engine-level configuration for the Dash special. Determines the cooldown of each dash. Bound to `EAS_HI_DASH`.

## Per level

- **Level 1** — 4-second cooldown (calculated as `5000 - level * 1000` in `g_items.c:4037`).
- **Level 2** — 3-second cooldown (calculated as `5000 - level * 1000` in `g_items.c:4037`).

## Notes

- Pairs with `MB_ATT_DASH_JUMP` for jump-cancel mid-dash.
- *Note:* i-frames at Level 2 were removed (commented out in `w_force.c:8465`). Level 2 only provides a faster cooldown.

---

`movement` · `dash` · `mobility`

<!-- icon-suggestion: dash -->
