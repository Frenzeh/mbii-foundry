# Quickthrow

`EAS_HI_QUICKTHROW`

> Throws a thermal detonator without switching weapons.

## What it does

Calls `Cmd_FireThermal_f`, lobbing a thermal at the target without leaving the current weapon. Lets gunner classes maintain their primary while still landing chip damage and crowd-control from explosives.

## Per level

(Capacity and refresh rate come from `MB_ATT_QUICKTHROW`.)

## Notes

- Cost: 35 of the class resource (Energy/Stamina) per throw, 2000 ms cooldown.
- Both `EAS_HI_QUICKTHROW` and `EAS_HI_QUICKTOSS` route to the same fire function — they're bound to different keys but pull from the same charge pool.
- Hero binds this to special3 in the stock kit.

---

`special` · `grenade` · `quickfire`

<!-- icon-suggestion: thermal-throw -->
