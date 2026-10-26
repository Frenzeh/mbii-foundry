# Tenloss Disruptor Rifle

`WP_DISRUPTOR`

> Illegal sniper rifle. Snap-fire primary; chargeable disintegrator scope alt.

## What it does

The Disruptor is the precision sniper of the Bounty Hunter / Mercenary side. Primary shoots a fixed-power disruptor bolt; secondary holds a charge through the `ZOOM_ADJUST` scope and releases an increasingly lethal beam — at full charge it can punch through Force shields and disintegrate the kill. Best at long lanes; weaker than other rifles up close.

## Primary fire

- **Damage** — 31
- **Ammo cost** — 5
- **Fire rate** — 1000ms

## Secondary fire

- **Mode** — Charged scoped beam (`CHGE_RUPTOR1`)
- **Damage** — 36 base, scaling to ~125+ at full charge
- **Ammo cost** — 6
- **Fire rate** — 1300ms
- **Effect** — Can shoot through Force shielding at high charge; disintegrates on lethal hit

## Notes

- Pairs with `MB_ATT_DISRUPTOR`.
- Force drain (Close/Far): 26/14 primary; blocking 17/11.
- Scope walks (`scopeWalk`); zoom is charge-coupled (`zoomCharges`).
- Bounty Hunter, Mandalorian, and Imperial Officer signature pick.

---

`sniper` · `disruptor` · `charge`

<!-- icon-suggestion: w_icon_disruptor -->
