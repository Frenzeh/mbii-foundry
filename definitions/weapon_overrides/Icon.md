# Icon

`Icon gfx/hud/w_icon_*`

> HUD weapon-select icon for this override.

## What it does

Custom icon shown in the weapon-switch UI and ammo display. Path is a shader / image relative to the gamedata root.

## Valid values

Shader path, typically under `gfx/hud/`. Examples:
- `gfx/hud/w_icon_blaster`
- `gfx/hud/w_icon_repeater`

## Notes

- If omitted, the engine uses `WeaponBasedOff`'s default icon.
- Foundry's WeaponInfo editor previews the icon live as you type.

---

`mbch` · `weapon` · `override` · `visuals`
