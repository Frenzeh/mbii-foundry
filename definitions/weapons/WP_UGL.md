# Universal Grenade Launcher

`WP_UGL`

> Under-barrel grenade launcher. Mode-swap weapon family with multiple grenade types.

## What it does

The UGL is an under-barrel launcher that fires `CHGE_GRENLAUNCH` grenades — base behavior is a contact/timed grenade with a 1250ms refire (500ms at level 4). Mode-cycle (`swapMode`, swap unlocked at level 3) flips primary/secondary. The UGL family branches via `MB_ATT_UGL_BURST`, `_IMPACT`, and `_BURST_MIXED` for different grenade behaviors.

## Primary fire

- **Mode** — Grenade launch (`CHGE_GRENLAUNCH`)
- **Fire rate** — 1250ms (level 1) → 500ms (level 4)

## Secondary fire

- **Mode** — Alternate launch profile (mode-swap)
- **Fire rate** — 1250ms → 500ms

## Notes

- Pairs with `MB_ATT_UGL`. Variants: `MB_ATT_UGL_BURST`, `MB_ATT_UGL_IMPACT`, `MB_ATT_UGL_BURST_MIXED`.
- FRENZY_GL feature line — see commit `buildTest/frenzy/r22-mgl-impact-ugl-pulse` for recent fixes.
- Owner-guard fix on launched-pulse on R22 (was firing self-explode immediately).

---

`launcher` · `grenade` · `frenzy-gl`

<!-- icon-suggestion: w_icon_relby_v10 -->
