# MBII Foundry — Project Instructions (for AI assistants)

> Source of truth for project rules, conventions, and architecture.
> Despite the name, this document is AI-agnostic — Gemini, Copilot, and other assistants can read it directly. `GEMINI.md` is a short redirect stub pointing back here.

---

## Project Overview

MBII Foundry is a **standalone visual editor** for Movie Battles II (MBII) content files: `.mbch` (character/class), `.sab` (saber), `.veh` (vehicle), and `.siege` configs. It lets users create and edit those files without knowing the underlying text-file syntax.

### Key Design Principles

1. **Standalone Tool** — works independently of any backend or AI service. Users run the binary and that's it.
2. **New User Friendly** — target audience is content creators, balance designers, and players
3. **Error Prevention** - Eliminate typos, missing brackets, quotation errors
4. **Visual & Intuitive** - Toggle buttons, icons, and clear labels instead of raw text
5. **Large File Handling** - Must handle massive .mbch files with 100+ attributes

### Primary Goals

- Make editing complex .mbch files **easy and visual**
- **Prevent syntax errors** that break FA files (brackets, quotes, pipes)
- Show **what each option does** with rich documentation
- Use **actual game icons** so users recognize what they're editing
- Generate **valid, clean FA files** every time

**Current Status**: Functional Fyne app. Wails migration PLANNED but ON HOLD.

**Current Focus**: Complete toggle-based editing widgets and expand documentation.

---

## Directory Structure

```
mbii-foundry/
├── CLAUDE.md              # THIS FILE - source of truth
├── GEMINI.md              # Stub: redirects to CLAUDE.md
├── README.md              # User-facing readme
├── build_app.sh           # macOS app bundle build script
│
├── docs/                  # All documentation
│   ├── ROADMAP.md         # Feature roadmap with priorities
│   ├── WAILS_MIGRATION.md # Migration plan (ON HOLD)
│   ├── UX_IMPROVEMENT_PLAN.md
│   ├── ARCHITECTURE.md
│   └── ASSET_VIEWER_PLAN.md
│
├── go_module/             # Current Fyne-based application
│   ├── main.go            # App entry, window setup
│   ├── fa_parser.go       # FA file parsing (read/write)
│   ├── icon_patterns.go   # Enum → icon path mapping
│   ├── icon_loader.go     # PK3 extraction, TGA→PNG
│   ├── enum_docs.go       # Rich documentation for enums
│   ├── schemas.go         # JSON schemas for validation
│   ├── theme.go           # Foundry theme (amber/orange sci-fi)
│   ├── mbch_editor.go     # Character file editor
│   ├── sab_editor.go      # Saber file editor
│   ├── veh_editor.go      # Vehicle file editor
│   ├── attribute_selector.go  # Toggle grid widget (WIP)
│   └── [other editors]
│
└── wails_app/             # FUTURE: Wails-based app (not created yet)
```

---

## Current Priorities

See `docs/ROADMAP.md` for full roadmap. Current focus:

1. **P1: Toggle Grid Widget** - Visual attribute selection
2. **P1: Icon Integration** - Show game icons in editor
3. **P1: Rich Tooltips** - Per-level documentation on hover
4. **P2: Expand Documentation** - Cover all ~150+ enums

**Wails migration is ON HOLD** until core UX features are validated in Fyne.

---

## Build Commands

```bash
# Build Go binary
cd go_module
go build -o mbii-foundry

# Run directly
./mbii-foundry

# Build macOS .app bundle (with code signing)
cd ..
./build_app.sh

# Output: "MBII Foundry.app" in parent directory
```

**Important**: macOS requires code signing. The build script runs:
```bash
codesign -s - -f --deep "$APP_BUNDLE"
```

---

## Technical Architecture

### Icon Pattern System
Icons are resolved from enum IDs to asset paths in PK3 files:

| Category | Pattern | Example |
|----------|---------|---------|
| Classes | `gfx/menus/classes/{short}` | `gfx/menus/classes/jedi` |
| Weapons | `gfx/hud/w_icon_{name}` | `gfx/hud/w_icon_blaster` |
| Skills | `gfx/hud/skill_{name}` | `gfx/hud/skill_push` |
| Items | `gfx/hud/i_icon_{name}` | `gfx/hud/i_icon_bacta` |

**Files**: `icon_patterns.go` (mapping), `icon_loader.go` (extraction)

### Documentation Schema
Every enum should have documentation in `enum_docs.go`:

```go
EnumDoc{
    ID:          "MB_ATT_PUSH",
    Name:        "Force Push",
    Category:    "Force Powers",
    Description: "Short description",
    Overview:    "Detailed paragraph",
    MaxLevel:    3,
    Levels: map[int]LevelDoc{
        1: {Name: "Basic", Effect: "...", FPCost: 20},
        2: {Name: "Strong", Effect: "...", FPCost: 30},
        3: {Name: "Master", Effect: "...", FPCost: 50},
    },
    Related: []string{"MB_ATT_PULL"},
    Tags:    []string{"force", "light_side"},
}
```

### FA File Format
```
MBCH
{
    MBClass,MB_CLASS_JEDI
    MBAttributes,MB_ATT_PUSH,3|MB_ATT_PULL,2|MB_ATT_SABER_OFFENSE,3
    MBWeapons,WP_SABER
    MBSaberStance,SS_MEDIUM|SS_STRONG
    MBClassFlags,CFL_BPFREEJUMPS
}
```

**⚠️ CRITICAL: MBCH files have an 8192 character limit.**
- The tool MUST track character count
- Warn users when approaching limit
- Prevent saving files that exceed limit

Key fields:
- `MBClass`: Base class enum
- `MBAttributes`: Pipe-separated `ENUM,LEVEL` pairs
- `MBWeapons`: Available weapons
- `MBClassFlags`: Bitfield flags

---

## Code Guidelines

### Go Code
- Use standard Go formatting (`go fmt`)
- Error handling: return errors, don't panic
- Keep UI code in editor files, logic in parser/loader files
- Comments for exported functions

### Fyne UI
- Use theme colors from `theme.go`
- Prefer `container.NewVBox/HBox` for layout
- Use `widget.NewButtonWithIcon` for icon buttons
- Tooltips via popup (Fyne has no native tooltips)

### Naming
- Go: `PascalCase` exported, `camelCase` internal
- Files: `snake_case.go`
- Enums: Match MBII conventions (`MB_ATT_*`, `WP_*`, `FP_*`)

---

## Common Tasks

### Add Enum Documentation
1. Edit `go_module/enum_docs.go`
2. Add entry to `EnumDocs` map
3. Include all levels for leveled attributes
4. Add tags for searchability

### Add Icon Mapping
1. Edit `go_module/icon_patterns.go`
2. Add to appropriate map:
   - `ClassIconMap` for MB_CLASS_*
   - `WeaponIconMap` for WP_*
   - `SkillIconMap` for FP_*, EAS_*
   - `ItemIconMap` for HI_*
   - `AttributeIconMap` for MB_ATT_*

### Test Icon Loading
1. Ensure gamedata path is configured
2. Run app, check icon display
3. Missing icons show placeholder

### Build and Test
```bash
cd go_module
go build -o mbii-foundry && ./mbii-foundry
```

---

## Known Issues

1. **Fyne tooltip limitation** - Using popup workaround
2. **Large icons crash** - Don't embed large resources in binary
3. **macOS signing required** - Unsigned apps won't launch
4. **TGA transparency** - Some icons need alpha channel handling

---

## Documentation Files

| File | Purpose |
|------|---------|
| `CLAUDE.md` | Source of truth for AI assistants (this file) |
| `GEMINI.md` | Stub: redirects to `CLAUDE.md` |
| `README.md` | User-facing overview + install instructions |
| `CONTRIBUTING.md` | Developer contribution guide |
| `USER_GUIDE.md` | How to use the app (end-user docs) |
| `docs/DEFINITIONS_GUIDE.md` | Editing per-enum prose (`definitions/*.md`) |
| `docs/ROADMAP.md` | Feature roadmap with priorities |
| `docs/ARCHITECTURE.md` | System architecture |
| `docs/UX_IMPROVEMENT_PLAN.md` | UX vision document |
| `docs/WAILS_MIGRATION.md` | Future Wails migration plan (on hold) |
| `docs/ASSET_VIEWER_PLAN.md` | Future asset browser improvements |

---

## Decision Log

| Decision | Rationale | Status |
|----------|-----------|--------|
| Use Fyne initially | Fast Go-native development | Active |
| Pattern-based icons | Scalable, matches game code | Done |
| Per-level docs | FA editing needs level info | Done |
| Wails migration | Better visual capabilities | ON HOLD |

---

## Research & Validation Resources

When documenting enums, verifying behavior, or looking up game mechanics, use these sources:

### Primary Sources
1. **MBII engine source** (from a local clone of the `moviebattles` repo):
   - `game/bg_public.h` — Enum definitions (MB_ATT_*, MB_CLASS_*, SS_*, etc.)
   - `game/bg_weapons.h` — Weapon enum (WP_*)
   - `game/bg_classes.h` — Class definitions
   - `game/bg_misc.c` — Item/weapon data with icons
   - `game/mb_defines.h` — Feature flag state (which enums are live in the current build)

2. **TextAssets** (a local clone of the MBII TextAssets repo):
   - FA file examples (`MBAssets3/ext_data/mb2/character/*.mbch`)
   - Class configurations, game text strings

3. **Gamedata** (user's MBII install):
   - PK3 files containing icons, sounds, models

Paths are user-specific; don't hardcode. Accept a path as input and resolve relative to it.

### External References
4. **Movie Battles II Wiki** - https://moviebattles.fandom.com/wiki/Moviebattles_Wikia
   - Official community documentation
   - Class guides, weapon stats, ability descriptions
   - Use for player-facing descriptions and tips

### Validation Workflow
When adding enum documentation:
1. Check source code for technical accuracy (enum values, costs)
2. Check wiki for player-friendly descriptions
3. Cross-reference with actual FA files in TextAssets
4. Test in-game if behavior is unclear

---

*Last updated: December 2024*
