# Disruptor Walls

`CFL_DISRUPTOR_WALLS`

> Fully charged Disruptor shots punch through walls and doors.

## What it does

`g_weapon.c:3223` — on a full-charge Disruptor shot, if the firer has this flag **and** `MB_ATT_DISRUPTOR` at rank 2+, the projectile's trace ignores normal solid-geometry blocking and passes through walls/doors, hitting enemies in cover.

## Notes

- Gated on both the flag and `MB_ATT_DISRUPTOR >= FORCE_LEVEL_2` — rank 1 disruptor doesn't get wallbangs.
- Requires a full charge — quick shots do not penetrate.
- Extreme sniper-boss flavour; almost never given to regular roster classes.

---

`sniper` · `weapon-mod` · `penetration`
