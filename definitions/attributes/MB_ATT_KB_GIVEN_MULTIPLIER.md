# Knockback Given Multiplier

`MB_ATT_KB_GIVEN_MULTIPLIER`

> Multiplier for how far this character knocks enemies back.

## What it does

Scales the knockback magnitude of every attack the user lands — saber swings, melee strikes, explosions. Lets you make a class hit "heavier" without raising raw damage. Default is 1.0.

## Notes

- Wookiees and SBDs typically have values above 1.0 to feel impactful in melee.
- Stacks multiplicatively with weapon-level knockback flags (`HELD_KNOCKBACK`).
- Sibling of `MB_ATT_KB_TAKEN_MULTIPLIER` (incoming knockback).

---

`multiplier` · `knockback` · `damage`

<!-- icon-suggestion: knockback-give -->
