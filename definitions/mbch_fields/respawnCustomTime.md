# Respawn Time

`respawnCustomTime`

> Sets a custom respawn timer for this specific class.

## What it does

Sets a custom respawn timer for this specific class.

**Default:** 0 (Uses gametype default, usually 10-20s).

## Valid values

- **Value:** Milliseconds (e.g., `10000` = 10s).
- **Effect:** Overrides the server's wave spawn timer for this class.

## Notes

- Use for "fodder" classes (Soldiers) to let them spawn faster (`5000` ms).
- Use for elite classes to punish death (`30000` ms).

---

`character` · `loadout`
