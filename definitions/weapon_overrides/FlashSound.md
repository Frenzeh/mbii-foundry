# FlashSound[N]

`FlashSound0 sound/weapons/foo/fire1.wav`
`FlashSound1 sound/weapons/foo/fire2.wav`
... up to `FlashSound3`

> Per-shot fire sound (primary fire). Up to 4 variants are randomized.

## What it does

Replaces the muzzle-flash sound. Multiple slots (0..3) let the engine round-robin between variants so rapid fire doesn't sound identical on every shot.

## Valid values

Path to a `.wav` or `.mp3` under `sound/`. One per slot.

## Notes

- `AltFlashSound0..3` covers alt-fire.
- `SelectSound` plays when the weapon is drawn (not on fire).

---

`mbch` · `weapon` · `override` · `sound`
