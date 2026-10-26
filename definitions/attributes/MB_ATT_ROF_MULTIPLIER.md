# Rate of Fire Multiplier

`MB_ATT_ROF_MULTIPLIER`

> Multiplier for weapon firing speed.

## What it does

Scales the inter-shot delay across all the user's weapons. Lower values fire faster, higher fire slower. Engine treats this as a fire-time multiplier, so 0.8 = 25% faster fire rate.

## Notes

- Applies to all weapons; for weapon-specific tuning use `rateOfFire` per-weapon.
- Used to balance high-damage classes by slowing their cycle, or to make support classes more gunner-like.
- Sibling of `MB_ATT_ROF_MELEE_MULTIPLIER` for melee weapons.

---

`multiplier` · `weapons` · `firerate`

<!-- icon-suggestion: rof -->
