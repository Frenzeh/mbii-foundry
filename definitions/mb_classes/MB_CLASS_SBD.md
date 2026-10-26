# Super Battle Droid

`MB_CLASS_SBD`

> CIS walking hardpoint — huge HP pool, wrist blaster, no shields, no locational damage.

## Role

The CIS "tank" class. SBDs have the highest base HP of any non-Wookiee (`SBD_HEALTH_0` = 110, scaling up to 240 at `MB_ATT_HULL_STRENGTH` 3), no armor pool (they're pure HP), and the Battery resource that drives their wrist blaster. Droids ignore locational damage and cannot pick up dropped weapons or armor — they live and die on what they spawn with.

## Default loadout

- **Base health** — 110 (up to 240 with Hull Strength ranks)
- **Base speed** — 250.0
- **Armor pool** — `NO_ARMOR_POOL`
- **Melee moves** — Kick only (`MM_KICK`)
- **Droid** — yes (`isDroid = qtrue`)
- **Scale** — 1.1x (larger hitbox)
- **Resource** — Battery (fuels wrist blaster)
- **Abilities (CS1-4)** — `EAS_HI_RECHARGE`, `EAS_HI_SBD_PM`, `EAS_HI_SBD_ZOOM`, none
- **Forcecfg** — `forcecfg/blue/SBD`
- **Model** — `sbd`

## Signature mechanics

- **Droid traits** — no locational damage, immune to poison, cannot use bacta / stims, ignored by Force Heal/Drain/Speed/Rage etc.
- **No pickups** — droids cannot grab ammo, armor, or dropped weapons.
- **Wrist blaster** — primary ranged weapon. Battery resource depletes per shot; `EAS_HI_RECHARGE` triggers a standing cooldown refill.
- **SBD Zoom** (`EAS_HI_SBD_ZOOM`) — binocular-equivalent zoom without a holdable slot.
- **Any resource assigned to this class behaves as Battery** (existing documentation note — true for all non-droid resources remapped onto SBDs).

## Counters

- EMP / ion weapons (Clone Blobs, DEMP2, Ion Rifle).
- Force Grip can immobilize despite droid immunity to Push/Pull variants.
- Rocket Launcher and Thermal Detonators one-shot the base HP pool.

---

`cis` · `droid` · `tank` · `ranged`

<!-- icon-suggestion: sbd -->
