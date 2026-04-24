# No Pickups

`CFL_NOPICKUPS`

> Blocks the class from picking up dropped weapons, ammo, armor, or items.

## What it does

Server-side pickup handler (`g_items.c:3323`) checks this flag in FA mode and refuses any item touch for the class. The player spawns with their loadout and cannot scavenge — essential for droid classes and any balance-sensitive spec that shouldn't be able to grab a dropped Rocket Launcher.

## Notes

- Droids (SBD, Droideka, other droid-flagged classes) have this naturally applied through the droid code path.
- In FA, often combined with high-HP / low-armor setups where pickups would trivialize the trade.

---

`restriction` · `loadout` · `balance`
