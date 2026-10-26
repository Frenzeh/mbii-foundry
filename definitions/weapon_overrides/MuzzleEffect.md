# MuzzleEffect

`MuzzleEffect effects/foo/muzzle.efx`

> Visual effect played at the muzzle on primary fire.

## What it does

Replaces the engine-default muzzle flash for this weapon. Triggered each shot at the weapon's muzzle tag.

## Valid values

Path to an `.efx` effect file (typically under `effects/`).

## Notes

- `AltMuzzleEffect` covers alt-fire.
- Effect is loaded once per fire — heavy effects with thousands of particles can hitch.
- Pair with `FlashSound` for matching audio.

---

`mbch` · `weapon` · `override` · `effect`
