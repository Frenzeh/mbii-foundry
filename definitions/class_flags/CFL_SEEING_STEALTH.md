# Seeing Stealth

`CFL_SEEING_STEALTH`

> Hides the class from Force Sense (the user becomes invisible to `FP_SEE`).

## What it does

Despite the misleading name, this flag makes the *wearer* undetectable by Force Sense. In `g_active.c`, when a Force-user scans via `FP_SEE`, any targets carrying this flag are dropped from the broadcast list — they do not appear on the Sense overlay unless they have been hit by a Tracker Dart (`PW_TRACKED_R/B` powerup bypasses the flag).

## Notes

- Code confirms: `g_active.c:5151` — `if (MB_FA_MODE && classflags & (1<<CFL_SEEING_STEALTH)) continue;` on the Sense scan loop.
- Tracker Dart / `PW_TRACKED` powerup overrides the stealth — tagged targets stay visible.
- Stealth classes (Cad Bane, infiltrators, assassin droids) pair this with low-profile weapons.

---

`stealth` · `anti-force` · `utility`
