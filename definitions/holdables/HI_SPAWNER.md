# Support Beacon

`HI_SPAWNER`

> Spawns a scripted ally NPC reinforcement.

## What it does

Drops a beacon that summons one of the configured NPC types as a friendly. Used in custom FA / Legends content for "Call Reinforcements" mechanics — what spawns is configured per-class via `MB_ATT_SPAWNER`.

## Notes

- NPC type, HP, and behavior come entirely from `MB_ATT_SPAWNER` configuration.
- One spawn per charge — no continuous summoning.
- Spawned NPC counts toward team-bot limits if the server enforces them.

---

`holdable` · `summon` · `support` · `legends`

<!-- icon-suggestion: spawner -->
