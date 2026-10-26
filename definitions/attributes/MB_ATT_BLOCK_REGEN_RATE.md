# Block Regen Rate

`MB_ATT_BLOCK_REGEN_RATE`

> Tick interval for BP regen, in milliseconds.

## What it does

Selects the tick interval for Block Point regen from the class's BP regen rate table. Lower = faster recovery between trades.

## Per level

- **Level 1** — slow ticks.
- **Level 2** — medium ticks.
- **Level 3** — fast ticks.

## Notes

- Set 0 to disable BP regen entirely (unforgiving duelist mode).
- Sibling of `MB_ATT_BLOCK_REGEN_AMOUNT` and `MB_ATT_BLOCK_REGEN_CAP`.

---

`regen` · `block` · `saber` · `bp`

<!-- icon-suggestion: regen-rate -->
