# Elite Trooper

`MB_CLASS_ELITETROOPER`

> Rebel officer / heavy specialist — rebel mirror of Commander with a different kit emphasis.

## Role

The Rebellion's elite rifle-infantry. Same chassis as Commander (80 HP, 225 speed, 10/20/30/40 armor), Energy resource for Dodge, but fewer default ability slots — the FA designer is expected to fill in the identity via `MB_ATT_ET_CCTRAINING` (a speed-boost tree on `FP_SPEED`) and heavier-weapon picks like T-21 or A280.

## Default loadout

- **Base health** — 80
- **Base speed** — 225.0
- **Armor pool** — `AP_10_20_30_40_M40`
- **Melee moves** — All
- **Resource** — Energy
- **Abilities (CS1-4)** — `EAS_HI_DODGE`, none, none, none
- **Forcecfg** — `forcecfg/red/commander`
- **Model** — `rebel_guerilla`

## Signature mechanics

- `MB_ATT_ET_CCTRAINING` provides progressive movement-speed scaling independent of weapon type.
- Energy pool feeds Dodge and other active-defense attributes.

## Counters

- Same weaknesses as Commander — sabers, explosives, heavy blasters.
- No innate Rally; less team-utility than Imperial Commander if both sides pick vanilla.

---

`rebel` · `elite` · `frontline` · `heavy`

<!-- icon-suggestion: com -->
