# E-Web Cannon

`MB_ATT_EWEB`

> Charge count for the deployable E-Web turret.

## What it does

Tracks how many `HI_EWEB` charges the user has and possibly the deployed cannon's HP. Each charge places one cannon; once destroyed (or repacked), the charge is consumed.

## Per level

- **Level 1** — 1 charge.
- **Level 2** — 2 charges.
- **Level 3** — 3 charges plus durability boost.

## Notes

- Sibling of the holdable `HI_EWEB`.
- Deployed cannon inherits the user's blaster weapon flags (`HELD_HIGHDAMAGE` etc.).
- Backing up un-deploys; the cannon does not vanish, just becomes redeployable.

---

`deployable` · `turret` · `heavy`

<!-- icon-suggestion: eweb -->
