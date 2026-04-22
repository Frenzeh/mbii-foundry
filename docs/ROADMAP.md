# MBII Foundry Roadmap

```
     ╔══════════════════════════════════════════════════════════════════╗
     ║                                                                  ║
     ║    ⚔️  THE PATH TO MASTERY  ⚔️                                   ║
     ║                                                                  ║
     ║    "Truly wonderful, the mind of a coder is." — Yoda            ║
     ║                                                                  ║
     ╚══════════════════════════════════════════════════════════════════╝
```

## Current Status: "It Builds" → "It's Essential"

We have successfully "invigorated" the patient - it walks and talks - but it lacks the reflexes and muscle memory of a professional tool.

**Core Mission:** Make FA editing foolproof for new users while giving power users professional-grade tools.

---

## Priority Levels
- **P0** - Critical safety/stability (must fix before release)
- **P1** - High priority, blocks professional use
- **P2** - Medium priority, improves UX significantly
- **P3** - Nice to have, polish

---

## Phase 1: The Critical "Safety Net" (Priority: HIGH)

> Currently, if a user makes a mistake, they are stuck. If the parser fails to read a specific line, that data is deleted upon the next save.

### 1.1 Undo/Redo System (Command Pattern) 🔴 P0

This is the mark of professional software. Every action must be reversible.

**Requirements:**
- [ ] Create generic `Command` interface with `Execute()` and `Undo()` methods
- [ ] Wrap every UI change in a Command object
- [ ] Push commands to a history stack
- [ ] Implement `Ctrl+Z` (Undo) and `Ctrl+Shift+Z` (Redo) shortcuts
- [ ] Show undo history in menu (Edit → Recent Actions)
- [ ] Limit history to ~50 actions to prevent memory bloat

**Implementation Strategy:**
```go
type Command interface {
    Execute()
    Undo()
    Description() string
}

type HistoryManager struct {
    undoStack []Command
    redoStack []Command
}
```

**Files affected:** `mbch_editor.go`, `sab_editor.go`, new `history.go`

### 1.2 Non-Destructive Parsing (AST-based) 🔴 P0

The current Regex parser is "lossy." It loses comments and formatting nuances.

**Requirements:**
- [ ] Write proper Lexer for MBII config format (Quake 3 style keys/values)
- [ ] Parse file into Abstract Syntax Tree (AST)
- [ ] AST preserves comments, whitespace, and original formatting
- [ ] Modify only the changed nodes in AST
- [ ] Write AST back to file, preserving untouched sections
- [ ] Handle multi-line values and quoted strings correctly

**Current Problem:**
```
// User's comment about this character
name "MyJedi"  // inline comment
```
Currently loses both comments. AST-based parsing keeps them.

**Files:** New `parser/lexer.go`, `parser/ast.go`, `parser/writer.go`

### 1.3 Character Limit Tracking 🟠 P1

MBCH files have an 8192 character limit. Exceeding this breaks the file silently.

**Requirements:**
- [ ] Track character count in real-time as user edits
- [ ] Display current count / 8192 in status bar
- [ ] Warn when approaching limit (e.g., >7500 chars)
- [ ] **Block saving** if file exceeds 8192 characters
- [ ] Show which attributes are using the most space

---

## Phase 2: The "Developer Loop" (Priority: HIGH)

> Modders don't just edit; they iterate. Edit → Pack → Test → Repeat.

### 2.1 One-Click Build & Run 🟠 P1

The ModpackManager needs to actually work.

**Requirements:**
- [ ] "Test" button that:
  1. Saves all open files
  2. Zips the folder into a temporary .pk3
  3. Moves it to MBII/ directory
  4. Launches `mbii.x86.exe` with `+devmap` and the specific character loaded
- [ ] Remember last test map (e.g., `mb2_dotf`)
- [ ] Option to launch with specific team/class selected
- [ ] Show build progress/status

**Platform considerations:**
- macOS: Launch via `open -a` or direct binary
- Windows: Launch via `cmd /c start`
- Linux: Launch via `xdg-open` or direct

### 2.2 Log Watcher 🟠 P1

Embed a small terminal in the bottom panel that tails `games.log`.

**Requirements:**
- [ ] Auto-detect MBII log location
- [ ] Tail log file in real-time
- [ ] Filter for errors/warnings
- [ ] Highlight errors about currently edited file in RED
- [ ] Click error to jump to relevant field in editor
- [ ] Collapsible panel (hidden by default)

---

## Phase 3: Bidirectional Source Editing (Priority: MEDIUM)

> Currently, the "Source" tab shows the file read-only. Power users hate UI for small tweaks.

### 3.1 Live Code Editing 🟡 P2

**Requirements:**
- [ ] Make Source tab editable
- [ ] Debounce: When user stops typing (300ms), try to parse text
- [ ] If valid: Update UI forms to match the source
- [ ] If invalid: Show red squiggle on error line, don't update UI
- [ ] Sync both directions: UI → Source and Source → UI
- [ ] Preserve cursor position when syncing

**Files:** New `source_editor.go`, modify `mbch_editor.go`

---

## Phase 4: Visual "Juice" & Polish (Priority: MEDIUM)

> The tool feels like a form-filler. It needs to feel like a design tool.

### 4.1 Asset Browser Gallery View 🟡 P2

The AssetBrowser is currently a list. It needs to be a visual gallery.

**Requirements:**
- [ ] Grid view with thumbnails
- [ ] Model previews (use md3view screenshots or lightweight renderer)
- [ ] Sound preview: Hover to play immediately
- [ ] Texture preview: Show actual TGA/PNG
- [ ] Filter by asset type (models, sounds, textures)
- [ ] Search across asset names

### 4.2 Visual Saber Designer 🟡 P2

The Saber editor has a "Blade" tab. This should draw the saber visually.

**Requirements:**
- [ ] Draw saber blade (width, length, color) on black background
- [ ] Real-time update as user changes values
- [ ] Show glow effect based on blade color
- [ ] Multi-blade preview for staff sabers
- [ ] Export preview as PNG

### 4.3 Icon Integration (Existing) 🟡 P2

Display actual MBII icons in the editor.

**Status:** Icon patterns defined, needs integration into UI widgets.

---

## Phase 5: The "Knowledge" Base (Priority: LOW but Cool)

> The tool relies on hardcoded lists that get outdated.

### 5.1 External Definition Loading 🟢 P3

**Requirements:**
- [ ] Load enums from JSON/YAML file instead of hardcoding
- [ ] Support hot-reload when file changes
- [ ] Version detection: Warn if definitions are outdated
- [ ] Community-contributed definitions

**Example:**
```yaml
# definitions/saber_flags.yaml
saber_flags:
  - name: "forceBlocking"
    description: "Saber blocks blaster bolts automatically"
    default: true
  - name: "notThrowable"
    description: "Cannot use saber throw with this hilt"
    default: false
```

### 5.2 Wiki Integration 🟢 P3

**Requirements:**
- [ ] Press F1 on any field to fetch help from MBII Wiki
- [ ] Cache wiki responses locally
- [ ] Display in Info Panel with formatting
- [ ] Fallback to local docs if offline

---

## Phase 6: Polish & Distribution (Priority: LOW)

### 6.1 Cross-Platform Builds 🟢 P3
- [ ] macOS Intel + Apple Silicon (Universal Binary)
- [ ] Windows 64-bit
- [ ] Linux 64-bit
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Auto-update mechanism

### 6.2 User Features 🟢 P3
- [ ] Multiple file tabs
- [ ] Recent files list (done)
- [ ] Template presets (partial)
- [ ] Diff view (compare files)
- [ ] Keyboard shortcuts
- [ ] Dark/Light theme toggle

---

## Completed ✅

### v1.2 (December 2024)
- [x] Fixed SAB editor typos (SABER_STAUR → SABER_STAFF, lungeatkmov → lungeatkmove)
- [x] Cleaned SaberTypes to active types only (SABER_SINGLE, SABER_STAFF)
- [x] Added missing updateUI/updateSaberFromUI methods to SAB editor
- [x] Added ValidateSaber method for saber validation
- [x] Fixed container import in siege/veh editors
- [x] Removed unused imports across all Go files

### v1.1
- [x] FA file parser (read/write MBCH, SAB, VEH)
- [x] Basic Fyne GUI application
- [x] Foundry theme (amber/orange colors)
- [x] macOS app bundle with code signing
- [x] Gamedata path configuration
- [x] Icon pattern mapping system
- [x] PK3 icon loader with TGA→PNG conversion
- [x] Documentation schema with per-level support
- [x] Initial enum documentation (~15 items)
- [x] Project documentation (CLAUDE.md + stub GEMINI.md)
- [x] Wails migration plan

---

## Technical Debt

- [ ] VEH editor needs comment stripping (like SAB editor)
- [ ] Add multi-blade UI support (currently only blade 1 is editable)
- [ ] Cross-verify all enum lists against MBII source code
- [ ] Add unit tests for FA parser
- [ ] Add integration tests for icon loading
- [ ] Clean up attribute_selector.go (incomplete widget)
- [ ] Remove unused Fyne code after Wails migration
- [ ] Consolidate duplicate schema definitions

---

## Frozen/On Hold

### Wails Migration
> **Status:** ON HOLD - Complete Phase 1-2 first.

Benefits: Full CSS styling, custom fonts, WebGL effects, modern web architecture.
Effort: ~8-12 hours.
See `docs/WAILS_MIGRATION.md` for full plan.

### New Editor Types
> **Status:** FROZEN - Focus on safety and workflow before adding more editors.

No new editor types (Siege, VEH improvements) until Undo/Redo and Build workflow are complete.

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| Dec 2024 | v1.2 - Fix typos, cleanup | Code accuracy for MBII text assets |
| Dec 2024 | Freeze new editors | Focus on safety (Undo) and workflow (Build) |
| Dec 2024 | Prioritize Undo/Redo | Professional software requires reversibility |
| Dec 2024 | AST-based parsing | Preserve user comments and formatting |
| Dec 2024 | Use Fyne initially | Fast Go-native development |
| Dec 2024 | Pattern-based icons | Scalable, matches game code |
| Dec 2024 | Per-level documentation | FA editing needs level-specific info |

---

## Next Steps (Immediate)

**If I were managing this project, I would freeze new "Editors" and focus entirely on Phase 1 (Safety) and Phase 2 (Workflow).**

1. **Refactor for Undo/Redo**: This requires changing how `updateCharacterFromUI` works.
2. **Finish the "Build PK3" button**: Connect the pipes so you can actually play what you create.

**You have built the engine; now you need to build the car.**

---

*Last updated: December 2024*
