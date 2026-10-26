# Respawn Count

`MB_ATT_RESPAWNS`

> Number of in-round respawns the user gets.

## What it does

Sets how many times the user can die and respawn within a single round. Trooper classes typically get 4 (5 lives total); Hero/Sith classes get 0 (one life).

## Per level

- **Level 1** — 1 respawn (2 lives total).
- **Level 2** — 2–3 respawns.
- **Level 3** — 4+ respawns (full trooper budget).

## Notes

- Sets the *additional* spawn count, not total lives. 0 means one-life-only.
- Overrides class-default if set in custom builds.
- Sibling of MBCH `extralives` field — same concept, different level of granularity.

---

`survival` · `respawn` · `general`

<!-- icon-suggestion: respawns -->
