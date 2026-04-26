# WeaponName

`WeaponName "Custom Display Name"`

> Display name shown in HUD / weapon-switch UI when this weapon is selected.

## What it does

Replaces the engine-default weapon name for this class only. Does not affect other classes carrying the same WP_*.

## Valid values

Quoted string. Spaces and Unicode allowed.

## Notes

- Use to surface lore-flavored names without renaming the global weapon.
- Truncated by HUD width — keep under ~24 characters.

---

`mbch` · `weapon` · `override` · `display`
