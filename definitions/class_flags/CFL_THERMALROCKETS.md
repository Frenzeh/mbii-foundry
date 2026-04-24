# Thermal Rockets

`CFL_THERMALROCKETS`

> Converts Rocket Launcher projectiles (`WP_PLX1`) into thermal-detonator-style munitions.

## What it does

Rocket-spawn and impact paths in `g_weapon.c` (6898+) branch on this flag and produce the thermal-rocket variant instead of the standard rocket. Includes different damage/radius/arc characteristics — the projectile is the "big boom" version used on heavy troopers and Siege-breakers.

## Notes

- Requires the class to carry `MB_ATT_PLX1` (or equivalent rocket-launcher attribute) to actually own the weapon.
- Significantly heavier AoE than baseline rockets; friendly-fire risk scales accordingly.
- Examples: Heavy Rocket Troopers, some Death-Star-era defender FA builds.

---

`weapon-mod` · `explosive` · `heavy`
