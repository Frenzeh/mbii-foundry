# MBII Foundry — Architecture Overview

## Unified Tool Design

MBII Foundry (binary: `fa_creator`) is a comprehensive content-creation tool for Movie Battles II.
It provides a unified interface for editing MBII content file types with integrated asset browsing and PK3 packaging.

## Supported File Types

| Extension | Description | Editor Tab |
|-----------|-------------|------------|
| `.mbch` | Character class definitions | Character Editor |
| `.sab` | Lightsaber configurations | Saber Editor |
| `.siege` | Siege mode class configs | Siege Editor |
| `.mbtc` | Team configurations | Team Editor |

## UI Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  FA Creator                                            [_][□][X] │
├─────────────────────────────────────────────────────────────────┤
│  File   Edit   View   Tools   Package   Help                    │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────┬───────────────────────┐ │
│ │                                     │                       │ │
│ │  ┌─────┬───────┬───────┬──────┐    │   Asset Browser       │ │
│ │  │Char │ Saber │ Siege │ Team │    │   ───────────────     │ │
│ │  └─────┴───────┴───────┴──────┘    │   [PK3 Source ▼]      │ │
│ │                                     │                       │ │
│ │  ┌─────────────────────────────┐   │   📁 models/          │ │
│ │  │  Active Editor Panel        │   │     📁 players/       │ │
│ │  │                             │   │       📁 cultist/     │ │
│ │  │  [Form fields based on      │   │       📁 luke/        │ │
│ │  │   selected file type]       │   │       📁 stormtrooper │ │
│ │  │                             │   │   📁 ext_data/        │ │
│ │  │  - Basic Info               │   │     📁 mb2/           │ │
│ │  │  - Equipment/Properties     │   │       📁 character/   │ │
│ │  │  - Stats                    │   │   📁 sound/           │ │
│ │  │  - Advanced                 │   │                       │ │
│ │  │                             │   ├───────────────────────┤ │
│ │  └─────────────────────────────┘   │   Preview             │ │
│ │                                     │   ┌─────────────────┐ │ │
│ │  ┌─────────────────────────────┐   │   │                 │ │ │
│ │  │  Validation Messages        │   │   │  [3D Preview    │ │ │
│ │  │  ✓ Name valid               │   │   │   or Image]     │ │ │
│ │  │  ⚠ Missing UI shader        │   │   │                 │ │ │
│ │  └─────────────────────────────┘   │   └─────────────────┘ │ │
│ └─────────────────────────────────────┴───────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│  Ready │ File: untitled.mbch │ Modified                         │
└─────────────────────────────────────────────────────────────────┘
```

## Component Structure

### Core Components

```
fa_creator/
├── go_module/
│   ├── main.go              # Application entry, window management
│   ├── app.go               # Main app state, menu handling
│   ├── editor_tabs.go       # Tab container for editors
│   ├── editors/
│   │   ├── base_editor.go   # Common editor interface
│   │   ├── mbch_editor.go   # Character editor
│   │   ├── sab_editor.go    # Saber editor
│   │   ├── siege_editor.go  # Siege editor
│   │   └── team_editor.go   # Team config editor
│   ├── browser/
│   │   ├── asset_browser.go # PK3 asset tree view
│   │   ├── pk3_source.go    # PK3 file handling
│   │   └── preview.go       # Asset preview panel
│   ├── dialogs/
│   │   ├── pk3_build.go     # Package build dialog
│   │   ├── preferences.go   # Settings dialog
│   │   └── about.go         # About dialog
│   └── utils/
│       ├── file_manager.go  # Backup, recent files
│       └── external.go      # External tool launching (MD3View)
├── parsers/                 # Python parsers (for MCP tools)
├── packager/                # PK3 packaging
└── schemas/                 # JSON schemas and enums
```

### Asset Browser Features

1. **PK3 Source Selection**
   - Browse installed PK3 files in gamedata
   - Extract and cache asset listings
   - Support multiple PK3 sources simultaneously

2. **Tree Navigation**
   - Hierarchical folder structure
   - Filter by asset type (models, sounds, textures)
   - Search functionality

3. **Asset Preview**
   - **Models (.glm)**: Launch MD3View or embedded preview
   - **Textures (.tga/.jpg)**: Image preview
   - **Sounds (.wav/.mp3)**: Audio playback
   - **Config files**: Text preview

4. **Drag & Drop**
   - Drag model path to Model field in editor
   - Drag sound to Soundset field
   - Drag texture to UIShader field

### Editor Interconnection

Editors share common data:
- Model selector pulls from Asset Browser
- Saber selector shows available .sab files
- Validation checks cross-reference assets

### External Tool Integration

**MD3View Integration:**
```go
func LaunchMD3View(modelPath string) error {
    // Path to MD3View executable
    md3viewPath := config.GetMD3ViewPath()

    // Launch with model file
    cmd := exec.Command(md3viewPath, modelPath)
    return cmd.Start()
}
```

**Future: Embedded 3D Preview**
Using g3n (Go 3D game engine) or go-gl for native OpenGL rendering
of GLM models directly in the application.

## Data Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   PK3 Files  │────▶│Asset Browser │────▶│   Editors    │
└──────────────┘     └──────────────┘     └──────────────┘
                            │                     │
                            ▼                     ▼
                     ┌──────────────┐     ┌──────────────┐
                     │   MD3View    │     │  Validators  │
                     │   (Preview)  │     └──────────────┘
                     └──────────────┘            │
                                                 ▼
                                          ┌──────────────┐
                                          │ PK3 Builder  │
                                          └──────────────┘
```

## Configuration

```json
{
  "gamedata_path": "/path/to/gamedata",
  "md3view_path": "/path/to/MD3View.exe",
  "recent_files_max": 20,
  "backup_count": 5,
  "theme": "dark",
  "default_author": "Your Name"
}
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Ctrl+N | New file |
| Ctrl+O | Open file |
| Ctrl+S | Save |
| Ctrl+Shift+S | Save As |
| Ctrl+E | Export JSON |
| Ctrl+B | Build PK3 |
| Ctrl+1-4 | Switch editor tabs |
| F5 | Validate |
| F6 | Preview model |
