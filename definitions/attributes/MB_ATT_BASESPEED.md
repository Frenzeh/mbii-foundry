# Base Speed

`MB_ATT_BASESPEED`

> Sets the user's base movement speed value.

## What it does

Overrides the class's default movement speed. Raw engine units — 250 is roughly Hero speed, 200 is heavy trooper, 300+ is dash-flavor.

## Per level

- **Level 1** — slow class baseline.
- **Level 2** — standard baseline.
- **Level 3** — fast baseline.

## Notes

- Engine units; Player default is ~250.
- Sibling of MBCH `baseSpeed` field — class-level setting vs attribute pick.
- `MB_ATT_DEXTERITY` adds to top speed; this sets the floor.

---

`movement` · `speed` · `general`

<!-- icon-suggestion: basespeed -->
