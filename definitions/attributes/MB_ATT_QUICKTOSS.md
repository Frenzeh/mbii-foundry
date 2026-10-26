# Quicktoss

`MB_ATT_QUICKTOSS`

> Throws a secondary grenade type without weapon swap.

## What it does

Companion to `MB_ATT_QUICKTHROW` — fires the alternate grenade slot (Frags or Pulse Grenades, whichever isn't on Quickthrow) on the bound special key. Both Quickthrow and Quicktoss route through `Cmd_FireThermal_f` but with different grenade selection.

## Per level

- **Level 1** — 1 charge.
- **Level 2** — 2 charges.
- **Level 3** — 3+ charges with faster cooldown.

## Notes

- Bound to `EAS_HI_QUICKTOSS`.
- Cost: 35 of class resource per throw, 2000 ms cooldown.
- Sibling of `MB_ATT_QUICKTHROW`.

---

`special` · `grenade` · `quickfire`

<!-- icon-suggestion: thermal-toss -->
