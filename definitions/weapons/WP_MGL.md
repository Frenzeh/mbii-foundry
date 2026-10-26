# Micro Grenade Launcher

`WP_MGL`

> Wrist-mounted sticky-bomb launcher. Fires sticky charges that detonate on trigger or impact.

## What it does

The MGL launches small sticky bombs (`CHGE_STICKY`) that adhere to surfaces and players. Both primary and secondary cycle at 500ms; the difference is detonation timing — primary fires the projectile, secondary or a follow-up trigger detonates all stuck rounds. Mode swap unlocked at level 0 (always available). FRENZY_STICKY tagged feature.

## Primary fire

- **Mode** — Fire sticky bomb
- **Ammo cost** — 3
- **Fire rate** — 500ms

## Secondary fire

- **Mode** — Detonate / fire (depending on swap state)
- **Fire rate** — 500ms

## Notes

- Pairs with `MB_ATT_MGL`. See also `MB_ATT_MGL_IMPACT` and `MB_ATT_MGL_BURST` variants.
- Uses sticky-bomb ammo pool (`AMMO_STICKY_BOMBS`, capacity 15).
- MGL impact and launched-pulse fixes shipped on R22 (see commit `buildTest/frenzy/r22-mgl-impact-ugl-pulse`).

---

`launcher` · `sticky` · `frenzy-gl`

<!-- icon-suggestion: w_icon_upl -->
