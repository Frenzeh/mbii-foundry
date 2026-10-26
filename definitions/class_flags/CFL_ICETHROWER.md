# Icethrower (Dead Code)

`CFL_ICETHROWER`

> Historical flag for a cryo-projector variant of the flamethrower. **Commented out in `bg_saga.h` and `bg_saga.c` — not currently compiled.**

## What it does

Originally intended to convert the flamethrower into a freeze weapon. In current source, `w_force.c:2211` branches on `throwerType == 3` (the numeric value formerly bound to `CFL_ICETHROWER`) — the class-flag route is dead. Cryo behaviour is driven entirely by per-weapon `throwerType`.

## Notes

- Real cryo weapons exist as `MB_ATT_THROWER_ICE` / `WP_THROWER` with `throwerType == 3`.
- The enum is commented out in `bg_saga.h:43` and `bg_saga.c:443`.
- Leave disabled in FA specs — use the corresponding thrower attribute instead.

---

`deprecated` · `dead-code` · `cryo`
