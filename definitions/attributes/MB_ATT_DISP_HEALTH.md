# Health Dispenser

`MB_ATT_DISP_HEALTH`

> Hold-to-heal allies — passive HP transfer.

## What it does

Hold the Use key while looking at an ally to restore their Health every 250 ms. Standard team-medic tool — keeps a teammate topped up while in cover or after a fight.

## Per level

- **Level 1** — small heal per tick.
- **Level 2** — medium heal per tick.
- **Level 3** — large heal per tick.

## Notes

- Tick interval is fixed at 250 ms; only the per-tick amount scales.
- Locks the dispenser-user in the Use animation; cannot fire or move freely while channeling.
- Sibling of `MB_ATT_DROP_HEALTH` (drop a one-shot pickup) and `MB_ATT_STIM_HEALTH` (single-target instant).

---

`supply` · `dispenser` · `heal` · `support`

<!-- icon-suggestion: dispenser-health -->
