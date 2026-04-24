# Melee Resistance

`CFL_STRONGAGAINSTPHYSICAL`

> Halves damage taken from physical attacks (punches, kicks, melee kata).

## What it does

In Siege/FA mode, any `MOD_MELEE`, `MOD_MELEE_KICK`, or `MOD_MELEE_KATA` hit against a class carrying this flag is multiplied by 0.5. Does not protect against saber damage, blaster fire, or explosives — it's a pure anti-brawler bit.

## Notes

- Ground truth: `g_combat.c` `damage *= 0.5` on the three melee MODs when the flag is set.
- Pairs well with heavy-armor classes who still don't want Wookiee Strength to chunk them.
- Counters `MB_ATT_WOOKIE_STRENGTH` effectively at Ranks 1-2; Strength 3's raw damage still hurts.

---

`defense` · `melee` · `mitigation`
