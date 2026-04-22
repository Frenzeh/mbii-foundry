# Contributing to MBII Foundry

```
        ╔════════════════════════════════════════════════════════════╗
        ║                                                            ║
        ║   ░░░  ███╗   ███╗██████╗ ██╗██╗    ░░░                    ║
        ║   ░░░  ████╗ ████║██╔══██╗██║██║    ░░░  F  O  U  N        ║
        ║   ░░░  ██╔████╔██║██████╔╝██║██║    ░░░  D  R  Y           ║
        ║   ░░░  ██║╚██╔╝██║██╔══██╗██║██║    ░░░                    ║
        ║   ░░░  ██║ ╚═╝ ██║██████╔╝██║██║    ░░░  A L P H A         ║
        ║   ░░░  ╚═╝     ╚═╝╚═════╝ ╚═╝╚═╝    ░░░                    ║
        ║                                                            ║
        ╚════════════════════════════════════════════════════════════╝
```

> *"Your focus determines your reality."* — Qui-Gon Jinn

Welcome. This guide gets you up to speed on the MBII Foundry codebase (binary still named `fa_creator` — historical) and on the kinds of contributions we're looking for during alpha.

## Quick Start

### Prerequisites
- Go 1.21 or later
- macOS: `brew install pkg-config`
- Linux: `sudo apt-get install gcc libgl1-mesa-dev xorg-dev`
- Windows: Just Go (CGO dependencies handled automatically)

### First Build
```bash
cd system/fa_creator/go_module
go build -o fa_creator
./fa_creator
```

If it compiles and runs, you're ready to contribute!

---

## Project Overview

FA Creator is a visual editor for Movie Battles II (MBII) "Full Authentic" game files. It's built with:

- **Go** - Core application language
- **Fyne** - Cross-platform GUI framework
- **No external DB** - All data is file-based (.mbch, .sab, .veh)

### What It Does
- Opens/saves MBII configuration files
- Provides form-based editing instead of raw text
- Validates files before saving
- Prevents common syntax errors (brackets, quotes, pipes)

### Key File Types
| Extension | Purpose | Editor File |
|-----------|---------|-------------|
| `.mbch` | Character class definitions | `mbch_editor.go` |
| `.sab` | Saber configurations | `sab_editor.go` |
| `.veh` | Vehicle definitions | `veh_editor.go` |
| `.siege` | Siege mode configs | `siege_editor.go` |

---

## Project Structure

```
fa_creator/
├── CLAUDE.md              # AI assistant instructions (source of truth)
├── CONTRIBUTING.md        # This file
├── README.md              # User-facing documentation
├── build_app.sh           # Build script for macOS .app
│
├── docs/
│   ├── ROADMAP.md         # Strategic roadmap (READ THIS!)
│   └── WAILS_MIGRATION.md # Future migration plan
│
└── go_module/             # Main Go application
    ├── main.go            # Entry point, window setup, menus
    ├── common.go          # Shared types (MBCHCharacter, etc.)
    ├── definitions.go     # Enum lists (classes, weapons, etc.)
    ├── validation.go      # Validation logic
    │
    ├── mbch_editor.go     # Character editor (largest file)
    ├── mbch_pointbuy.go   # Point buy UI component
    ├── mbch_weaponinfo.go # Weapon override editor
    │
    ├── sab_editor.go      # Saber editor
    ├── veh_editor.go      # Vehicle editor
    ├── siege_editor.go    # Siege mode editor
    ├── bulk_editor.go     # Bulk team editing
    │
    ├── asset_browser.go   # PK3 file browser
    ├── modpack_manager.go # Modpack packaging
    ├── info_panel.go      # Information display
    ├── syntax_highlighter.go # Source code highlighting
    │
    └── logger.go          # Logging utilities
```

---

## Current Priorities

**Read `docs/ROADMAP.md` for the full strategic plan.**

We are currently focused on:

### Phase 1: Safety (P0/P1)
1. **Undo/Redo System** - Command pattern implementation
2. **AST-based Parsing** - Preserve comments and formatting
3. **Character Limit Tracking** - MBCH files have 8192 char limit

### Phase 2: Workflow (P1)
1. **One-Click Build & Run** - Test your FA files in-game
2. **Log Watcher** - See game errors in the editor

**New editors are FROZEN until Phase 1-2 are complete.**

---

## How to Contribute

### Finding Work
1. Check `docs/ROADMAP.md` for priorities
2. Look for `// TODO:` comments in code
3. Open issues on GitHub
4. Ask in discussions

### Making Changes

1. **Create a branch** from `main`
2. **Make focused changes** - One feature/fix per PR
3. **Test manually** - No automated tests yet
4. **Update docs** if needed
5. **Submit PR** with clear description

### Commit Messages
Follow this format:
```
Type: Short description

- Bullet point details
- Another detail

🤖 Generated with [Claude Code](https://claude.com/claude-code)
```

Types: `Fix`, `Feat`, `Refactor`, `Docs`, `Build`

---

## Code Patterns

### Editor Structure
Each editor follows this pattern:

```go
type XxxEditor struct {
    container   *fyne.Container   // Root UI element
    currentPath string            // Currently open file
    data        *XxxData          // Parsed file data

    // UI widgets (entry fields, checkboxes, etc.)
    nameEntry *widget.Entry
    // ...
}

func NewXxxEditor() *XxxEditor {
    e := &XxxEditor{data: &XxxData{}}
    e.createUI()
    return e
}

func (e *XxxEditor) GetContent() fyne.CanvasObject { return e.container }
func (e *XxxEditor) LoadFile(path string) error { ... }
func (e *XxxEditor) SaveFile(path string) error { ... }
func (e *XxxEditor) GetCurrentPath() string { return e.currentPath }

// Sync UI ↔ Data
func (e *XxxEditor) updateUI() { ... }         // Data → UI
func (e *XxxEditor) updateDataFromUI() { ... } // UI → Data
```

### Adding a New Field

1. Add to the data struct in editor file
2. Add UI widget to `createUI()`
3. Add mapping in `updateUI()` (data → widget)
4. Add mapping in `updateDataFromUI()` (widget → data)
5. Add parsing in `LoadFile()` or `setField()`
6. Add output in `SaveFile()`
7. Add validation if needed

### Parsing Pattern
```go
func (e *XxxEditor) setField(key, value string) {
    switch key {
    case "name":
        e.data.Name = value
    case "speed":
        e.data.Speed, _ = strconv.Atoi(value)
    // ... more cases
    default:
        e.data.ExtraFields[key] = value  // Preserve unknown fields
    }
}
```

---

## AI Assistant Notes

### Working with AI (Claude, Gemini, etc.)

**IMPORTANT: Backslash Escape Problem**

AI file-writing tools corrupt Go regex patterns. The backslashes get mangled.

**Symptom:** Build fails with regex errors after AI edit.

**Solution:** Use Python via Bash for regex-containing edits:

```bash
python3 << 'PYEOF'
path = "path/to/file.go"
with open(path, 'r') as f:
    content = f.read()
content = content.replace('old_string', 'new_string')
with open(path, 'w') as f:
    f.write(content)
PYEOF
```

### Key Documentation
- **CLAUDE.md** - Instructions for Claude AI
- **GEMINI.md** - Onboarding for other AIs
- Both point to this file for human developers

---

## Testing

### Manual Testing Checklist
Before submitting a PR:

- [ ] App compiles: `go build -o fa_creator`
- [ ] App launches without errors
- [ ] Can open existing file
- [ ] Can edit and save file
- [ ] Saved file loads correctly
- [ ] Validate function works
- [ ] No regressions in other editors

### Testing Commands
```bash
# Build
cd go_module
go build -o fa_creator

# Run
./fa_creator

# Check for compile errors quickly
go vet ./...
```

---

## Build & Release

### Local Build
```bash
./build_app.sh  # Creates "FA Creator.app"
```

### CI/CD
GitHub Actions handle:
- **Push to main**: Build all platforms, upload artifacts
- **Push tag v***: Build + create GitHub Release

See `.github/workflows/` for details.

---

## Common Gotchas

### 1. MBCH Character Limit
Files over 8192 characters silently break. We need to add tracking (Phase 1).

### 2. Parser is Lossy
Current regex parser loses comments. AST-based parser is planned (Phase 1).

### 3. No Undo/Redo
Users can't undo mistakes. High priority fix (Phase 1).

### 4. Multi-Blade UI
Only blade 1 is editable in UI, but all blades are stored correctly.

### 5. Fyne Tooltips
Fyne has no native tooltips. We use popup workarounds.

---

## Getting Help

- **Issues**: GitHub Issues for bugs/features
- **Discussions**: GitHub Discussions for questions
- **Docs**: Start with `ROADMAP.md`, then `CLAUDE.md`

---

## License

See the root LICENSE file for terms. Part of the broader MBII modding ecosystem.

---

```
    ╔═══════════════════════════════════════════════════════════════════╗
    ║  May the Force be with you, Developer.                            ║
    ║                                                                   ║
    ║  "Do. Or do not. There is no try." — Yoda                         ║
    ╚═══════════════════════════════════════════════════════════════════╝
```

*Last updated: December 2024*
