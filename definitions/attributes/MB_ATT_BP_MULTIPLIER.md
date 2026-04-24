# Block Points Multiplier

`MB_ATT_BP_MULTIPLIER`

> Multiplier for the maximum Block Point pool.

## What it does

Scales the user's maximum BP — the resource consumed by saber blocks and gunfire deflects. Higher values = more sustained defense before guard-break. Default 1.0.

## Notes

- Stacks with `MB_ATT_FP_SABER_DEFENSE` rank-based BP pool.
- Doesn't change BP regen rate — pair with `MB_ATT_BLOCK_REGEN_*` siblings if you want both.
- Set to 0 for "no block" classes (gunners with sabers as decoration).

---

`multiplier` · `block` · `bp` · `saber`

<!-- icon-suggestion: bp-multiplier -->
