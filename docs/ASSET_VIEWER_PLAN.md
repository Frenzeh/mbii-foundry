# MBII Foundry — Asset Viewer Implementation Plan

Based on research of [NetRadiant-Custom](https://github.com/Garux/netradiant-custom) and JKA/MBII directory structure.

## JKA/MBII Directory Structure

Unlike mapping tools that focus on textures and brushwork, MBII Foundry focuses on **player models, skins, icons, and effects**.

### Key Directories

```
PK3 Structure (MBII-specific):
├── models/
│   └── players/                    # PRIMARY - Player models
│       └── <modelname>/
│           ├── model.glm           # Ghoul2 model (primary)
│           ├── *.gla               # Animation file
│           ├── *.MD3               # MD3 parts (some models)
│           ├── model_<skin>.skin   # Skin definitions
│           ├── mb2_icon_<skin>.jpg # Character icons (64x64)
│           ├── *.jpg/*.tga         # Textures
│           └── animation.cfg       # Animation config
│
├── ext_data/
│   ├── mb2/character/              # MBCH files we edit
│   ├── sabers/                     # SAB files we edit
│   └── vehicles/                   # VEH files we edit
│
├── gfx/
│   ├── 2d/                         # UI graphics, crosshairs
│   ├── effects/                    # Effect textures
│   └── hud/                        # HUD elements
│
├── shaders/                        # Shader definitions
│   ├── *.shader                    # Q3 shader format
│   └── *.mtr                       # Material files
│
├── effects/                        # EFX effect definitions
│   ├── Blaster/
│   ├── Saber/
│   └── ...
│
└── sound/                          # Audio (not priority)
```

### Key Differences from NetRadiant

| NetRadiant Focus | MBII Foundry Focus |
|------------------|------------------|
| `textures/` (world brushes) | `models/players/` (characters) |
| Map shaders | Character skins |
| Brush models | GLM/MD3 models |
| Level geometry | Class icons (mb2_icon_*.jpg) |

---

## Feature 1: Model Preview Panel

### Purpose
Display selected GLM/MD3 models with rotation, zoom, and skin switching.

### Implementation Approach

**Option A: Embed MD3View** (Simpler)
- Already have MD3View integration
- Extract model to temp, launch viewer
- Works but external window

**Option B: Native OpenGL Renderer** (Better UX)
- Use Fyne's OpenGL canvas
- Port MD3 loading from NetRadiant's `plugins/md3model/`
- GLM is more complex (Ghoul2) - may need external tool

**Option C: Thumbnail Generation** (Practical)
- Generate preview thumbnails on first scan
- Use existing MD3View to batch-render previews
- Cache as JPG in temp directory

### Recommended: Option C + A
1. Generate thumbnail previews for grid view
2. Double-click launches MD3View for full inspection

### Directory Navigation
```go
// Quick navigation for model browser
var ModelDirectories = []string{
    "models/players",           // Player models
    "models/weapons2",          // Weapon models (for sabers)
    "models/map_objects",       // Props
}
```

---

## Feature 2: Skin Browser

### Purpose
Grid view of available skins for the selected model, showing the icon preview.

### Key Files
- `models/players/<model>/mb2_icon_<skin>.jpg` - Selection icons
- `models/players/<model>/model_<skin>.skin` - Skin mappings

### Implementation

```go
type SkinEntry struct {
    ModelName   string   // e.g., "anakin"
    SkinName    string   // e.g., "default", "mus"
    IconPath    string   // e.g., "models/players/anakin/mb2_icon_mus.jpg"
    SkinFile    string   // e.g., "models/players/anakin/model_mus.skin"
}

// Scan for skins in a model directory
func (ab *AssetBrowser) scanModelSkins(modelPath string) []SkinEntry {
    // Look for mb2_icon_*.jpg files
    // Match with model_*.skin files
    // Return list of available skins
}
```

### UI Layout
```
┌─────────────────────────────────────────┐
│ Model: anakin                      [▼]  │
├─────────────────────────────────────────┤
│ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐    │
│ │ icon │ │ icon │ │ icon │ │ icon │    │
│ │      │ │      │ │      │ │      │    │
│ └──────┘ └──────┘ └──────┘ └──────┘    │
│ default   mus      jedi     dark       │
├─────────────────────────────────────────┤
│ Selected: model_mus.skin               │
│ Path: models/players/anakin            │
│                        [Insert] [Copy] │
└─────────────────────────────────────────┘
```

---

## Feature 3: GFX Preview

### Purpose
Browse and preview GFX assets (icons, effects, HUD elements).

### Key Directories
```go
var GFXDirectories = []string{
    "gfx/2d",           // Crosshairs, UI elements
    "gfx/effects",      // Effect textures
    "gfx/hud",          // HUD graphics
    "gfx/menus",        // Menu graphics
}
```

### Implementation
- Extract TGA/JPG from PK3 on selection
- Display in preview panel using Fyne's image widget
- Support TGA with alpha channel

```go
func (ab *AssetBrowser) previewImage(asset *AssetEntry) fyne.CanvasObject {
    // Extract to temp
    tempPath := ab.extractToTemp(asset)

    // Load image
    img := canvas.NewImageFromFile(tempPath)
    img.FillMode = canvas.ImageFillContain

    return img
}
```

---

## Feature 4: Shader Viewer

### Purpose
Parse and display JKA .shader files with syntax highlighting.

### Shader Format (Q3-style)
```
models/players/anakin/body
{
    qer_editorimage models/players/anakin/body.tga
    {
        map models/players/anakin/body.tga
        rgbGen lightingDiffuse
    }
}
```

### Implementation

```go
type Shader struct {
    Name       string              // Full shader path
    EditorImage string             // qer_editorimage path
    Stages     []ShaderStage       // Rendering stages
    Flags      map[string]string   // surfaceparm, cull, etc.
}

type ShaderStage struct {
    Map        string  // Texture path
    BlendFunc  string  // Blend mode
    RgbGen     string  // RGB generation
    AlphaGen   string  // Alpha generation
}
```

### UI Layout
```
┌─────────────────────────────────────────┐
│ Shader: models/players/anakin/body     │
├─────────────────────────────────────────┤
│ Editor Image: [preview]                 │
│                                         │
│ Stages:                                 │
│   1. map: body.tga                      │
│      rgbGen: lightingDiffuse            │
│                                         │
│ Properties:                             │
│   surfaceparm: flesh                    │
│   cull: twosided                        │
├─────────────────────────────────────────┤
│ Raw Source:                             │
│ ┌─────────────────────────────────────┐ │
│ │ models/players/anakin/body          │ │
│ │ {                                   │ │
│ │     qer_editorimage ...             │ │
│ │ }                                   │ │
│ └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

### Shader Directory Quick Access
```go
var ShaderDirectories = []string{
    "shaders",          // Main shader directory
    "scripts",          // Some shaders here too
}
```

---

## Updated Asset Browser Architecture

### New Filter Categories
```go
const (
    AssetTypeModel     AssetType = "model"
    AssetTypeTexture   AssetType = "texture"
    AssetTypeSound     AssetType = "sound"
    AssetTypeCharacter AssetType = "character"  // MBCH
    AssetTypeSaber     AssetType = "saber"      // SAB
    AssetTypeVehicle   AssetType = "vehicle"    // VEH
    AssetTypeSkin      AssetType = "skin"       // .skin files
    AssetTypeShader    AssetType = "shader"     // .shader files
    AssetTypeIcon      AssetType = "icon"       // mb2_icon_*.jpg
    AssetTypeEffect    AssetType = "effect"     // .efx files
    AssetTypeGFX       AssetType = "gfx"        // gfx/ images
)
```

### Quick Navigation Bookmarks
```go
var QuickNavPaths = map[string][]string{
    "Player Models":  {"models/players"},
    "Weapons":        {"models/weapons2"},
    "Sabers (Data)":  {"ext_data/sabers"},
    "Characters":     {"ext_data/mb2/character"},
    "Vehicles":       {"ext_data/vehicles"},
    "Effects":        {"effects"},
    "GFX":            {"gfx/2d", "gfx/effects", "gfx/hud"},
    "Shaders":        {"shaders", "scripts"},
}
```

---

## Implementation Priority

### Phase 1: Enhanced Asset Browser (Quick Wins)
1. Add icon preview for mb2_icon_*.jpg files
2. Add quick navigation bookmarks
3. Improve filtering (Icons, GFX, Effects categories)
4. TGA/JPG preview in preview panel

### Phase 2: Skin Browser
1. Scan model directories for available skins
2. Grid view of skin icons
3. Click to insert into MBCH editor fields

### Phase 3: Shader Viewer
1. Parse .shader file format
2. Display parsed properties
3. Syntax-highlighted raw view
4. Link to referenced textures

### Phase 4: Model Preview
1. Thumbnail generation system
2. MD3 native rendering (if feasible)
3. GLM via MD3View integration

---

## NetRadiant Code References

Useful files from [netradiant-custom](https://github.com/Garux/netradiant-custom):

| Feature | Source File | Notes |
|---------|-------------|-------|
| Texture Grid | `radiant/texwindow.cpp` | GL grid layout, callback-driven |
| Model Browser | `radiant/modelwindow.cpp` | Grid cells, rotation, auto-fit |
| Image Loading | `radiant/image.cpp` | TGA/JPG/PNG loading |
| MD3 Loader | `plugins/md3model/` | MD3 format parsing |
| PK3 VFS | `plugins/vfspk3/` | Archive abstraction |
| Shader Parser | `plugins/shaders/` | Q3 shader parsing |

---

## Technical Notes

### Image Format Support

JKA/MBII uses primarily **TGA and JPG** formats:
- `.tga` - Truevision TGA (often with alpha channel)
- `.jpg` - JPEG (icons, some textures)
- `.png` - Occasionally used in newer content

**TGA Loading in Go:**
```go
import "github.com/ftrvxmtrx/tga"

func loadTGA(path string) (image.Image, error) {
    f, _ := os.Open(path)
    defer f.Close()
    return tga.Decode(f)
}
```

**Add to go.mod:**
```
require github.com/ftrvxmtrx/tga v0.0.0-20150524081124-bd8e8d5be13a
```

**Universal Image Loader:**
```go
func loadImage(path string) (image.Image, error) {
    ext := strings.ToLower(filepath.Ext(path))

    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    switch ext {
    case ".tga":
        return tga.Decode(f)
    case ".jpg", ".jpeg":
        return jpeg.Decode(f)
    case ".png":
        return png.Decode(f)
    default:
        return nil, fmt.Errorf("unsupported format: %s", ext)
    }
}
```

### Fyne Image Display
```go
img := canvas.NewImageFromFile(path)
img.FillMode = canvas.ImageFillContain
img.SetMinSize(fyne.NewSize(64, 64))
```

### Grid Layout for Icons
```go
grid := container.NewGridWrap(fyne.NewSize(80, 100),
    // icon widgets...
)
```
