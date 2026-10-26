# Akimbo Pistol 3

`CFL_AKIMBOPISTOL3`

> Upgrades dual-wielded pistols (Westar, Bryar, Clone Pistol) to Level 3 performance.

## What it does

When a class with this flag akimbo-fires a pistol (`g_weapon.c` dual-shot paths, `bg_pmove.c` crouch/akimbo handling), the shot is processed at Pistol 3 damage scaling for both guns. Applies to `WP_MANDO_PISTOL`, `WP_BRYAR_OLD`, and `WP_CLONEPISTOL` akimbo variants.

## Notes

- Ammo drains faster — both barrels fire as Level 3 shots each.
- `g_weapon.c:1791` notes the dual-pistol charge shot is additionally scaled down (1.5x vs single pistol's 1.7x) because charged dual-pistol damage was oppressive.
- Elite Mandalorian / Jango / Boba FA specs typically carry this.

---

`weapon-mod` · `pistol` · `mando`
