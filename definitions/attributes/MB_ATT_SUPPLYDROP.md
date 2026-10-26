# Supply Drop Charges

`MB_ATT_SUPPLYDROP`

> Pool of supply-drop uses for the dispenser/drop/stim family.

## What it does

Master charge counter for the dispenser-rework supply system. Activates on the Use key (legacy "ammo drop" behavior). Each rank grows the total drops the user can place per round.

## Per level

- **Level 1** — small pool of drops.
- **Level 2** — medium pool.
- **Level 3** — large pool.

## Notes

- Does NOT define what is dropped — that's the `MB_ATT_DROP_*` and `MB_ATT_DISP_*` attributes.
- Acts as the budget for all drop/stim/dispenser actions combined.
- Use key triggers a context-aware drop based on what the class has configured.

---

`supply` · `pool` · `drops`

<!-- icon-suggestion: supplydrop -->
