# Health Drop

`MB_ATT_DROP_HEALTH`

> Drops a one-shot health pickup on the ground.

## What it does

Spawns a Health pickup item at the user's feet that any ally can run over to consume. Unlike the dispenser (channeled), this is a fire-and-forget — drop it during a fight and rotate.

## Per level

- **Level 1** — small heal per pickup.
- **Level 2** — medium heal per pickup.
- **Level 3** — large heal per pickup.

## Notes

- Pickup is single-use; first ally over it consumes it.
- Sibling of `MB_ATT_DISP_HEALTH` (channeled) and `MB_ATT_STIM_HEALTH` (single-target instant).
- Charge count and total drops scale with `MB_ATT_SUPPLYDROP`.

---

`supply` · `drop` · `heal` · `pickup`

<!-- icon-suggestion: drop-health -->
