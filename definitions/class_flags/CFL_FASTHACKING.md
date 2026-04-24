# Fast Hacking

`CFL_FASTHACKING`

> Speeds up usable-object hacks (Siege objectives, doors, panels).

## What it does

When the player touches a `trigger_hack`-style Siege objective, the flag is checked in `g_trigger.c` and the hack timer is applied at an accelerated rate. Used for slicer / tech-specialist FA characters whose identity is "fast objective completion."

## Notes

- Applies only in FA mode (`MB_FA_MODE` guard in `g_trigger.c`).
- Pairs with a deliberately weak weapon loadout to create pure objective-runner specs.
- R2 / R5 astromech FA presets commonly carry this.

---

`utility` · `objective` · `support`
