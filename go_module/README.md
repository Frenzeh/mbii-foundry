# FA Creator

```
 ╔════════════════════════════════════════════════════════════════════════════════╗
 ║                                                                                ║
 ║  ███████╗ █████╗      ██████╗██████╗ ███████╗ █████╗ ████████╗ ██████╗ ██████╗ ║
 ║  ██╔════╝██╔══██╗    ██╔════╝██╔══██╗██╔════╝██╔══██╗╚══██╔══╝██╔═══██╗██╔══██╗║
 ║  █████╗  ███████║    ██║     ██████╔╝█████╗  ███████║   ██║   ██║   ██║██████╔╝║
 ║  ██╔══╝  ██╔══██║    ██║     ██╔══██╗██╔══╝  ██╔══██║   ██║   ██║   ██║██╔══██╗║
 ║  ██║     ██║  ██║    ╚██████╗██║  ██║███████╗██║  ██║   ██║   ╚██████╔╝██║  ██║║
 ║  ╚═╝     ╚═╝  ╚═╝     ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝║
 ║                                                                                ║
 ║        Full Authentic Content Editor for Movie Battles II                      ║
 ║                                                                                ║
 ╚════════════════════════════════════════════════════════════════════════════════╝
```

> *"This is where the fun begins."* — Anakin Skywalker

A comprehensive GUI editor for creating and editing MBII (Movie Battles II) Full Authentic content files.

**Repository:** [github.com/Frenzeh/mbii-holocron](https://github.com/Frenzeh/mbii-holocron)
**Location:** `system/fa_creator/go_module/`
**Part of:** MBII Holocron - AI-powered development assistant for Movie Battles II

## Overview

FA Creator is a cross-platform desktop application built with Go and [Fyne](https://fyne.io/) that provides visual editors for MBII's custom file formats:

- **Character files** (`.mbch`) - Class definitions for Full Authentic mode
- **Saber files** (`.sab`) - Lightsaber configurations with 100+ properties
- **Vehicle files** (`.veh`) - Vehicle definitions with 150+ properties
- **Siege files** (`.siege`) - Siege mode class configs (uses character format)

## Features

### Multi-Format Editor
- Tabbed interface for editing multiple file types
- Context-sensitive menus and toolbar
- Auto-detection of file type on open

### Character Editor (.mbch)
- Class selection from all MBII classes
- Weapon and attribute configuration
- Force power assignments
- Saber style and color settings
- Stat multipliers (AP, BP, CS, AS)
- Custom build point system support
- Class limits and respawn settings

### Saber Editor (.sab)
- 2 active saber types (SABER_SINGLE, SABER_STAFF)
- Up to 8 blades with individual color/length/radius
- Combat bonuses (lock, parry, break parry, disarm)
- Speed and damage modifiers
- 25 behavior flags (movement restrictions, blade effects)
- Sound configuration (on/off/loop/swing/hit)
- Visual effect paths

### Vehicle Editor (.veh)
- 7 vehicle types (speeder, fighter, walker, animal, etc.)
- Speed and handling configuration
- Durability settings (armor, shields, mass)
- Dual weapon systems with ammo
- Camera overrides (range, FOV, offset)
- Explosion and fuel settings
- MBII-specific properties (VehicleScale, RamDamage)

### Asset Browser
- Browse PK3 file contents
- Filter by asset type (models, sounds, effects, textures)
- Preview asset paths for easy insertion
- MD3View integration for model viewing

### File Management
- Automatic backup system (keeps last 5 backups per file)
- Recent files list (up to 20 files)
- JSON import/export for all formats
- Validation before save

### Templates
Quick-start templates for common configurations:

**Characters:**
- Jedi, Sith, Soldier, Bounty Hunter, Mandalorian

**Sabers:**
- Single Blade, Staff (Double-Bladed), Darksaber

**Vehicles:**
- Speeder, Starfighter, Walker, Animal/Mount

## Building

### Requirements
- Go 1.21 or later
- Fyne dependencies (see [Fyne Getting Started](https://developer.fyne.io/started/))

### macOS
```bash
# Install dependencies (first time only)
brew install pkg-config

# Build
go build -o fa_creator

# Run
./fa_creator
```

### Windows
```powershell
# Build
go build -o fa_creator.exe

# Run
.\fa_creator.exe
```

### Linux
```bash
# Install dependencies (Debian/Ubuntu)
sudo apt-get install gcc libgl1-mesa-dev xorg-dev

# Build
go build -o fa_creator

# Run
./fa_creator
```

## Usage

### Opening Files
- **File → Open** or toolbar Open button
- Drag and drop files onto the window
- Double-click recent files in File menu
- Files are auto-routed to the correct editor based on extension

### Creating New Files
- **File → New** creates a blank document in the current editor tab
- **Templates** menu provides pre-configured starting points
- Switch tabs before clicking New to create that file type

### Saving Files
- **File → Save** (Ctrl+S) saves to current path
- **File → Save As** prompts for new location
- Automatic backup is created before overwriting

### Validation
- **Edit → Validate** or toolbar Validate button
- Checks required fields and value ranges
- Shows detailed error messages

### JSON Export/Import
- **File → Export JSON** saves human-readable JSON
- **File → Import JSON** loads from JSON format
- Useful for version control and scripting

## Configuration

Preferences are stored in `~/.fa_creator_config.json`:

```json
{
  "gamedata_path": "/path/to/gamedata",
  "md3view_path": "/path/to/MD3View",
  "default_author": "Your Name",
  "theme": "dark",
  "last_open_dir": "/last/directory",
  "window_width": 1400,
  "window_height": 900
}
```

### Setting Gamedata Path
1. **View → Set Gamedata Path**
2. Select your MBII gamedata folder
3. Asset browser will populate with PK3 contents

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Ctrl+N | New |
| Ctrl+O | Open |
| Ctrl+S | Save |
| Ctrl+Shift+S | Save As |
| F5 | Validate |
| Ctrl+1-4 | Switch Editor Tabs |

## File Format Reference

### .mbch Structure
```
ClassInfo
{
    name            "_H_MyJedi"
    MBClass         MB_CLASS_JEDI
    model           "cultist"
    skin            "default"
    weapons         WP_SABER|WP_MELEE
    saberstyle      SS_FAST|SS_MEDIUM|SS_STRONG
    maxhealth       100
    forcepool       100
    // ... more properties
}

description "Character description text"
```

### .sab Structure
```
my_saber
{
    name            "My Custom Saber"
    saberType       SABER_SINGLE
    saberModel      "models/weapons2/saber/saber_w.glm"
    saberColor      blue
    saberLength     32.0
    lockBonus       2
    parryBonus      1
    // ... more properties
}
```

### .veh Structure
```
my_speeder
{
    type            VH_SPEEDER
    model           "models/players/vehicle/swoop.glm"
    speedMax        600
    acceleration    20
    armor           100
    weap1           WP_BLASTER
    // ... more properties
}
```

## Project Structure

```
go_module/
├── main.go              # Application entry, window setup
├── common.go            # Shared types, MBCHCharacter struct
├── logger.go            # Logging utilities
├── definitions.go       # Enum definitions loader
├── validation.go        # Validation framework
├── mbch_editor.go       # Character editor (main)
├── mbch_pointbuy.go     # Point buy UI component
├── mbch_weaponinfo.go   # Weapon override editor
├── sab_editor.go        # Saber editor
├── veh_editor.go        # Vehicle editor
├── siege_editor.go      # Siege mode editor
├── bulk_editor.go       # Bulk team editor
├── modpack_manager.go   # Modpack management
├── info_panel.go        # Information panel
├── asset_browser.go     # PK3 asset browser
├── go.mod / go.sum      # Go module files
└── README.md            # This file
```

## Related Components

FA Creator is part of the MBII Holocron project:

- **Python Parsers** (`../parsers/`) - Parsing libraries for all formats
- **MCP Tools** (`../../mcp_server/`) - AI-accessible tools for automation
- **PK3 Packager** (`../packager/`) - Asset packaging utilities

## Known Limitations

- Multi-blade UI only shows first blade (others stored/loaded correctly)
- Turret configuration not exposed in vehicle UI
- No undo/redo (use backups)
- PK3 write/packaging through MCP tools only

## License

Part of the MBII Holocron project.

## AI/LLM Development Notes

**IMPORTANT:** When using AI assistants (Claude, Gemini, etc.) to modify Go source files in this project, be aware of a critical limitation:

### The Backslash Escape Problem

AI tool file-writing operations (Edit, Write, write_file, etc.) consistently corrupt Go string literals containing backslash escape sequences, particularly in regex patterns.

**Symptoms:**
- Double backslashes become single (or vice versa)
- Escaped quotes become malformed
- Newlines inserted inside string literals
- Keywords like `switch` become `sswitch`

**Solution:** Use Python via Bash to modify Go files:

```bash
python3 << 'PYEOF'
path = "path/to/file.go"
with open(path, 'r') as f:
    content = f.read()

# Make replacements
content = content.replace('old_string', 'new_string')

with open(path, 'w') as f:
    f.write(content)
PYEOF
```

This bypasses the AI tool string processing and writes bytes directly.

**Valid Go regex patterns (for reference):**
- Double-quoted: `regexp.MustCompile("(\\w+)\\s+")`
- Raw strings: `regexp.MustCompile(\x60(\\w+)\\s+\x60)`


## Version History

### v2.1 / v1.2 (December 2024)
- Fixed SAB editor typos (SABER_STAUR → SABER_STAFF, lungeatkmov → lungeatkmove)
- Cleaned SaberTypes to active only (SABER_SINGLE, SABER_STAFF)
- Added missing updateUI/updateSaberFromUI methods to SAB editor
- Added ValidateSaber method for saber validation
- Fixed container import in siege/veh editors
- Removed unused imports across all Go files
- Strategic roadmap update for professional tooling

### v2.0 (December 2024)
- Complete rewrite with modular architecture
- Added Siege Editor, Bulk Editor, Modpack Manager
- Point Buy system with custom skills
- Weapon Override (WeaponInfo) editor
- Information panel component
- Validation framework
- Fixed regex parsing for all editors

### v1.1
- Added Saber Editor with 100+ properties
- Added Vehicle Editor with 150+ properties
- Indexed blade properties (saberColor1-8, etc.)
- Context-sensitive file operations
- Templates for all file types

### v1.0
- Initial release with Character Editor
- Asset Browser
- Backup system
- JSON import/export

## Development Roadmap

See `../docs/ROADMAP.md` for detailed roadmap. Current priorities:

1. **Phase 1 (Safety)**: Undo/Redo system, AST-based parsing
2. **Phase 2 (Workflow)**: One-click Build & Run, Log Watcher
3. **Phase 3+**: Bidirectional editing, Visual polish
