# Wrist Rocket

`EAS_HI_ROCKET`

> Fires the wrist-mounted rocket without weapon swap.

## What it does

Calls `Cmd_FireRocket_f`, launching a single guided rocket from the wrist hardpoint. Mandalorian-flavored — lets the player hold a primary blaster and still land an explosive with the special key.

## Notes

- 2000 ms cooldown; cost is paid in `MB_ATT_ROCKET` ammo, not Force.
- Bound to Mandalorian special1 in the stock kit.
- Damage and tracking style scale with the underlying `MB_ATT_ROCKET` attribute, not this binding.

---

`special` · `mandalorian` · `wrist` · `explosive`

<!-- icon-suggestion: rocket -->
