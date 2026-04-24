# Turret Gun

`WP_TURRET`

> Map-mounted automated turret weapon. Entity-spawned, not a player loadout pick.

## What it does

`WP_TURRET` is a turret-entity weapon used by automated map turrets and certain emplacements. Distinct from `WP_EMPLACED_GUN` in that it's typically AI-controlled or remote-fired rather than manually mounted. Fire rates are zero in the base data (event-driven by the turret entity, not the weapon table).

## Primary fire

- **Mode** — Driven by turret AI / entity logic
- **Fire rate** — Entity-controlled

## Secondary fire

- **Mode** — Same as primary (no separate alt)
- **Fire rate** — Entity-controlled

## Notes

- Not a player-purchasable weapon — spawned by the map or by special abilities.
- No paired `MB_ATT_*` (attribute is `MB_ATT_INVALID`).
- See `g_turret.c` (or equivalent) for the entity behavior.

---

`turret` · `entity` · `automated`

<!-- icon-suggestion: w_icon_turret -->
