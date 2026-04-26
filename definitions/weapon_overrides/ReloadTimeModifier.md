# ReloadTimeModifier

`ReloadTimeModifier 0.75`

> Multiplier on the base weapon's reload duration.

## What it does

Multiplies the base weapon's reload time by this factor. `0.75` = 25% faster reload; `1.5` = 50% slower.

## Valid values

Float, typical range 0.25..2.0. Engine clamps absurd values.

## Notes

- Stacks multiplicatively with attribute-driven reload bonuses.
- Effect is per-class — does not change other classes' reloads on the same WP_*.

---

`mbch` · `weapon` · `override` · `timing`
