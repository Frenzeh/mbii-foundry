# clipSize

`clipSize 12`

> Override magazine size for this weapon override.

## What it does

Sets how many shots are in one magazine before a reload is required. Distinct from total ammo (`customAmmo`).

## Valid values

Positive integer (typically 1..200).

## Notes

- Pair with `ReloadTimeModifier` to balance reload-frequency vs. reload-duration.
- Setting clipSize == customAmmo effectively disables reloading (one-mag ammo pool).

---

`mbch` · `weapon` · `override` · `ammo`
