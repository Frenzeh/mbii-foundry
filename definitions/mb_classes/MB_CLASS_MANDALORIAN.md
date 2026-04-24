# Mandalorian

`MB_CLASS_MANDALORIAN`

> Jetpack-mounted bounty hunter — rockets, flamer, wrist laser, Beskar armor.

## Role

The free-flying offensive specialist. 100 HP, 212.5 speed, the game's richest armor pool (25/50/75/100), Fuel resource for the jetpack. The only class flagged `isMando = qtrue`, which drives jetpack physics and wrist-gauntlet attribute wiring. Default abilities expose the Rocket and Flamethrower bindings; the FA designer typically layers Wrist Laser and Whistling Birds on top.

## Default loadout

- **Base health** — 100
- **Base speed** — 212.5
- **Armor pool** — `AP_25_50_75_100_M100`
- **Melee moves** — All
- **isMando** — yes (jetpack + gauntlet support)
- **Resource** — Fuel (jetpack)
- **Abilities (CS1-4)** — `EAS_HI_ROCKET`, `EAS_HI_FLAME`, none, none
- **Forcecfg** — `forcecfg/blue/manda`
- **Model** — `mbmandy` (skin `mbrgb1`)

## Signature mechanics

- **Mando-only attributes** — `MB_ATT_MANDO_PISTOL` (Westar), `MB_ATT_WRISTLASER`, `MB_ATT_FLAMETHROWER` (gauntlet), `MB_ATT_WHISTLINGBIRD`, `MB_ATT_BESKAR`.
- **Gauntlet mode toggle** — melee-secondary cycles `EAS_HI_FLAME` ↔ `EAS_HI_WRIST` when both Wristlaser and Flamethrower are equipped. `SMBF_GAUNTLET_MODE` stat flag tracks the state.
- **Jetpack** — Fuel resource + cooldown + overheat; `CFL_NO_JETPACK_COOLDOWN` and `CFL_NO_FUEL_USE` remove those limits per-class.
- **Wrist Laser** — 65 damage per shot, 4-shot magazine, 6s regen cooldown (`MAX_WRIST_LASER_SHOTS`).

## Counters

- Anti-air focus fire while airborne.
- Force Lightning / Ion (disrupts jetpack + armor).
- Saber deflection of Wristlaser / Rocket.

---

`imperial` · `mando` · `ranged` · `mobile`

<!-- icon-suggestion: manda -->
