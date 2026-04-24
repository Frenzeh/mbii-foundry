# Knockback Taken Multiplier

`MB_ATT_KB_TAKEN_MULTIPLIER`

> Multiplier for how far this character is knocked back.

## What it does

Scales incoming knockback from Force Push, explosions, and melee. Lower values make the character feel heavier and harder to displace; 0.0 makes them effectively immovable. Default is 1.0.

## Notes

- Droidekas and SBDs commonly use 0.5–0.8 for "weighty" feel.
- 0.0 makes the user immune to ledge-pushes and Force-Push displacement (use sparingly — ground knockdowns may still apply).
- Sibling of `MB_ATT_KB_GIVEN_MULTIPLIER` (outgoing knockback).

---

`multiplier` · `knockback` · `defense`

<!-- icon-suggestion: knockback-take -->
