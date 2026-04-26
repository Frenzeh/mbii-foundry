# WeaponBasedOff

`WeaponBasedOff WP_*`

> Stat inheritance source for fields the override does not explicitly set.

## What it does

Specifies which base weapon's stats fall through for any field not declared in the override block. If omitted, defaults to the value of `WeaponToReplace`.

## Valid values

Any `WP_*` constant.

## Notes

- Useful for re-skinning: `WeaponToReplace WP_REPEATER` + `WeaponBasedOff WP_T21` makes the repeater slot fire like a T-21.
- Animation, fire rate, projectile behavior, force costs all inherit from this if not overridden.

---

`mbch` · `weapon` · `override`
