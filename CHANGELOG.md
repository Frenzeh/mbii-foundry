# Changelog

All notable changes to MBII Foundry are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versions follow
[Semantic Versioning](https://semver.org/) where practical, with alpha/beta
pre-release suffixes until the project stabilizes.

## [Unreleased]

## [0.13.0-alpha] — 2026-05-12

### Added
- **PK3-aware inline autocomplete** on path fields (model, skin, UI shader,
  soundset). Typing surfaces a popup of indexed paths that mixes loose
  files and PK3 contents — the VFS index already merged both sources;
  this just exposes them inline instead of requiring the browse-dialog
  detour. `VFS.Suggest(query, accept, max)` is the underlying helper;
  `AttachPathSuggest(entry, vfs, accept)` chains it non-destructively
  onto any existing `*widget.Entry` (preserves the user's prior
  OnChanged hooks).
- **File → Validate Folder…** walks a directory recursively, parses
  every `.mbch` it finds, and reports per-file results — parse errors
  plus engine-required-field failures (currently: empty `name`, which
  mirrors `BG_SiegeParseClassFile`'s `ERR_DROP` at `bg_saga.c:2341`).
  Useful for catching schema drift after an MBII patch.
- **View → Show Developer Fields** menu toggle plus `AppConfig`
  persistence. Schema fields can now opt out of the default loadout-
  editor surface with `"dev": true`; the toggle reveals them on demand.
  Initial dev-tagged set: `AB_*Flags` (six ability bitmask fields),
  jetpack tag / offset / angle triplets (six), `saberDamageStyle`,
  `forceblocking`. `MBCHEditor` exposes a `devSurfaces` registry so
  future UI subsystems append their enclosing container at build time
  and get visibility-flipped for free.

### Removed
- **Orphan `/parsers/` (Python) directory.** 3,295 lines across
  `mbch_parser.py` / `sab_parser.py` / `veh_parser.py` / `file_manager.py`
  / `__init__.py` — zero callers (Go, Python, docs, CI). The Go module
  has its own `go_module/parsers/` which is unrelated and still in use.
  Net diff for this release: **+322 / −3,309 lines**.

### Testing
- `TestDrainVariantsOrdering` (parsers): pins the v0.12.2 bug-#4 fix —
  `model_N` / `skin_N` / `uishader_N` / `saber_N` / `customred_N` emit
  adjacent to their base field in numeric order, and `GenerateMBCH` no
  longer mutates the input character.
- `TestVFSSuggestPK3Reach` (main): integration backbone for Phase 2 —
  builds a real gamedata + textassets layout with one loose file and
  one in-PK3 file, asserts `Suggest()` surfaces both, and exercises
  the `accept` filter + `max` cap.

## [0.12.3-alpha] — 2026-05-12

### Fixed
- **Mouse-wheel scroll was swallowed when the cursor was over a
  point-buy slot field** (tester bug #1 from Cinder, 4/28/26). Root
  cause: `widget.Entry` in Fyne 2.7.1 embeds its own `*widget.Scroll`
  for horizontal text-overflow (`entry.go:87,187-196`); Fyne's hit-
  testing returns the deepest matching `Scrollable` under the cursor,
  so wheel events landed on the inner Scroll and never reached the
  outer `container.NewVScroll`. Fix: new `NewSlotEntry()` helper that
  sets `Wrapping=TextWrapOff` + `Scroll=ScrollNone` together, which
  removes the inner Scroll from the render tree entirely (entry.go:189).
  Applied to all PointBuyUI entries inside scrollable regions; modal-
  dialog entries left as `NewInputEntry` since they're not in a scroll
  container.

## [0.12.2-alpha] — 2026-05-11

### Added
- **49 engine fields backfilled into the schema** (`mbch_schema.json`).
  Cross-referenced against `bg_saga.c`'s exhaustive `SGPV(cI, …)` /
  `G_SetHeldFlag` / `BG_SiegeGetPairedValue` sweeps; **zero engine
  class-level or WeaponInfo gaps remain**. Breakdown:
    - 19 class-level animation overrides (jump / land / getup variants,
      kick / punch / throw, L+R strafes, `throwAnimReleaseFrame`)
    - 12 jetpack fields (effects, sounds, jet tags, offsets, angles)
    - 6 class specials (`special[12]HUD`, `meleeSpecial1-4`)
    - 6 ability flags (`AB_WLaser/WBirds/Rocket/TDart/PDart/FLightning`)
    - 2 NPC slots (`spawnerNPC`, `roundNPC`)
    - 3 saber (`saberSpecialDamage`, `saberDamageStyle`, `forceblocking`)
    - 1 rank (`rankSaberSpecialDamage`)
    - WeaponInfo: `animReadyRun/ZoomRun`, `animAttackRun/ZoomRun`,
      `splashDamage`, `splashRadius`
    - ForceInfo: `gripDamage`

### Fixed
Tester bug round from Cinder (4/28/26):

- **#2 — Point-buy data wasn't saving.** Two duplicate widgets (one
  in the Identity accordion, one in the Point Buy tab) wrote to the
  same `IsCustomBuild` / `MBPoints` fields. The main-tab `mbPointsEntry`
  had no `OnChanged`, so typing into it never updated the character
  until save time, when `updateCharacterFromUI` re-read its (still
  `"0"`) text and clobbered any live value set by the Point Buy tab.
  Fix: bidirectional mirror between the two views, plus a live
  `OnChanged` on the main-tab entry.
- **#3 — Empty `name` produced files that fail to load.** Engine
  hard-errors with `Com_Error: Siege class without name entry`
  (`bg_saga.c:2341`) if the field is blank. `SaveFile` now falls back
  to the filename basename when `char.Name` is empty.
- **#4 — `model_1` / `skin_1` / `uishader_1` and other `_N` variants
  got exiled to the bottom of the ClassInfo block** via the
  `ExtraFields` alphabetic dump. New `drainVariants` writer helper
  pulls `_N` keys adjacent to their base field in numeric order.
  Operates on a copy of `ExtraFields` so callers' data is not
  mutated (the legends round-trip tests depend on this).
- **#5 / #6 — `forceregen` / `maxarmor` / `forcepool` / multipliers
  silently clamped at `10` / `999` / `999` / `10.0`** in
  `updateCharacterFromUI`. Engine has no such caps — they were UI
  inventions. Raised to `1e6` (ints) and `1000.0` (floats).

## [0.12.1-alpha] — 2026-04-27

### Fixed
- **App froze with beach-ball cursor when opening any MBCH from Recents.**
  Sample dump showed every thread parked in `__psynch_cvwait`. Root cause:
  `VFS.Refresh()` held the *write* lock for the entire 10–30 second scan
  of all PK3s in gamedata. Every concurrent reader (icon resolution,
  shader resolver, portrait fallback scan) blocked on `vfs.mu.RLock`,
  starving the main thread for the duration of the scan. `Refresh` now
  scans into a staging index without holding the lock and swaps
  atomically — concurrent readers see either the old or the new index,
  never block.
- **Earlier round of `fyne.Do` defers from main thread caused a different
  deadlock** (queue waited for main, main waited for queue). All those
  wrappers stripped from `openFileFromPath`, `SetAssetBrowser`,
  `updateUI`, and the click handlers in `WeaponGrid` / `HoldableGrid` /
  `WeaponFlagsEditor`. Inline refresh is the right call; the legitimate
  background-goroutine `fyne.Do` calls (HTTP fetches, source-panel
  cross-thread sync, update banner) are untouched.

### Changed
- **Portrait "no image found" label cleaned up** — was rendering as
  `"override · no image found"` next to the placeholder icon. Now blank
  when nothing resolves; the placeholder image carries the meaning.
- **Last-resort portrait fallback** added — when the explicit
  `mb2_icon_<skin>` / `mb2_icon_default` candidates miss, the resolver
  now scans the VFS for any `models/players/<model>/mb2_icon_*` and
  uses the first match. Result is cached per model. Fixes characters
  that ship under non-standard names (e.g. `jedi_zf` only has
  `mb2_icon_legends1.jpg`).
- **Shader resolver** parses `*.shader` files lazily on first lookup
  to map shader names to texture paths, with graceful "not built yet"
  return. Prebuilt off the main thread after VFS refresh completes.

### Added
- **Help → Icon Inventory…** debug window enumerating every embedded
  HUD icon / boxicon at runtime — confirms the asset pipeline is
  working end-to-end. Also reachable via the toolbar's grid icon.
- **Boxicon SVG fallback set** (~22 outline glyphs) — when no MBII HUD
  art exists for an attribute, a thematic boxicon stands in.
- **Click an attribute / weapon icon → pin the sidebar** to that ID's
  docs (sticky context). Hover still routes through the transient
  hover dispatcher.
- **Defer all my fyne.Do experiments**: we'll revisit perf via
  proper background-thread parsing in a later release rather than
  trying to defer-onto-the-main-loop.

### Added
- CI workflow (build + vet + gofmt + test across Linux/macOS/Windows)
- Release workflow (triggered on `v*` tags, builds + publishes binaries)
- Issue templates (bug, feature, definition error) + PR template
- Dependabot config for Go modules and GitHub Actions
- Self-contained parser round-trip tests
- README install badges + "Where do I find .mbch files to edit?" section

## [0.11.9-alpha] — 2026-04-26

### Fixed
- **(i) info button on attribute rows now updates the Context sidebar.**
  Was wired through the hover dispatcher, which the default-OFF hover toggle
  silently swallowed. Click now routes through the sticky-context path, so
  the sidebar pins the attribute's docs regardless of hover state.

### Changed
- **Expanded `attributeIconAliases` and `forceIconAliases`** so far more
  attribute rows pick up the embedded MBII HUD PNGs we already ship —
  health/armour/block regen rates, force-pool / battery, cloak, binoculars,
  shield, seeker, sentry gun, eweb, bacta, PSD, repeater, flechette, micro
  grenades, UGL/MGL variants, det pack / trip mines / sticky bombs,
  flamethrower, beskar, plus FP_REPULSE / FP_PROJECTION / FP_BATTLEMED /
  FP_DOMINATION / FP_ATTUNEMENT / FP_STASIS / FP_DARKRAGE.

## [0.11.8-alpha] — 2026-04-26

### Fixed
- **Attributes that aren't yet on the loadout couldn't be turned on** — the
  level-pill widget was hiding the 1/2/3 buttons whenever `CurrentVal == 0`,
  leaving only the "Off" pill clickable. With nothing to click, new attributes
  were unaddable. All pills now stay visible; the active level gets
  HighImportance for emphasis.
- **Save → AppData\Local\Temp\…\.txt** — `applyEdits` was capturing the
  editor's "original path" *after* `LoadFile` had already mutated it to point
  at the parser-roundtrip temp file. The "restore" no-op'd, the editor
  stayed pointed at `…\Temp\foundry-apply-XXXX.txt`, and Save wrote back
  there. Original path is now snapshotted *before* `LoadFile`. Belt-and-
  suspenders: `saveFile` now treats any path under the OS temp dir as
  unset and routes through Save As.

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
