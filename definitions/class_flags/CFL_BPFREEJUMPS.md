# BP-Free Jumps

`CFL_BPFREEJUMPS`

> Jumping does not drain Block Points (saber defense).

## What it does

In `bg_pmove.c`, when a force-user jumps, the normal BP cost is skipped if this flag is set. Also checked in vehicle-mount contexts and `MB_ATT_FP_SABER_DEFENSE 3` overrides. Designed for agile duelist characters (Yoda, Maul) whose identity is constant motion without penalty.

## Notes

- Only meaningful on saber-using (`isFS`) classes — non-force-users don't have a BP pool to drain.
- Functionally stacks with `MB_ATT_FP_SABER_DEFENSE 3`, which has the same effect as a passive benefit of rank 3 defense.

---

`force-user` · `mobility` · `defense`
