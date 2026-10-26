# Heavy Melee

`CFL_HEAVYMELEE`

> Forces the class's melee hits to count as heavy-weapon damage (armor-piercing, higher knockback).

## What it does

In Siege mode, `G_MeleeHeavyMelee()` returns true for flagged classes, routing melee/kick/kata damage through the heavy-weapon damage pipeline. This bypasses some light-armor mitigation, produces stronger knockback, and is treated like a "heavy weapon" hit for knockdown/dismember checks.

## Notes

- Ground truth: `g_combat.c:294` — `GT_SIEGE && MB_FA_MODE && classflag & (1 << CFL_HEAVYMELEE)` returns true for `IsHeavyWeapon(MOD_MELEE|MELEE_KICK|MELEE_KATA)`.
- Stacks with `MB_ATT_WOOKIE_STRENGTH` — Wookiee Strength 3 on a Heavy Melee class is the game's hardest-hitting melee trade.
- Does not directly double damage — it reclassifies the damage type, and the resulting number depends on target armor / flags / MODs applied downstream.

---

`melee` · `offense` · `armor-piercing`
