# Universal Grenade Launcher

`MB_ATT_UGL`

> Under-barrel grenade launcher attribute. Mode-swap weapon family.

## What it does

Grants `WP_UGL` — an under-barrel launcher that fires `CHGE_GRENLAUNCH` grenades. Refire 1250ms (level 1) → 500ms (level 4). Mode-cycle weapon; swap unlocks at level 3. Variants: `MB_ATT_UGL_BURST`, `MB_ATT_UGL_IMPACT`, `MB_ATT_UGL_BURST_MIXED`.

## Per level

- **Level 1** — Standard contact/timed grenade; 1250ms refire.
- **Level 2** — Faster refire (1000ms).
- **Level 3** — Mode-swap unlocked; 750ms refire.
- **Level 4** — 500ms refire, fastest cadence.

## Notes

- Unlocks `WP_UGL`.
- Variants: `MB_ATT_UGL_BURST` (burst-fire), `MB_ATT_UGL_IMPACT` (impact-fuse), `MB_ATT_UGL_BURST_MIXED` (mixed pattern).
- FRENZY_GL feature line.
- See R22 fix branch `buildTest/frenzy/r22-mgl-impact-ugl-pulse` for owner-guard fix on launched-pulse.

---

`launcher` · `grenade` · `frenzy-gl`

<!-- icon-suggestion: icon_stats_ugl -->
