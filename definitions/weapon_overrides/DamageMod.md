# DamageMod

`DamageMod 0.6`

> Multiplier on the weapon's per-shot damage.

## What it does

Scales damage dealt by this weapon for this class. `0.6` = 60% of base; `1.5` = 150% of base.

## Valid values

Float, typical range 0.1..3.0.

## Notes

- Applied multiplicatively over the base weapon damage.
- Stacks with global class-level `damageGiven` and incoming `damageTaken` modifiers.
- Use this for "weaker pistol on a medic" or "buffed sniper for a sharpshooter" archetypes.

---

`mbch` · `weapon` · `override` · `damage`
