# MBII Foundry — Gemini Onboarding

> This document is the onboarding context for Gemini and other AI assistants. **CLAUDE.md** is the source of truth for project rules and conventions — always defer to it for coding standards, architecture decisions, and workflows.
>
> The binary is still named `fa_creator` for historical reasons; the app/repo brand is "MBII Foundry."

---

## Quick Context

MBII Foundry is a **standalone visual editor** for Movie Battles II (MBII) content files: `.mbch` (character/class), `.sab` (saber), `.veh` (vehicle), `.siege` (siege config). It replaces manual text editing with an intuitive GUI.

**Current State**: Functional Fyne-based app; Wails migration planned but on hold until core UX is settled.

---

## CRITICAL: Design Philosophy

### This is a STANDALONE tool
- **No backend required** — users run one binary and that's it. Don't introduce runtime dependencies on external servers or AI services.
- **Target audience: players, content creators, balance designers** — assume no programming background.
- **Optional AI hints** via a local backend are a bonus feature, not a requirement. Core functionality must work fully offline.

### Primary Problem Being Solved
FA files are prone to human error when edited manually:
```
MBAttributes,MB_ATT_PUSH,3|MB_ATT_PULL,2|MB_ATT_SABER_OFFENSE,3
```
- Typos in enum names break the file
- Missing pipes, commas, or brackets cause silent failures
- Users don't know what `MB_ATT_PUSH,3` means vs level 1 or 2
- Large files with 100+ attributes are overwhelming
- **MBCH files have an 8192 character limit** - easy to exceed without realizing

### Solution: Visual Editing
Instead of typing `MB_ATT_PUSH,3`, users:
1. See "Force Push" with its game icon
2. Toggle it ON/OFF with a checkbox
3. Select level 1/2/3 with buttons
4. Hover for "Level 3: Force Wave - 360° push, knockdown, 50 FP"

**The tool generates valid syntax automatically. Users never see raw text.**

---

## Key Files to Read First

1. **CLAUDE.md** - Project rules, conventions, build commands (SOURCE OF TRUTH)
2. **docs/WAILS_MIGRATION.md** - Migration plan from Fyne to Wails
3. **docs/UX_IMPROVEMENT_PLAN.md** - Vision for intuitive editing interface
4. **docs/ARCHITECTURE.md** - System architecture overview

---

## Current Project State

### What's Done ✅
- [x] FA file parser (`go_module/fa_parser.go`) - reads/writes MBCH, SAB, VEH
- [x] Icon pattern system (`go_module/icon_patterns.go`) - maps enums to game asset paths
- [x] PK3 icon loader (`go_module/icon_loader.go`) - extracts TGA/PNG from game archives
- [x] Rich documentation schema (`go_module/enum_docs.go`) - per-level docs for enums
- [x] Basic Fyne editors for MBCH, SAB, VEH files
- [x] Foundry theme (amber/orange sci-fi colors)
- [x] macOS app bundle with code signing
- [x] Gamedata path configuration dialog

### What's Planned 📋
- [ ] **Wails Migration** - Replace Fyne with web frontend (see docs/WAILS_MIGRATION.md)
- [ ] **Toggle Grid Widget** - Visual attribute selection instead of text input
- [ ] **Game Icons in UI** - Display actual MBII icons from PK3 files
- [ ] **Rich Tooltips** - Hover for detailed per-level documentation
- [ ] **Sci-Fi Styling** - Custom fonts (Futura, Hack), glows, WebGL effects
- [ ] **Cross-platform builds** - macOS, Windows, Linux

### Migration Decision: HOLD
The Wails migration is **planned but not started**. Current focus should be on:
1. Completing the toggle-based editing widgets in Fyne first
2. Expanding enum documentation coverage
3. Testing icon loading from various PK3 files

Migration to Wails is approved but waiting until core UX features are validated.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      FA Creator                              │
├─────────────────────────────────────────────────────────────┤
│  UI Layer (Currently Fyne, Future: Wails Web)               │
│  - Editor tabs (MBCH, SAB, VEH)                             │
│  - Attribute selector widgets                                │
│  - Icon display                                              │
├─────────────────────────────────────────────────────────────┤
│  Business Logic (Go)                                         │
│  - fa_parser.go: Parse/write FA files                       │
│  - icon_patterns.go: Enum → asset path mapping              │
│  - icon_loader.go: PK3 extraction, TGA→PNG conversion       │
│  - enum_docs.go: Documentation database                      │
├─────────────────────────────────────────────────────────────┤
│  Data Sources                                                │
│  - MBII PK3 files (icons, assets)                           │
│  - FA files (.mbch, .sab, .veh)                             │
│  - Config (gamedata path, preferences)                       │
└─────────────────────────────────────────────────────────────┘
```

---

## Icon Pattern System

Icons are resolved from enum IDs using patterns discovered from MBII source code:

| Category | Pattern | Example |
|----------|---------|---------|
| Classes | `gfx/menus/classes/{short}` | `gfx/menus/classes/jedi` |
| Weapons | `gfx/hud/w_icon_{name}` | `gfx/hud/w_icon_blaster` |
| Skills | `gfx/hud/skill_{name}` | `gfx/hud/skill_push` |
| Items | `gfx/hud/i_icon_{name}` | `gfx/hud/i_icon_bacta` |

See `icon_patterns.go` for complete mappings.

---

## Documentation Schema

Every enum should have documentation in `enum_docs.go`:

```go
EnumDoc{
    ID:          "MB_ATT_PUSH",
    Name:        "Force Push",
    Category:    "Force Powers",
    Description: "Push enemies away with the Force",
    Overview:    "Detailed paragraph...",
    Alignment:   "light",
    MaxLevel:    3,
    Levels: map[int]LevelDoc{
        1: {Name: "Basic Push", Effect: "...", FPCost: 20, Tip: "..."},
        2: {Name: "Powerful Push", ...},
        3: {Name: "Force Wave", ...},
    },
    Tips:    []string{"Tip 1", "Tip 2"},
    Related: []string{"MB_ATT_PULL"},
    Tags:    []string{"force", "light_side", "knockback"},
}
```

**Current Coverage**: ~15 enums documented (Force powers, basic weapons, equipment)
**Goal**: All ~150+ attributes documented

---

## Build & Run

```bash
# Current (Fyne)
cd go_module
go build -o fa_creator
./fa_creator

# macOS App Bundle
cd ..
./build_app.sh   # Creates "FA Creator.app"

# Future (Wails) - NOT YET IMPLEMENTED
cd wails_app
wails dev        # Development with hot reload
wails build      # Production build
```

---

## Common Tasks

### Add Documentation for an Enum
1. Open `go_module/enum_docs.go`
2. Add entry to `EnumDocs` map following existing pattern
3. Include all levels if it's a leveled attribute
4. Add related enums and search tags

### Add Icon Mapping
1. Open `go_module/icon_patterns.go`
2. Add to appropriate map: `ClassIconMap`, `WeaponIconMap`, `SkillIconMap`, `ItemIconMap`, or `AttributeIconMap`
3. Verify icon exists in MBII PK3 files

### Test Icon Loading
1. Set gamedata path in app preferences
2. Call `GetIconPath(enumID)` to verify path resolution
3. Check PK3 files contain the expected asset

---

## Things to Watch Out For

### Fyne Limitations
- No native tooltips (using popup workaround)
- Limited styling capabilities
- No WebGL/custom shaders
- This is why Wails migration is planned

### macOS Code Signing
- App MUST be signed or it won't launch
- Build script includes: `codesign -s - -f --deep`
- Ad-hoc signing is sufficient for local use

### PK3 File Loading
- PK3 files are ZIP archives
- Icons are usually TGA format (need conversion to PNG)
- Some icons have transparency (32-bit TGA)
- Case-insensitive path matching needed

### FA File Format
- Windows line endings (CRLF) in some files
- Attribute format: `ENUM,LEVEL|ENUM,LEVEL|...`
- Some fields are bitfields, some are pipe-separated lists

---

## Questions? Problems?

1. Check **CLAUDE.md** first - it has coding standards and conventions
2. Check **docs/** folder for detailed plans
3. Run `go build` to verify code compiles
4. Test changes with actual FA files from MBII

---

## Handoff Checklist

Before continuing development:
- [ ] Read CLAUDE.md completely
- [ ] Review docs/WAILS_MIGRATION.md if planning migration
- [ ] Verify gamedata path points to valid MBII installation
- [ ] Test that current Fyne app builds and runs
- [ ] Understand icon pattern system in icon_patterns.go

---

## Research & Validation Resources

Use these sources to validate enum documentation and game mechanics:

### Local Development Directories
Paths are user-specific. Accept them as input (env var or CLI arg), don't hardcode.

1. **MBII engine source** (a local clone of the `moviebattles` repo):
   - `game/bg_public.h` — Enum definitions (MB_ATT_*, MB_CLASS_*, SS_*, HI_*)
   - `game/bg_weapons.h` — Weapon enum (WP_*)
   - `game/bg_classes.h` — Class definitions
   - `game/bg_misc.c` — Item/weapon data with icons
   - `game/mb_defines.h` — Feature-flag state for the active build

2. **TextAssets** (a local clone of the MBII TextAssets repo):
   - Real FA file examples (`MBAssets3/ext_data/mb2/character/*.mbch`)
   - Class configurations, strings

3. **Gamedata** (user's MBII install):
   - PK3 files with game icons, sounds, models

### External Reference
4. **Movie Battles II Wiki** - https://moviebattles.fandom.com/wiki/Moviebattles_Wikia
   - Community documentation
   - Player-friendly ability descriptions
   - Class guides and tips

### When Adding Documentation
1. Source code → technical accuracy (enum names, FP costs)
2. Wiki → player-friendly descriptions and tips
3. TextAssets → real-world FA file examples
4. Cross-reference all three before finalizing

---

*Last updated: December 2024 by Claude*
*Next planned work: Complete toggle-based attribute editing widgets*
