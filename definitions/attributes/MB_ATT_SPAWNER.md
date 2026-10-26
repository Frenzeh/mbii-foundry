# NPC Spawner

`MB_ATT_SPAWNER`

> Configures which allied NPC the Spawner Beacon summons.

## What it does

When the user activates `EAS_HI_SPAWNER` / `HI_SPAWNER`, an allied NPC of the configured type spawns at the beacon location. The rank picks one of three NPC pools defined in the FA scripting.

## Per level

- **Level 1** — Tier 1 NPC (e.g. basic trooper).
- **Level 2** — Tier 2 NPC (e.g. Elite trooper).
- **Level 3** — Tier 3 NPC (e.g. Hero unit).

## Notes

- Specific NPC type is set by the FA configuration, not by this attribute alone.
- Each spawn consumes one beacon charge.
- NPC counts toward the team-bot limit if the server enforces it.

---

`supply` · `summon` · `npc` · `legends`

<!-- icon-suggestion: spawner -->
