# WeaponToReplace

`WeaponToReplace WP_*`

> Required. Specifies which base weapon this override block replaces for the class.

## What it does

Sets the WP_* enum that this override targets. When the class equips that weapon, the engine reads the override block instead of the base weapon definition.

## Valid values

Any non-sentinel `WP_*` constant (e.g. `WP_BLASTER`, `WP_REPEATER`, `WP_SABER`, `WP_FLECHETTE`).

## Notes

- Only one override block per WP_* per class — repeated targets are ignored.
- The block index (`WeaponInfo0` vs `WeaponInfo1`) is not significant; targeting is via this field.
- Pair with `WeaponBasedOff` if you want stats inherited from a different weapon than the target.

---

`mbch` · `weapon` · `override` · `required`
