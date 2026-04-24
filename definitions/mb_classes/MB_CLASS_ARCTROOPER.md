# ARC Trooper

`MB_CLASS_ARCTROOPER`

> Republic special-forces — elite Clone chassis with Dexterity and WESTAR-M5 access.

## Role

The elite Clone variant: 100 HP, same 187.5 speed as Clone Trooper, premium armor pool (25/50/75/100), Stamina resource shared with regular Clones. Defaults expose Sprint + Dex rolls. ARC Troopers gate on the WESTAR-M5 (`MB_ATT_WESTARM5`) which can equip either a scope or an underbarrel grenade launcher via the ARC-specific attributes.

## Default loadout

- **Base health** — 100
- **Base speed** — 187.5
- **Armor pool** — `AP_25_50_75_100_M100`
- **Melee moves** — All
- **Resource** — Stamina
- **Abilities (CS1-4)** — `EAS_HI_SPRINT`, `EAS_HI_DEX`, none, none
- **Forcecfg** — `forcecfg/red/arc`
- **Model** — `clonetrooper_p1` (skin `arc`)

## Signature mechanics

- **ARC-only attributes** — `MB_ATT_WESTARM5`, `MB_ATT_ARC_RIFLE_SCOPE`, `MB_ATT_ARC_RIFLE_GRENADELAUNCHER`. Scope and Grenade Launcher are mutually exclusive — Calendar Events code swaps one off when setting the other.
- **Dexterity** — acrobatic roll/getup ability on CS2; key to ARC survivability.
- **Shared with Clone** — `MB_ATT_CLONEBLOBS`, `MB_ATT_STRONGBLOBS`, `MB_ATT_CLONERIFLE`, `MB_ATT_CCTRAINING`.

## Counters

- Same as Clone — sabers, Wookiees, dart-based DoTs.
- Rocket / TD AoE.

---

`republic` · `clone` · `ranged` · `elite`

<!-- icon-suggestion: arc -->
