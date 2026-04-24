# Stat Viewer

`CFL_STATVIEWER`

> Broadcasts extended teammate data — health, armor — to the wearer's HUD.

## What it does

Server-side, once per second the flagged player receives an extended-info packet (`G_SiegeClientExData`) listing teammate stats. Client renders this as health/armor bars above allied heads. No wallhack on enemies — it's a teammate-only visibility buff.

## Notes

- Refresh interval is hard-coded to 1000 ms in `g_main.c`.
- Standard on Medic, Commander, and support classes where triage / shot-calling matters.
- Requires Siege/FA mode (guarded by `MB_FA_MODE`).

---

`utility` · `hud` · `support`
