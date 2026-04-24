# Dash

`EAS_HI_DASH`

> Triggers the Dash burst-movement special.

## What it does

Performs a short, fast directional burst keyed to the player's current movement input. Eight-way dashing is supported (WASD, with alt-fire for backward variant). Uses Stamina/Energy on activation rather than Force.

## Notes

- Bind to `special1`/`special2` so it shares a cooldown with other movement tools.
- Effect, cooldown, and i-frames are configured by `MB_ATT_DASH` (the attribute), not by the binding itself.
- Cost is rolled into the Dash attribute table — the EAS entry just dispatches the action.

---

`movement` · `special` · `acrobatics`

<!-- icon-suggestion: dash -->
