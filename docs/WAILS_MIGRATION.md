# Wails Migration Plan

## Overview

Migrating MBII Foundry from Fyne (Go-native GUI) to Wails (Go backend + Web frontend) to enable richer visual design with WebGL effects, custom fonts, and modern CSS styling. **Status: on hold** until Fyne-version UX features stabilize.

---

## Phase 1: Setup & Scaffolding

### 1.1 Install Wails CLI
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor  # Verify installation
```

### 1.2 Create New Wails Project
```bash
cd /path/to/mbii-foundry
wails init -n wails_app -t vanilla  # or svelte/react/vue
```

### 1.3 Project Structure
```
wails_app/
├── main.go                 # Wails bootstrap
├── app.go                  # Backend API (exposed to JS)
├── fa_parser.go            # ← Copy from go_module
├── icon_patterns.go        # ← Copy from go_module
├── icon_loader.go          # ← Copy from go_module (modify for Wails)
├── enum_docs.go            # ← Copy from go_module
├── schemas.go              # ← Copy from go_module
├── wails.json              # Wails config
├── build/                  # Build artifacts
└── frontend/
    ├── index.html
    ├── src/
    │   ├── main.js         # Entry point
    │   ├── style.css       # Global styles
    │   └── components/     # UI components
    ├── assets/
    │   ├── fonts/          # Futura, Hack
    │   └── images/
    └── wailsjs/            # Auto-generated Go bindings
```

---

## Phase 2: Backend Migration

### 2.1 Files to Copy (No Changes Needed)
- `fa_parser.go` - FA file parsing logic
- `enum_docs.go` - Documentation schema
- `icon_patterns.go` - Icon path resolution
- `schemas.go` - JSON schema definitions

### 2.2 Files to Modify

#### icon_loader.go
Remove Fyne dependencies, return raw bytes instead of Fyne resources:
```go
// Before (Fyne)
func (l *IconLoader) GetIcon(enumID string) *canvas.Image

// After (Wails)
func (l *IconLoader) GetIconPNG(enumID string) ([]byte, error)
// Frontend displays via: <img src="data:image/png;base64,{base64data}">
```

### 2.3 Create app.go (API Layer)
```go
package main

import (
    "context"
    "encoding/base64"
)

type App struct {
    ctx        context.Context
    iconLoader *IconLoader
    config     *Config
}

func NewApp() *App {
    return &App{}
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
    a.config = LoadConfig()
    if a.config.GamedataPath != "" {
        a.iconLoader = NewIconLoader(a.config.GamedataPath)
    }
}

// ═══════════════════════════════════════════════════════════════
// File Operations
// ═══════════════════════════════════════════════════════════════

func (a *App) OpenFileDialog(title string, filters []string) (string, error) {
    return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
        Title: title,
        Filters: []runtime.FileFilter{{
            DisplayName: "FA Files",
            Pattern:     "*.mbch;*.sab;*.veh",
        }},
    })
}

func (a *App) LoadFAFile(path string) (*FAFile, error) {
    return ParseFAFile(path)
}

func (a *App) SaveFAFile(path string, data *FAFile) error {
    return WriteFAFile(path, data)
}

// ═══════════════════════════════════════════════════════════════
// Documentation
// ═══════════════════════════════════════════════════════════════

func (a *App) GetEnumDoc(enumID string) *EnumDoc {
    return GetEnumDoc(enumID)
}

func (a *App) GetEnumsByCategory(category string) []EnumDoc {
    return GetEnumsByCategory(category)
}

func (a *App) GetAllCategories() []string {
    return GetEnumCategories()
}

func (a *App) SearchEnums(query string) []EnumDoc {
    return SearchEnums(query)
}

// ═══════════════════════════════════════════════════════════════
// Icons
// ═══════════════════════════════════════════════════════════════

func (a *App) GetIconBase64(enumID string) string {
    if a.iconLoader == nil {
        return ""
    }
    data := a.iconLoader.GetIconPNG(enumID)
    if data == nil {
        return ""
    }
    return base64.StdEncoding.EncodeToString(data)
}

// ═══════════════════════════════════════════════════════════════
// Config
// ═══════════════════════════════════════════════════════════════

func (a *App) GetConfig() *Config {
    return a.config
}

func (a *App) SetGamedataPath(path string) error {
    a.config.GamedataPath = path
    a.iconLoader = NewIconLoader(path)
    return SaveConfig(a.config)
}
```

### 2.4 Update main.go
```go
package main

import (
    "embed"
    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/options"
    "github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
    app := NewApp()

    err := wails.Run(&options.App{
        Title:  "FA Creator",
        Width:  1200,
        Height: 800,
        AssetServer: &assetserver.Options{
            Assets: assets,
        },
        BackgroundColour: &options.RGBA{R: 20, G: 20, B: 25, A: 1},
        OnStartup:        app.startup,
        Bind: []interface{}{
            app,
        },
    })
    if err != nil {
        println("Error:", err.Error())
    }
}
```

---

## Phase 3: Frontend Development

### 3.1 Design System

#### Colors (CSS Variables)
```css
:root {
    /* Foundry Theme */
    --bg-primary: #14141a;
    --bg-secondary: #1a1a22;
    --bg-tertiary: #22222c;

    --accent-primary: #ffaa32;    /* Amber */
    --accent-secondary: #ff8c00;  /* Orange */
    --accent-glow: rgba(255, 170, 50, 0.3);

    --text-primary: #ffffff;
    --text-secondary: #a0a0a0;
    --text-muted: #606060;

    --border-color: #333340;
    --border-glow: rgba(255, 170, 50, 0.5);

    /* Status */
    --success: #4ade80;
    --warning: #facc15;
    --error: #f87171;
}
```

#### Typography
```css
@font-face {
    font-family: 'Futura';
    src: url('/assets/fonts/Futura.woff2') format('woff2');
}

@font-face {
    font-family: 'Hack';
    src: url('/assets/fonts/Hack-Regular.woff2') format('woff2');
}

:root {
    --font-display: 'Futura', sans-serif;
    --font-mono: 'Hack', monospace;
    --font-body: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}
```

### 3.2 Component Structure

```
frontend/src/components/
├── layout/
│   ├── Sidebar.js          # Navigation
│   ├── Header.js           # Title bar
│   └── StatusBar.js        # Bottom info bar
├── editor/
│   ├── AttributeGrid.js    # Toggle grid for attributes
│   ├── WeaponSelector.js   # Weapon picker with icons
│   ├── ForcePanel.js       # Force powers with levels
│   ├── ClassDropdown.js    # Class selector with preview
│   └── PropertyEditor.js   # Generic field editor
├── shared/
│   ├── IconButton.js       # Button with game icon
│   ├── LevelSelector.js    # 1-2-3 level picker
│   ├── Tooltip.js          # Rich tooltip popup
│   └── GlowCard.js         # Card with glow effect
└── effects/
    └── ParticleBackground.js  # WebGL particles (optional)
```

### 3.3 Calling Go from JavaScript
```javascript
// Wails auto-generates bindings in frontend/wailsjs/go/main/App.js

import { LoadFAFile, GetEnumDoc, GetIconBase64 } from '../wailsjs/go/main/App';

// Load a file
const faData = await LoadFAFile('/path/to/file.mbch');

// Get documentation
const doc = await GetEnumDoc('MB_ATT_PUSH');
console.log(doc.Name);        // "Force Push"
console.log(doc.Levels[3]);   // Level 3 details

// Get icon as base64
const iconData = await GetIconBase64('MB_ATT_PUSH');
const imgSrc = `data:image/png;base64,${iconData}`;
```

---

## Phase 4: Feature Parity Checklist

### Must Have (MVP)
- [ ] Open/Save FA files (.mbch, .sab, .veh)
- [ ] Gamedata path configuration
- [ ] Attribute editing with toggle grid
- [ ] Weapon selection
- [ ] Force power selection with levels
- [ ] Class selection
- [ ] Basic field editing (text, numbers)
- [ ] Icons loaded from PK3 files
- [ ] Tooltips with enum documentation

### Nice to Have
- [ ] Search/filter enums
- [ ] Undo/redo
- [ ] Multiple file tabs
- [ ] Diff view (compare files)
- [ ] Template presets
- [ ] WebGL background effects
- [ ] Keyboard shortcuts

---

## Phase 5: Build & Distribution

### Development
```bash
cd wails_app
wails dev  # Hot reload mode
```

### Production Builds
```bash
# macOS
wails build -platform darwin/universal

# Windows
wails build -platform windows/amd64

# Linux
wails build -platform linux/amd64
```

### Output Locations
- macOS: `build/bin/FA Creator.app`
- Windows: `build/bin/FA Creator.exe`
- Linux: `build/bin/fa_creator`

---

## Timeline Estimate

| Phase | Task | Estimate |
|-------|------|----------|
| 1 | Setup & Scaffolding | 30 min |
| 2 | Backend Migration | 1-2 hours |
| 3 | Frontend - Basic UI | 2-3 hours |
| 3 | Frontend - Styling & Polish | 2-3 hours |
| 4 | Testing & Bug Fixes | 1-2 hours |
| 5 | Build & Package | 30 min |
| **Total** | | **~8-12 hours** |

---

## Migration Order

1. **Setup** - Create Wails project, verify builds
2. **Backend** - Copy Go files, create app.go API
3. **Basic Frontend** - File open/save, simple display
4. **Attribute Editor** - Toggle grid with icons
5. **Styling** - Apply design system, fonts, glows
6. **Polish** - Tooltips, animations, effects
7. **Test** - Cross-platform builds, edge cases
