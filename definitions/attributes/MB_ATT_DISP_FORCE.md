# Force/Resource Dispenser

`MB_ATT_DISP_FORCE`

> Hold-to-restore-resource allies — passive class resource transfer.

## What it does

Hold the Use key while looking at an ally to restore their class resource (Force, Energy, Stamina, Battery, Fuel) every 250 ms. Pool affected depends on the recipient's class.

## Per level

- **Level 1** — small resource per tick.
- **Level 2** — medium resource per tick.
- **Level 3** — large resource per tick.

## Notes

- Tick interval is fixed at 250 ms.
- Sibling of `MB_ATT_DROP_FORCE` and `MB_ATT_STIM_FORCE`.
- Restores whichever resource the recipient class uses — single attribute covers all pools.

---

`supply` · `dispenser` · `resource` · `support`

<!-- icon-suggestion: dispenser-force -->
