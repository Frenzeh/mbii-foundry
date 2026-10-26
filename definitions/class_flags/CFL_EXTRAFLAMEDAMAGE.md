# Extra Flame Damage (Dead Code)

`CFL_EXTRAFLAMEDAMAGE`

> Historical flag for boosted flamethrower output. **Commented out in `bg_saga.h` and `bg_saga.c` — not currently compiled.**

## What it does

Originally intended to double flamethrower damage and swap the visual to blue fire. In current source, the `throwerType` enum values 1/2 are used directly in `w_force.c:2228` and the flag check is dead (left as a comment). Setting this in FA data produces no effect.

## Notes

- Equivalent behavior today is expressed via weapon thrower-type parameters rather than a class flag.
- The enum entry is commented in `bg_saga.h:42`; `ENUM2STRING` is commented in `bg_saga.c:442`. The flag is still recognized by the parser as an identifier only if the compile reintroduces it.
- Leave disabled in FA specs for now; keep the file for historical reference and in case the flag is revived.

---

`deprecated` · `dead-code` · `flamethrower`
