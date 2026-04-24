# MGL Impact

`MB_ATT_MGL_IMPACT`

> Impact-fuse variant of the MGL. Sticky bombs detonate on contact instead of remote trigger.

## What it does

Modifies `WP_MGL` so sticky bombs detonate on contact rather than waiting for a remote trigger. Faster damage application but loses the place-and-trap option of standard MGL. Best when you want guaranteed-on-target detonations against fast-moving targets.

## Per level

- **Level 1** — Contact detonation enabled.
- **Level 2** — Larger blast radius on impact.
- **Level 3** — Maximum damage; tightest impact profile.

## Notes

- Requires `MB_ATT_MGL` for the base launcher.
- Variants: `MB_ATT_MGL_BURST`.
- R22 fix: missing owner guard on `touch_stickyBombExplode` was causing immediate self-explode — fixed in `buildTest/frenzy/r22-mgl-impact-ugl-pulse`.

---

`launcher` · `impact` · `frenzy-sticky`

<!-- icon-suggestion: icon_stats_mgl -->
