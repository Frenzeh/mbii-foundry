# Lightsaber

`WP_SABER`

> The elegant weapon of a more civilized age. Multi-style melee with parry and reflection.

## What it does

The lightsaber is a melee weapon with seven stances (Blue/Yellow/Red/Cyan/Purple/Staff/Dual) — each with distinct damage, swing speed, and Body Point cost. Sabers reflect blaster bolts when in `MB_ATT_FP_SABER_DEFENSE`, can throw with `MB_ATT_FP_SABERTHROW`, and parry with timing-window block. Stance choice defines the entire combat style of a Jedi/Sith class.

## Primary fire

- **Mode** — Swing (style-dependent)
- **Fire rate** — 100ms (animation drives actual cadence)
- **Damage** — Style-dependent (see below)

## Secondary fire

- **Mode** — Special (kick / saber throw / dual-blade-flourish per style)
- **Fire rate** — 100ms

## Damage by style (base)

- **Blue (fast)** — ~120 max combo
- **Yellow (medium)** — ~240
- **Red (strong)** — ~520 (massive single hit)
- **Cyan** — ~120 (fast variant)
- **Purple (Desann)** — ~400 (heavy, wide swings)
- **Staff** — ~260 (rapid double-blade)
- **Duals** — ~260 (fast dual-wield)

## Notes

- Pairs with `MB_ATT_FP_SABER_OFFENSE`, `MB_ATT_FP_SABER_DEFENSE`, `MB_ATT_FP_SABERTHROW`.
- Block, deflect, and parry mechanics live in `bg_pmove.c` / `bg_saberLoadSave.c`.
- Lockout windows on each style — committing to Red leaves you vulnerable mid-swing.
- Defense rank determines whether you reflect bolts away or eat them.

---

`saber` · `melee` · `force-user`

<!-- icon-suggestion: w_icon_lightsaber -->
