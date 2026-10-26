# Damage Taken Multiplier

`MB_ATT_DMG_TAKEN_MULTIPLIER`

> Multiplier for all incoming damage.

## What it does

Scales every damage source the user receives. Default 1.0. Jedi/Sith often have a hidden 0.9 (10% reduction) baked into their class. 0.5 means take half damage; 1.5 means take 50% more.

## Notes

- Applied late in the damage pipeline, after armor and Cortosis.
- Combine with `MB_ATT_BLAST_ARMOUR` / `MB_ATT_CORTOSIS` for layered defenses.
- Sibling of `MB_ATT_DMG_GIVEN_MULTIPLIER`.

---

`multiplier` · `damage` · `defense`

<!-- icon-suggestion: damage-take -->
