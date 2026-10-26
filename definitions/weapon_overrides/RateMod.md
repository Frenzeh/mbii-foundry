# RateMod

`RateMod 1.2`

> Multiplier on weapon's fire rate.

## What it does

Scales the weapon's rate of fire. `1.2` = fires 20% faster; `0.8` = fires 20% slower.

## Valid values

Float, typical range 0.25..2.5.

## Notes

- Modifies cooldown between shots — high values reduce delay.
- Stacks with attribute-driven rate buffs (e.g. ROF point-buy).
- Engine may cap absolute fire rate to prevent network desync.

---

`mbch` · `weapon` · `override` · `timing`
