# Kill Team on Death (VIP)

`CFL_KILLTEAMONDEATH`

> When the flagged player dies, every member of their team dies instantly.

## What it does

`g_combat.c:3651` — on player_die, if the flag is set, `G_KillTeam()` is invoked on the deceased's team, dealing 999 damage to every teammate as if the world killed them. Forms the core of custom VIP / "Escort the VIP" scenarios.

## Notes

- Only the flagged player's death triggers the wipe; teammates dying normally do not cascade.
- Make sure only one VIP is assigned per team per round — otherwise a second VIP's death is pointless.
- Works in any gametype that runs death-event code (Open, FA, Duel-RM).

---

`vip` · `mode-specific` · `objective`
