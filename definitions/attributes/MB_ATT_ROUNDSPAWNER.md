# Round Spawner

`MB_ATT_ROUNDSPAWNER`

> NPC summoned at round start for this class.

## What it does

NPC spawn associated with this class — fires once at round start, not on demand. Used in custom Legends/FA content to give a player class a passive companion or auto-spawned ally.

## Per level

- **Level 1** — Tier 1 NPC at round start.
- **Level 2** — Tier 2 NPC.
- **Level 3** — Tier 3 NPC.

## Notes

- One-shot at round start; not reusable mid-round.
- Distinct from `MB_ATT_SPAWNER` which is a usable holdable.
- NPC type is configured in the FA data, not by this attribute alone.

---

`legends` · `npc` · `summon` · `roundstart`

<!-- icon-suggestion: spawner-round -->
