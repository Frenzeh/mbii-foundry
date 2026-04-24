# Droideka

`MB_CLASS_DROIDEKA`

> CIS destroyer droid — shielded, fast-rolling, no armor, no pickups.

## Role

A dedicated vehicle-style class: the Droideka is piloted as a vehicle entity (the pilot's health/armor are tracked via `entVehicleAsPlayer`). Deploy-mode grants a personal shield plus the twin repeating blasters; roll-mode grants mobility but disables fire. Droid flags apply — no locational damage, immune to most buffs, cannot pick up items, cannot be force-healed / drained / rage'd etc.

## Default loadout

- **Base health** — 260 (up to 520 with Deka Hull 2+)
- **Base speed** — 250.0
- **Armor pool** — `NO_ARMOR_POOL`
- **Melee moves** — none (`MM_NONE`)
- **Droid** — yes
- **Scale** — 0.7x (small crouched hitbox)
- **Resource** — Deka Shield (energy-bubble pool)
- **Abilities (CS1-4)** — none by default
- **Forcecfg** — `forcecfg/blue/deka`
- **Model** — `droideka`

## Signature mechanics

- **Vehicle-piloted** — the Droideka is implemented as a player piloting a vehicle, which is why HoloPad's stat tracking and some damage calcs traverse `entVehicleAsPlayer`.
- **Droideka-only attributes** — `MB_ATT_DEKA_SHIELD`, `MB_ATT_DEKA_HULL`, `MB_ATT_DEKA_DEPLOY`, `MB_ATT_DEKA_POWER`, `MB_ATT_DEKA_FIREPOWER`.
- **Deploy/roll toggle** — deploy = shield + fire, roll = mobility + vulnerability. `MB_ATT_DEKA_DEPLOY` shortens the transition animation.
- **No pickups / droid immunity** — as with SBD.

## Counters

- Ion / EMP weapons (DEMP2, Pulse Grenades, Clone Blobs) strip the shield quickly.
- Rockets / TDs hit the pilot through the shield's outer bubble when it's dropped.
- Sabers can close the gap during a deploy animation.

---

`cis` · `droid` · `tank` · `heavy`

<!-- icon-suggestion: deka -->
