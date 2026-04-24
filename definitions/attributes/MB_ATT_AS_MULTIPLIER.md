# Attack Speed Multiplier

`MB_ATT_AS_MULTIPLIER`

> Multiplier for saber animation speed.

## What it does

Scales how fast saber swing animations play. Effectively the engine equivalent of `animSpeedScale` on a per-saber basis, but applied to the player rather than the weapon. Default 1.0.

## Notes

- Faster animations = faster commitment recovery, but can desync visual reads for opponents.
- Stacks with the saber-file `animSpeedScale` and with `MB_ATT_SS_*` style ranks.
- Pairs with `MB_ATT_CS_MULTIPLIER` (chain speed) to fully tune saber tempo.

---

`multiplier` · `saber` · `attack-speed`

<!-- icon-suggestion: attack-speed -->
