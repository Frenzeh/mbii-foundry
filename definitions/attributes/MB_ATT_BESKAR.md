# Beskar Armor

`MB_ATT_BESKAR`

**Class-specific:** Mandalorian only.

Multiplicative damage reduction against specific sources. Applied *before* Armor Points absorb the rest — beskar and AP stack.

### Damage multipliers

Values below are multipliers applied to incoming damage. `×0.60` = take 60% of normal damage (40% reduction). `×1.00` = no effect.

| Source | L1 | L2 | L3 |
|---|---|---|---|
| Saber / saber throw | ×0.65 | ×0.65 | ×0.60 |
| Rocket / whistling bird / projectiles | ×0.75 | ×0.75 | ×0.70 |
| All explosives | — | — | ×0.70 |
| Torso / back | ×0.80 | ×0.80 | ×0.75 |
| Head | ×1.00 | ×0.80 | ×0.75 |
| Legs / feet | ×1.00 | ×1.00 | ×0.85 |
| Waist | ×1.00 | ×1.00 | ×0.85 |
| Fall damage | ×0.85 | ×0.85 | ×0.85 |

### What isn't reduced

- **Blaster bolts / bullet weapons** — no reduction at any level. Mitigate with AP.
- **Sniper weapons** — take **10% *more* damage** (`BESKAR_REDUCTION_SNIPER 0.10`).

### Per-level summary

- **L1**: saber + projectile/rocket + fall damage. Only torso gets locational DR.
- **L2**: L1 coverage + head DR (20%). Same damage types as L1.
- **L3**: coverage expands to *all* explosives. Adds leg and waist DR. Head DR bumps to 25%, saber to 40%, rocket to 30%.

### Related

- `MB_ATT_ARMOUR` — base AP pool; beskar reduces before AP absorbs
- `CFL_NOLOCATIONALDAMAGE` — disables beskar's locational bonuses
- `MB_ATT_DURABILITY` — separate flat-reduction mechanic (not beskar)

### Source references

- Constants: [`game/bg_weapons.h:1065-1084`](https://github.com/MBIIDevTeam/moviebattles/blob/master/game/bg_weapons.h#L1065-L1084)
- Application: [`game/g_combat.c:4920-5155`](https://github.com/MBIIDevTeam/moviebattles/blob/master/game/g_combat.c#L4920-L5155)
- Saber interaction: `game/w_saber.c:3098`
- Missile interaction: `game/g_missile.c:1721-1754`

---

*This is the reference template for a well-written definition. If the definition you're editing lacks a stats table, mechanics breakdown, and source refs, it probably needs work. See [`docs/DEFINITIONS_GUIDE.md`](../../docs/DEFINITIONS_GUIDE.md).*

<!-- icon-suggestion: icon_stats_beskar -->
