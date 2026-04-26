# Changelog

All notable changes to MBII Foundry are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions follow
[Semantic Versioning](https://semver.org/) where practical, with alpha/beta
pre-release suffixes until the project stabilizes.

## [Unreleased]

### Added
- CI workflow (build + vet + gofmt + test across Linux/macOS/Windows)
- Release workflow (triggered on `v*` tags, builds + publishes binaries)
- Issue templates (bug, feature, definition error) + PR template
- Dependabot config for Go modules and GitHub Actions
- Self-contained parser round-trip tests
- README install badges + "Where do I find .mbch files to edit?" section

## [0.11.7-alpha] — 2026-04-26

### Fixed
- **Weapon Flags + Weapon Mods pickers were missing weapons** that exist in
  the canonical catalog but lack a custom HUD icon (e.g. `WP_BLASTER_PISTOL`).
  Both pickers now source from `MBIIWeapons` unioned with `weaponIconAliases`
  rather than only the alias table.
- **Apply button on the source panel now reports "no source changes to push
  back to the form" via the status bar** when clicked with no edits. Previously
  silent — clicking Apply with no changes looked like the button was broken.
- **MBCH editor's freshly-loaded files no longer show as dirty** — programmatic
  `SetText` / `SetSelected` during `updateUI` now silenced via a `loading` guard
  so the dirty flag doesn't fire on every widget reset. Belt-and-suspenders
  `MarkClean()` after `LoadFile` for any callback that slips through.

### Changed
- **Resources umbrella section** added to the Attributes tab (first, default
  open). Pulls anything that controls a pool or regen rate (Force pool, Fuel,
  Stamina, Battery, BP/HP/AR regen, FORCEFOCUS, …) plus a new "Class Scalars"
  sub-tile that surfaces the static `apMultiplier` / `bpMultiplier` /
  `csMultiplier` / `asMultiplier` / `forceRegen` / `speed` ClassInfo float
  fields. The previous floating banner above the grid is gone.
- **Section-first visual hierarchy** in the Attributes grid — each sub-bucket
  (Pistols, Rifles, Heavy, Force/Core, Force/Defensive, …) now wraps in its
  own TilePanel with a per-bucket accent color. Per-attribute fill bumped
  back up to `12/55` so each row keeps its category color cue without
  competing with the section background.
- **Boxicon-style fallback iconography** — 22 minimal outline SVGs (heart,
  shield, bolt, flame, snowflake, droplet, fuel, footstep, gear, target,
  explosion, saber, fist, wave, eye, battery, timer, wrench, bag, skull,
  refresh, dash, jetpack, atom, swap, star, box). Keyword mapper picks the
  best-fit icon for any attribute lacking MBII HUD art. Includes
  weapon-archetype matches so Pistols / Rifles / Launchers / Grenades /
  Melee weapon-attribute rows pick up glyphs.
- **`MB_ATT_TURN_RATE`** moved from "General" to "Class Specific" (it's a
  Droideka roll-mode stat).
- **`MB_ATT_*_MULTIPLIER` (13 entries)** filtered out of the Attributes grid
  — they're point-buy primitives, surfaced in the Point Buy tab's slot picker.
  New "Custom Multipliers" hint card on the Point Buy tab explains the move.

### Added
- **Hover-swap toggle** in the Info Panel header. Default OFF — hover events
  on grids no-op; click/focus on a field still updates the panel. Click the
  eye icon to opt into "peek the docs on hover" mode.
- **`MB_CLASS_OBSERVER`** is now selectable (FA round-spawner / pit slot).
- **Weapon Inventory rehaul (first cut)** — full-width cards on the Inventory
  tab: 48px icon · weapon name + WP_* · level pills bound to the paired
  `MB_ATT_*` · Flags + Override badges. Per-category accent color (violet
  melee, teal sidearms, green rifles, amber heavy).
- **Description rail** on the Profile tab — drag the splitter to grow the
  description into the available height instead of pushing identity fields
  off-screen.
- **Per-block byte-budget validation** — ClassInfo (8192), WeaponInfoN (4096),
  ForceInfoN (2048), per-key value (2048). Warns at 90%, errors at 100%.
- **R22.0.00 batch 2** (10 new HELD_* flags, 4 EAS specials, WeaponInfo
  override field reference docs in `definitions/weapon_overrides/`).

## [0.11.1-alpha] — 2026-04-23

### Added
- **R22.0.00 absorption batch 2**:
  - 10 new `HELD_*` weapon flags surfaced in the editor and reference
    library: `HELD_LIFT`, `HELD_SLIPPERY`, `HELD_DISARM`, `HELD_NODISARM`,
    `HELD_PULL`, `HELD_CRIPPLE`, `HELD_FORCEFOCUS`, `HELD_LIFESTEAL`,
    `HELD_FLASH`, `HELD_BACTA`.
  - 4 new class specials: `EAS_HI_PETCONTROL`, `EAS_HI_PSHIELD`,
    `EAS_FP_DARKRAGE`, `EAS_FP_STASIS`.
  - `MB_CLASS_OBSERVER` promoted out of the hidden list — now selectable
    in the class picker for FA round-spawner / pit setups.
- **WeaponInfo override field reference** at
  `definitions/weapon_overrides/` — per-field documentation for
  `WeaponToReplace`, `WeaponBasedOff`, `Icon`, model / view / hands /
  barrel paths, muzzle / missile / charge / hitscan effects, flash /
  charge / select sounds, ammo + clip + reload mods, damage / rate /
  velocity multipliers, force-pool multipliers, and the per-block
  animation override set.
- New Library accordion sections — Weapon Flags, Class Specials, and
  Weapon Overrides — surface the new docs without dumping them into
  the catch-all Glossary.
- Per-block byte-budget validation: ClassInfo (8192), WeaponInfoN
  (4096), ForceInfoN (2048), and per-key value cap (2048). Warnings
  fire at 90% of budget; "exceeds" issues at 100%. Catches over-stuffed
  blocks before the engine truncates them silently at load time.

### Fixed
- `go vet` warning in `logger.go` (non-constant format string in panic handler)
- `gofmt` applied across the entire Go codebase

### Changed
- `TestParseRealFiles` now skips cleanly when a TextAssets checkout isn't
  available (was previously relying on hardcoded relative paths)

---

## [0.1.0-alpha] — 2026-04-22

Initial public alpha.

### Added
- Visual editor for `.mbch`, `.sab`, `.veh`, `.siege` files (Fyne GUI)
- Curated enum metadata: 278 attributes, 53 weapons, 25 classes, 8 saber
  styles, 18 class flags, force powers, holdables
- Per-enum markdown documentation (371 polished, 164 stubs pending)
- Asset browser for PK3 contents
- Legends point-buy simulator
- Modpack packager
- macOS `.app` bundle build script
- Optional Holocron AI integration (pairs with a local backend if present)

### Known Issues
- 30.7% of enum definitions are AI-generated stubs awaiting human prose
  (see `docs/DEFINITIONS_GUIDE.md`)
- No prebuilt binaries in this release — build from source
