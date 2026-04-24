# Micro Grenade Launcher

`MB_ATT_MGL`

> Wrist-mounted sticky-bomb launcher attribute. Fires sticky charges that detonate on trigger.

## What it does

Grants `WP_MGL` — the micro grenade launcher firing sticky bombs (`CHGE_STICKY`) that adhere to surfaces and players. Both modes cycle at 500ms. Mode-swap unlocked at level 0 (always available). Uses the sticky-bomb ammo pool (`AMMO_STICKY_BOMBS`, capacity 15).

## Per level

- **Level 1** — Basic sticky-bomb launcher; 500ms cycle.
- **Level 2** — Improved stick adhesion and detonation timing.
- **Level 3** — Maximum damage; faster cycle.

## Notes

- Unlocks `WP_MGL`.
- FRENZY_STICKY feature line.
- Variants: `MB_ATT_MGL_IMPACT`, `MB_ATT_MGL_BURST`.
- See R22 fix branch `buildTest/frenzy/r22-mgl-impact-ugl-pulse` for the owner-guard fix on touch_stickyBombExplode.

---

`launcher` · `sticky` · `frenzy-sticky`

<!-- icon-suggestion: icon_stats_mgl -->
