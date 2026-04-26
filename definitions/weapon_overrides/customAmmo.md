# customAmmo

`customAmmo 60`

> Override starting ammo / max ammo for this weapon on this class.

## What it does

Sets a class-specific ammo capacity for this weapon override. Replaces the engine-default ammo amount. Used for both starting ammo and ammo cap.

## Valid values

Positive integer (typically 1..999).

## Notes

- `clipSize` controls magazine size (per-reload chunk); `customAmmo` controls total reserves.
- Setting too high may interact poorly with ammo-regen flags.

---

`mbch` · `weapon` · `override` · `ammo`
