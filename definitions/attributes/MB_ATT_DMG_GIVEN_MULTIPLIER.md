# Damage Given Multiplier

`MB_ATT_DMG_GIVEN_MULTIPLIER`

> Multiplier for all damage this character outputs.

## What it does

Scales every damage source the user produces — gun shots, saber hits, explosions, Force damage. Default 1.0; set 0.8 for a "soft hitter", 1.2 for a glass cannon variant.

## Notes

- Applies after weapon-level multipliers (`HELD_HIGHDAMAGE` etc.).
- Useful for class balance tuning without editing every weapon attribute.
- Sibling of `MB_ATT_DMG_TAKEN_MULTIPLIER`.

---

`multiplier` · `damage` · `offense`

<!-- icon-suggestion: damage-give -->
