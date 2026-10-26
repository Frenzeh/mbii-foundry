# Run-Fast Melee

`CFL_RUNFASTMELEE`

> +15% movement speed while `WP_MELEE` is held.

## What it does

`g_active.c:2882` — when computing class speed in FA mode, multiplies by 1.15 if the flag is set and the held weapon is `WP_MELEE`. Intended for characters whose identity is "fast fists" without a full CCTRAINING tree.

## Notes

- Gated behind `#ifdef NEW_CLASSFLAGS`; needs the R21+ build.
- Stacks with `MB_ATT_CCTRAINING` rank bonuses, so mixing both can push melee speed very high.
- Largely superseded by `MB_ATT_CCTRAINING` and weapon-level `HELD_SPEED` flags in modern specs, but still valid for simple configs.

---

`melee` · `mobility` · `modifier`
