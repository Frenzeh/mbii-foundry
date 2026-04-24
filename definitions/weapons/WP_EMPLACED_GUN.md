# Emplaced Gun

`WP_EMPLACED_GUN`

> Static map-mounted heavy blaster. Use-trigger entity, not a personal loadout weapon.

## What it does

Emplaced guns are entity-spawned turrets on the map (e.g. on Hoth bunker walls or Endor bridges). Players mount the gun with `+use` and fire at a fixed 100ms cadence in both modes. The gun has its own ammo pool tied to the spawner, no reload, and locks the player into a turning-arc constraint while mounted.

## Primary fire

- **Mode** — Sustained heavy blaster
- **Fire rate** — 100ms

## Secondary fire

- **Mode** — Same fire profile (no separate alt)
- **Fire rate** — 100ms

## Notes

- Not a player-purchasable weapon — spawns with map geometry.
- No paired `MB_ATT_*` (attribute is `MB_ATT_INVALID`).
- Players dismount with `+use` again or by taking damage.

---

`map-mounted` · `static` · `heavy`

<!-- icon-suggestion: w_icon_emplaced -->
