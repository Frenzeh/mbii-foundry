package main

import (
	"archive/zip"
	"bytes" // Added
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time" // Added for TappableButton

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ftrvxmtrx/tga"
	"github.com/nfnt/resize"
)

type AssetType string

const (
	AssetTypeModel     AssetType = "model"
	AssetTypeTexture   AssetType = "texture"
	AssetTypeSound     AssetType = "sound"
	AssetTypeCharacter AssetType = "character"
	AssetTypeSaber     AssetType = "saber"
	AssetTypeSkin      AssetType = "skin"
	AssetTypeShader    AssetType = "shader"
	AssetTypeIcon      AssetType = "icon"
	AssetTypeGFX       AssetType = "gfx"
	AssetTypeEffect    AssetType = "effect"
	AssetTypeVehicle   AssetType = "vehicle" // New
	AssetTypeOther     AssetType = "other"
)

var QuickNavPaths = map[string]string{
	"Player Models":     "models/players",
	"Weapons":           "models/weapons2",
	"Characters (MBCH)": "ext_data/mb2/character",
	"Sabers (SAB)":      "ext_data/sabers",
	"Vehicles (VEH)":    "ext_data/vehicles",
	"GFX - 2D":          "gfx/2d",
	"GFX - HUD":         "gfx/hud",
	"Effects":           "effects",
}

type AssetEntry struct {
	Name       string
	Path       string
	Size       int64
	Type       AssetType
	PK3Source  string
	IsDir      bool
	Children   []*AssetEntry
	Compressed int64
	Icon       fyne.Resource
}

type AssetBrowser struct {
	container      *fyne.Container
	tree           *widget.Tree
	grid           *fyne.Container
	sourceSelect   *widget.Select
	quickNavSelect *widget.Select
	searchEntry    *widget.Entry
	statusLabel    *widget.Label
	topBar         *fyne.Container // Exposed for visibility toggling

	// View Controls
	viewModeSelect *widget.Select
	zoomSlider     *widget.Slider
	sortSelect     *widget.Select

	gamedataPath   string
	textAssetsPath string // New
	pk3Files       []string
	currentPK3     string
	assets         map[string]*AssetEntry
	rootEntries    []*AssetEntry
	currentDir     *AssetEntry

	onAssetSelected func(asset *AssetEntry)
	onAssetDouble   func(asset *AssetEntry) // New double-click handler

	// Tracks the currently-selected GridItem so we can clear its
	// highlight when another item is clicked. Cleared on every
	// loadGrid / loadFS.
	selectedItem *GridItem

	md3viewPath string
	loadLock    sync.Mutex

	favorites     []string
	favoritesFile string

	viewMode ViewMode
	sortMode SortMode
	iconSize float32

	vfs *VirtualFileSystem
}

type ViewMode string

const (
	ViewModeGrid ViewMode = "Grid"
	ViewModeList ViewMode = "List"
)

type SortMode string

const (
	SortNameAsc  SortMode = "Name (A-Z)"
	SortNameDesc SortMode = "Name (Z-A)"
	SortSizeAsc  SortMode = "Size (Smallest)"
	SortSizeDesc SortMode = "Size (Largest)"
	SortType     SortMode = "Type"
)

func NewAssetBrowser(gamedataPath, textAssetsPath string) *AssetBrowser {
	ab := &AssetBrowser{
		gamedataPath:   gamedataPath,
		textAssetsPath: textAssetsPath,
		assets:         make(map[string]*AssetEntry),
		rootEntries:    []*AssetEntry{},
		favorites:      []string{},
		viewMode:       ViewModeGrid,
		sortMode:       SortNameAsc,
		iconSize:       100.0,
		vfs:            NewVirtualFileSystem(gamedataPath, textAssetsPath),
	}

	ab.favoritesFile = filepath.Join(AppConfigDir(), "favorites.json")
	ab.loadFavorites()

	ab.loadConfig()
	ab.scanPK3Files()
	ab.createUI()
	return ab
}

func (ab *AssetBrowser) SetPaths(gamedata, textAssets string) {
	ab.gamedataPath = gamedata
	ab.textAssetsPath = textAssets
	ab.vfs = NewVirtualFileSystem(gamedata, textAssets)
	ab.scanPK3Files() // Re-scan if gamedata changed
	ab.refreshSources()
}

func (ab *AssetBrowser) loadFavorites() {
	data, err := os.ReadFile(ab.favoritesFile)
	if err == nil {
		json.Unmarshal(data, &ab.favorites)
	}
}

func (ab *AssetBrowser) saveFavorites() {
	data, _ := json.Marshal(ab.favorites)
	os.WriteFile(ab.favoritesFile, data, 0644)
}

func (ab *AssetBrowser) addToFavorites(path string) {
	for _, f := range ab.favorites {
		if f == path {
			return
		}
	}
	ab.favorites = append(ab.favorites, path)
	ab.saveFavorites()
	ab.refreshSources()
}

func (ab *AssetBrowser) loadConfig() {
	configPath := filepath.Join(ab.gamedataPath, "..", "mbii-foundry_config.json")
	data, err := os.ReadFile(configPath)
	if err == nil {
		var config struct {
			MD3ViewPath string `json:"md3view_path"`
		}
		json.Unmarshal(data, &config)
		ab.md3viewPath = config.MD3ViewPath
	}
}

func (ab *AssetBrowser) scanPK3Files() {
	ab.pk3Files = []string{}
	if ab.gamedataPath == "" {
		return
	}

	// Search candidate directories
	candidates := []string{ab.gamedataPath} // The path itself
	candidates = append(candidates, filepath.Join(ab.gamedataPath, "MBII"))
	candidates = append(candidates, filepath.Join(ab.gamedataPath, "MBIITest"))
	candidates = append(candidates, filepath.Join(ab.gamedataPath, "base"))

	// Also check parent if user selected MBII folder directly
	parent := filepath.Dir(ab.gamedataPath)
	if filepath.Base(ab.gamedataPath) == "MBII" || filepath.Base(ab.gamedataPath) == "MBIITest" {
		candidates = append(candidates, filepath.Join(parent, "base"))
		candidates = append(candidates, filepath.Join(parent, "MBII")) // Duplicate but handled
	}

	uniquePaths := make(map[string]bool)

	for _, dir := range candidates {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.pk3"))
		for _, m := range matches {
			if !uniquePaths[m] {
				ab.pk3Files = append(ab.pk3Files, m)
				uniquePaths[m] = true
			}
		}
	}
	sort.Strings(ab.pk3Files)
}

func (ab *AssetBrowser) refreshSources() {
	sources := []string{"Home", "Computer", "All Game Data (Virtual)", "--- Locations ---"}

	// 1. Detect Cloud Storage (macOS specific mainly, but useful)
	homeDir, _ := os.UserHomeDir()
	cloudStoragePath := filepath.Join(homeDir, "Library", "CloudStorage")
	if entries, err := os.ReadDir(cloudStoragePath); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				sources = append(sources, "Cloud: "+e.Name())
				// Store mapping implicitly? Or simpler: the selection handler handles prefix
			}
		}
	}

	// 2. Detect Volumes (External Drives)
	if entries, err := os.ReadDir("/Volumes"); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "Macintosh HD" {
				sources = append(sources, "Volume: "+e.Name())
			}
		}
	}

	// 3. Workspace Root (Parent of gamedata)
	if ab.gamedataPath != "" {
		workspace := filepath.Dir(ab.gamedataPath)
		sources = append(sources, "Workspace: "+filepath.Base(workspace))
	}

	if ab.textAssetsPath != "" {
		sources = append(sources, "TextAssets")
	}

	// 4. Favorites
	if len(ab.favorites) > 0 {
		sources = append(sources, "--- Favorites ---")
		sources = append(sources, ab.favorites...)
	}

	sources = append(sources, "--- PK3s ---")
	pk3Names := make([]string, len(ab.pk3Files))
	for i, p := range ab.pk3Files {
		pk3Names[i] = filepath.Base(p)
	}
	sources = append(sources, pk3Names...)

	ab.sourceSelect.Options = sources
	ab.sourceSelect.Refresh()
}

func (ab *AssetBrowser) createUI() {
	ab.sourceSelect = widget.NewSelect([]string{}, func(s string) {
		if s == "All Game Data (Virtual)" {
			ab.loadVFS("") // Load VFS root
		} else if s == "Home" {
			homeDir, _ := os.UserHomeDir()
			ab.loadFS(homeDir)
		} else if s == "Computer" {
			ab.loadFS("/")
		} else if s == "TextAssets" {
			if ab.textAssetsPath != "" {
				ab.loadFS(ab.textAssetsPath)
			}
		} else if strings.HasPrefix(s, "Workspace:") {
			if ab.gamedataPath != "" {
				workspace := filepath.Dir(ab.gamedataPath)
				ab.loadFS(workspace)
			}
		} else if strings.HasPrefix(s, "Cloud: ") {
			name := strings.TrimPrefix(s, "Cloud: ")
			homeDir, _ := os.UserHomeDir()
			path := filepath.Join(homeDir, "Library", "CloudStorage", name)
			ab.loadFS(path)
		} else if strings.HasPrefix(s, "Volume: ") {
			name := strings.TrimPrefix(s, "Volume: ")
			ab.loadFS(filepath.Join("/Volumes", name))
		} else if strings.HasPrefix(s, "---") {
			// Separator, do nothing
		} else {
			// Check Favorites (Path)
			isFav := false
			for _, f := range ab.favorites {
				if f == s {
					ab.loadFS(f)
					isFav = true
					break
				}
			}
			if isFav {
				return
			}

			// Check PK3s
			for _, p := range ab.pk3Files {
				if filepath.Base(p) == s {
					ab.loadPK3(p)
					return
				}
			}

			// Fallback: try to load as path if it looks like one
			if filepath.IsAbs(s) {
				ab.loadFS(s)
			}
		}
	})
	ab.sourceSelect.PlaceHolder = "Select Source..."
	ab.refreshSources() // Populate initial options

	favBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		if ab.currentDir != nil && ab.currentDir.PK3Source == "" && ab.currentDir.Path != "" {
			ab.addToFavorites(ab.currentDir.Path)
		}
	})
	favBtn.Importance = widget.LowImportance

	navOptions := make([]string, 0, len(QuickNavPaths))
	for k := range QuickNavPaths {
		navOptions = append(navOptions, k)
	}
	sort.Strings(navOptions)
	ab.quickNavSelect = widget.NewSelect(navOptions, func(s string) {
		if path, ok := QuickNavPaths[s]; ok {
			ab.navigateToPath(path)
		}
	})
	ab.quickNavSelect.PlaceHolder = "Quick Nav..."

	ab.searchEntry = NewInputEntry()
	ab.searchEntry.SetPlaceHolder("Search...")
	ab.searchEntry.OnChanged = func(s string) { ab.filterGrid(s) }

	// View Controls
	ab.viewModeSelect = widget.NewSelect([]string{string(ViewModeGrid), string(ViewModeList)}, func(s string) {
		ab.viewMode = ViewMode(s)
		if ab.currentDir != nil {
			ab.loadGrid(ab.currentDir)
		}
	})
	ab.viewModeSelect.SetSelected(string(ab.viewMode))

	ab.sortSelect = widget.NewSelect([]string{string(SortNameAsc), string(SortNameDesc), string(SortSizeAsc), string(SortSizeDesc), string(SortType)}, func(s string) {
		ab.sortMode = SortMode(s)
		if ab.currentDir != nil {
			ab.loadGrid(ab.currentDir)
		}
	})
	ab.sortSelect.SetSelected(string(ab.sortMode))

	ab.zoomSlider = widget.NewSlider(50, 250)
	ab.zoomSlider.Value = float64(ab.iconSize)
	ab.zoomSlider.OnChanged = func(f float64) {
		ab.iconSize = float32(f)
		ab.updateGridSize() // New method to update layout without full reload
	}

	// Tightened layout: navigation buttons + source + search on two rows
	// instead of four. Up/Home/Refresh buttons satisfy "how do I go back
	// out of a dir?" without users needing to find the ".. (Up)" tile.
	// View/Sort collapse into a compact row; Zoom moves into that row
	// too (no dedicated label, takes remaining space).
	upBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if ab.currentDir != nil && ab.currentDir.PK3Source == "" && ab.currentDir.Path != "" {
			parent := filepath.Dir(ab.currentDir.Path)
			if parent != "" && parent != "." {
				ab.loadFS(parent)
			}
		}
	})
	upBtn.Importance = widget.LowImportance

	homeBtn := widget.NewButtonWithIcon("", theme.HomeIcon(), func() {
		home, _ := os.UserHomeDir()
		ab.loadFS(home)
	})
	homeBtn.Importance = widget.LowImportance

	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		if ab.currentDir != nil && ab.currentDir.PK3Source == "" && ab.currentDir.Path != "" {
			ab.loadFS(ab.currentDir.Path)
		}
	})
	refreshBtn.Importance = widget.LowImportance

	navButtons := container.NewHBox(upBtn, homeBtn, refreshBtn)

	ab.topBar = container.NewVBox(
		container.NewBorder(nil, nil, navButtons, favBtn, ab.sourceSelect),
		ab.searchEntry,
		container.NewGridWithColumns(3, ab.viewModeSelect, ab.sortSelect, ab.zoomSlider),
	)

	ab.tree = widget.NewTree(
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			entry := ab.assets[id]
			if id == "" { // Root
				ids := []widget.TreeNodeID{}
				for _, e := range ab.rootEntries {
					if e.IsDir {
						ids = append(ids, e.Path)
					}
				}
				return ids
			}
			if entry != nil {
				ids := []widget.TreeNodeID{}
				for _, child := range entry.Children {
					if child.IsDir {
						ids = append(ids, child.Path)
					}
				}
				return ids
			}
			return nil
		},
		func(id widget.TreeNodeID) bool { return true },
		func(branch bool) fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.FolderIcon()), widget.NewLabel("Dir"))
		},
		func(id widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			if entry, ok := ab.assets[id]; ok {
				obj.(*fyne.Container).Objects[1].(*widget.Label).SetText(entry.Name)
			}
		},
	)
	ab.tree.OnSelected = func(id widget.TreeNodeID) {
		if entry, ok := ab.assets[id]; ok {
			if entry.IsDir {
				if entry.PK3Source == "" {
					// FS Mode: Navigate into this folder
					ab.loadFS(entry.Path)
				} else if entry.PK3Source == "VFS" {
					// VFS Mode
					ab.loadVFS(entry.Path)
				} else {
					// PK3 Mode: Show contents in grid
					ab.loadGrid(entry)
				}
			} else {
				// Single click on file in tree, select it
				if ab.onAssetSelected != nil {
					ab.onAssetSelected(entry)
				}
			}
		}
	}

	ab.grid = container.NewGridWrap(fyne.NewSize(ab.iconSize, ab.iconSize+30)) // Init with size
	// Empty by default — "Ready" was noise that duplicated the app's
	// global status label. This label now just mirrors directory
	// load results (see loadFS / loadPK3).
	ab.statusLabel = widget.NewLabel("")

	split := container.NewHSplit(container.NewScroll(ab.tree), container.NewScroll(ab.grid))
	split.SetOffset(0.3)

	ab.container = container.NewBorder(ab.topBar, ab.statusLabel, nil, nil, split)
}

func (ab *AssetBrowser) updateGridSize() {
	if ab.viewMode == ViewModeList {
		ab.grid.Layout = layout.NewVBoxLayout()
	} else {
		ab.grid.Layout = layout.NewGridWrapLayout(fyne.NewSize(ab.iconSize, ab.iconSize+30))
	}
	ab.grid.Refresh()
}

// Helper to sort assets
func (ab *AssetBrowser) sortAssets(assets []*AssetEntry) {
	sort.Slice(assets, func(i, j int) bool {
		a, b := assets[i], assets[j]
		// Always keep directories first?
		if a.IsDir != b.IsDir {
			return a.IsDir
		}

		switch ab.sortMode {
		case SortNameAsc:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortNameDesc:
			return strings.ToLower(a.Name) > strings.ToLower(b.Name)
		case SortSizeAsc:
			return a.Size < b.Size
		case SortSizeDesc:
			return a.Size > b.Size
		case SortType:
			if a.Type != b.Type {
				return a.Type < b.Type
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		}
	})
}

func (ab *AssetBrowser) ShowTopBar(show bool) {
	if show {
		ab.topBar.Show()
	} else {
		ab.topBar.Hide()
	}
	ab.container.Refresh()
}

func (ab *AssetBrowser) GetContent() fyne.CanvasObject { return ab.container }

func (ab *AssetBrowser) loadPK3(pk3Path string) {
	ab.currentPK3 = pk3Path
	ab.assets = make(map[string]*AssetEntry)
	ab.rootEntries = []*AssetEntry{}
	ab.grid.Objects = nil

	reader, err := zip.OpenReader(pk3Path)
	if err != nil {
		ab.statusLabel.SetText("Error opening PK3")
		return
	}
	defer reader.Close()

	dirMap := make(map[string]*AssetEntry)

	for _, file := range reader.File {
		path := file.Name
		if strings.HasSuffix(path, "/") {
			continue
		}

		entry := &AssetEntry{
			Name: filepath.Base(path), Path: path, Size: int64(file.UncompressedSize64),
			Type: detectAssetType(path), PK3Source: pk3Path, IsDir: false,
		}
		ab.assets[path] = entry

		dir := filepath.Dir(path)
		ab.ensureDirectory(dirMap, dir, pk3Path)
		if parent, ok := dirMap[dir]; ok {
			parent.Children = append(parent.Children, entry)
		}
	}

	for _, entry := range dirMap {
		if !strings.Contains(entry.Path, "/") || filepath.Dir(entry.Path) == "." {
			ab.rootEntries = append(ab.rootEntries, entry)
		}
		ab.assets[entry.Path] = entry
	}

	ab.statusLabel.SetText(fmt.Sprintf("Loaded %d assets", len(ab.assets)))
	ab.tree.Refresh()
}

func (ab *AssetBrowser) ensureDirectory(dirMap map[string]*AssetEntry, path, pk3Source string) {
	if _, exists := dirMap[path]; exists || path == "." {
		return
	}

	entry := &AssetEntry{Name: filepath.Base(path), Path: path, Type: AssetTypeOther, PK3Source: pk3Source, IsDir: true, Children: []*AssetEntry{}}
	dirMap[path] = entry
	ab.assets[path] = entry

	parent := filepath.Dir(path)
	if parent != "." && parent != path {
		ab.ensureDirectory(dirMap, parent, pk3Source)
		if parentEntry, ok := dirMap[parent]; ok {
			parentEntry.Children = append(parentEntry.Children, entry)
		}
	} else if parent == "." {
		ab.rootEntries = append(ab.rootEntries, entry)
	}
}

func (ab *AssetBrowser) loadGrid(dir *AssetEntry) {
	ab.currentDir = dir
	// Clear any selected-item pointer — the widget's about to be
	// destroyed when we rebuild the grid.
	ab.selectedItem = nil

	ab.grid.Objects = nil
	ab.grid.Refresh()

	ab.loadLock.Lock()
	defer ab.loadLock.Unlock()

	var objects []fyne.CanvasObject

	// Add ".." (Parent Directory) if not at root
	if dir.Path != "" && dir.Path != "." {
		if ab.currentPK3 != "" {
			parentPath := filepath.Dir(dir.Path)
			if parentPath == "." {
				parentPath = ""
			} // Fix parent of top-level folders
			parentDir := ab.assets[parentPath]

			// For VFS, ensure we have a parent entry even if not cached
			if parentDir == nil && ab.currentPK3 == "VFS" {
				parentDir = &AssetEntry{Name: "(Parent)", Path: parentPath, IsDir: true, PK3Source: "VFS"}
			} else if parentDir == nil {
				parentDir = &AssetEntry{Name: "(Parent)", Path: parentPath, IsDir: true, PK3Source: ab.currentPK3}
			}
			objects = append(objects, ab.createGridItem(parentDir, true))
		}
	}

	// Create a copy to sort without affecting tree order (optional)
	children := make([]*AssetEntry, len(dir.Children))
	copy(children, dir.Children)
	ab.sortAssets(children)

	for _, child := range children {
		objects = append(objects, ab.createGridItem(child, false))
	}

	ab.updateGridSize() // Ensure layout is correct
	ab.grid.Objects = objects
	ab.grid.Refresh()
}

type TappableButton struct {
	widget.Button
	OnDoubleTapped func()
	OnRightClick   func(*fyne.PointEvent)
	lastTap        time.Time
}

func (t *TappableButton) Tapped(event *fyne.PointEvent) {
	if time.Since(t.lastTap) < 300*time.Millisecond && t.OnDoubleTapped != nil {
		t.OnDoubleTapped()
		t.lastTap = time.Time{} // Reset for next tap
	} else {
		t.Button.Tapped(event)
		t.lastTap = time.Now()
	}
}

func (t *TappableButton) TappedSecondary(event *fyne.PointEvent) {
	if t.OnRightClick != nil {
		t.OnRightClick(event)
	}
}

// Create TappableButton
func NewTappableButton(label string, icon fyne.Resource, tapped func()) *TappableButton {
	btn := &TappableButton{}
	btn.ExtendBaseWidget(btn)
	btn.Text = label
	btn.Icon = icon
	btn.OnTapped = tapped
	return btn
}

// GridItem is a custom widget for asset display. Implements
// desktop.Hoverable so the background tints on mouse-over, and tracks
// Selected state so the tapped item renders with a primary-color
// highlight. The AssetBrowser is responsible for clearing the prior
// selection when a new item is tapped (see createGridItem).
type GridItem struct {
	widget.BaseWidget
	Text           string
	Icon           fyne.Resource
	OnTapped       func()
	OnDoubleTapped func()
	ViewMode       ViewMode

	// TypeBadge is a short tag (MBCH / SAB / VEH / etc.) rendered
	// centered on the icon in grid view so file types are distinguishable
	// at a glance without mangling the filename with a "[MBCH] " prefix.
	// Empty string = no badge.
	TypeBadge string

	selected   bool
	hovering   bool
	background *canvas.Rectangle
}

func NewGridItem(text string, icon fyne.Resource, tapped func()) *GridItem {
	g := &GridItem{Text: text, Icon: icon, OnTapped: tapped}
	g.ExtendBaseWidget(g)
	return g
}

func (g *GridItem) CreateRenderer() fyne.WidgetRenderer {
	g.background = canvas.NewRectangle(color.Transparent)
	g.background.CornerRadius = 4

	img := widget.NewIcon(g.Icon)
	lbl := widget.NewLabel(g.Text)
	lbl.TextStyle = fyne.TextStyle{Monospace: true}

	var iconArea fyne.CanvasObject = img
	// Grid-view type badge: render a compact centered label on top of
	// the document icon. List view skips this since the extension is
	// already visible in the filename.
	if g.TypeBadge != "" && g.ViewMode != ViewModeList {
		badge := canvas.NewText(g.TypeBadge, theme.ForegroundColor())
		badge.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
		badge.TextSize = 11
		badge.Alignment = fyne.TextAlignCenter
		iconArea = container.NewStack(img, container.NewCenter(badge))
	}

	var content *fyne.Container
	if g.ViewMode == ViewModeList {
		lbl.Alignment = fyne.TextAlignLeading
		content = container.NewHBox(iconArea, lbl)
	} else {
		lbl.Alignment = fyne.TextAlignCenter
		lbl.Wrapping = fyne.TextTruncate
		content = container.NewBorder(nil, lbl, nil, nil, iconArea)
	}
	// Stack background behind the content so click/hover feedback
	// shows without affecting layout.
	return widget.NewSimpleRenderer(container.NewStack(g.background, content))
}

func (g *GridItem) Tapped(_ *fyne.PointEvent) {
	if g.OnTapped != nil {
		g.OnTapped()
	}
}

func (g *GridItem) DoubleTapped(_ *fyne.PointEvent) {
	if g.OnDoubleTapped != nil {
		g.OnDoubleTapped()
	}
}

// SetSelected highlights the item with the primary theme color. Caller
// (AssetBrowser) clears the previous selection before setting a new
// one so only one GridItem is highlighted at a time.
func (g *GridItem) SetSelected(selected bool) {
	if g.selected == selected {
		return
	}
	g.selected = selected
	g.updateBackground()
}

// --- desktop.Hoverable ------------------------------------------------

func (g *GridItem) MouseIn(*desktop.MouseEvent) {
	g.hovering = true
	g.updateBackground()
}

func (g *GridItem) MouseOut() {
	g.hovering = false
	g.updateBackground()
}

func (g *GridItem) MouseMoved(*desktop.MouseEvent) {}

func (g *GridItem) updateBackground() {
	if g.background == nil {
		return
	}
	switch {
	case g.selected:
		// Primary-tinted fill for selection — clearly differentiates
		// from hover. Alpha 96 keeps the label readable.
		g.background.FillColor = tintWithAlpha(CurrentThemeColor, 96)
	case g.hovering:
		g.background.FillColor = tintWithAlpha(CurrentThemeColor, 38)
	default:
		g.background.FillColor = color.Transparent
	}
	g.background.Refresh()
}

func tintWithAlpha(c color.Color, alpha uint8) color.Color {
	r, gr, b, _ := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(gr >> 8), B: uint8(b >> 8), A: alpha}
}

// createGridItem creates a clickable item for the asset grid.
//
// Visual treatment:
//   - Parent ("..") entries get a dedicated up-arrow icon + "⬆ Up" label.
//   - Image/model files get their own theme icons.
//   - Other known MBII file types get a type badge (MBCH / SAB / VEH /
//     SIEGE / MBTC / SKIN / …) overlaid on the icon in grid view; in
//     list view there's no overlay since the extension is already in
//     the filename.
func (ab *AssetBrowser) createGridItem(entry *AssetEntry, isParent bool) fyne.CanvasObject {
	var icon fyne.Resource = theme.FileIcon()
	displayName := entry.Name
	var typeBadge string // only populated for the grid-view overlay

	switch {
	case isParent:
		icon = theme.NavigateBackIcon()
		displayName = "⬆ Up"
	case entry.IsDir:
		icon = theme.FolderIcon()
	case ab.isImageAsset(entry):
		icon = theme.FileImageIcon()
	case entry.Type == AssetTypeModel:
		icon = theme.ComputerIcon()
	default:
		// In grid view, render the type as a badge over the icon so
		// users can tell an .mbch from a .sab at a glance without the
		// filename getting mangled by a "[MBCH] " prefix. In list view
		// the filename is shown in full — no badge needed.
		if ab.viewMode == ViewModeGrid {
			typeBadge = assetTypeTag(entry)
		}
	}

	item := NewGridItem(displayName, icon, nil)
	item.ViewMode = ab.viewMode
	item.TypeBadge = typeBadge
	item.OnTapped = func() {
		// Highlight this item; clear any previous selection so only
		// one item shows the primary-tinted background.
		if ab.selectedItem != nil && ab.selectedItem != item {
			ab.selectedItem.SetSelected(false)
		}
		item.SetSelected(true)
		ab.selectedItem = item
		if ab.onAssetSelected != nil {
			ab.onAssetSelected(entry)
		}
	}

	item.OnDoubleTapped = func() {
		if entry.IsDir || isParent {
			if entry.PK3Source == "" {
				ab.loadFS(entry.Path)
			} else if entry.PK3Source == "VFS" {
				ab.loadVFS(entry.Path)
			} else {
				ab.loadGrid(entry)
			}
		} else {
			if ab.onAssetDouble != nil {
				ab.onAssetDouble(entry)
			}
		}
	}

	return item
}

func (ab *AssetBrowser) ensureCacheDir() string {
	dir := filepath.Join(os.TempDir(), "mbii-fa-cache")
	os.MkdirAll(dir, 0755)
	return dir
}

func (ab *AssetBrowser) LoadIconResource(path string) fyne.Resource {
	// 1. Check Cache
	hash := md5.Sum([]byte(path))
	hashStr := hex.EncodeToString(hash[:])
	cachePath := filepath.Join(ab.ensureCacheDir(), hashStr+".png")

	if data, err := os.ReadFile(cachePath); err == nil {
		return fyne.NewStaticResource(filepath.Base(path), data)
	}

	if ab.vfs == nil {
		return nil
	}

	// 2. Load from VFS
	rc, err := ab.vfs.ReadFile(path)
	if err != nil {
		return nil
	}
	defer rc.Close()

	var img image.Image
	ext := strings.ToLower(filepath.Ext(path))

	if ext == ".tga" {
		img, err = tga.Decode(rc)
	} else if ext == ".jpg" || ext == ".jpeg" {
		img, err = jpeg.Decode(rc)
	} else if ext == ".png" {
		img, err = png.Decode(rc)
	} else {
		// Shaders? Not handled yet
		return nil
	}

	if err != nil || img == nil {
		return nil
	}

	// 3. Resize to Icon Size (e.g. 64x64 or 128x128)
	// Larger for quality, smaller for speed. 128 is good.
	// Use Thumbnail to preserve aspect ratio
	img = resize.Thumbnail(128, 128, img, resize.Lanczos3)

	// 4. Encode to PNG and Cache
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}

	data := buf.Bytes()
	os.WriteFile(cachePath, data, 0644)

	return fyne.NewStaticResource(filepath.Base(path)+".png", data)
}

func (ab *AssetBrowser) isImageAsset(asset *AssetEntry) bool {
	return asset.Type == AssetTypeTexture || asset.Type == AssetTypeIcon || asset.Type == AssetTypeGFX
}

func (ab *AssetBrowser) loadImage(asset *AssetEntry) image.Image {
	if ab.vfs != nil && asset.PK3Source != "" && asset.PK3Source != "VFS" {
		// Try using VFS helper if it's a known PK3
		rc, err := ab.vfs.ReadFile(asset.Path)
		if err == nil {
			defer rc.Close()
			return decodeImage(rc, asset.Name)
		}
	} else if asset.PK3Source != "" && asset.PK3Source != "VFS" {
		// Legacy PK3 loading
		reader, err := zip.OpenReader(asset.PK3Source)
		if err != nil {
			return nil
		}
		defer reader.Close()

		for _, f := range reader.File {
			if f.Name == asset.Path {
				rc, err := f.Open()
				if err != nil {
					return nil
				}
				defer rc.Close()
				return decodeImage(rc, asset.Name)
			}
		}
	} else if asset.PK3Source == "VFS" && ab.vfs != nil {
		// VFS loading
		rc, err := ab.vfs.ReadFile(asset.Path)
		if err == nil {
			defer rc.Close()
			return decodeImage(rc, asset.Name)
		}
	}
	return nil
}

func decodeImage(r io.Reader, filename string) image.Image {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".tga" {
		img, _ := tga.Decode(r)
		return img
	} else if ext == ".jpg" {
		img, _ := jpeg.Decode(r)
		return img
	} else if ext == ".png" {
		img, _ := png.Decode(r)
		return img
	}
	return nil
}

func (ab *AssetBrowser) filterGrid(text string) {
	if text == "" {
		if ab.currentDir != nil {
			ab.loadGrid(ab.currentDir)
		}
		return
	}

	ab.grid.Objects = nil

	ab.loadLock.Lock()
	defer ab.loadLock.Unlock()

	var objects []fyne.CanvasObject
	textLower := strings.ToLower(text)

	count := 0
	maxResults := 100

	for _, entry := range ab.assets {
		if count >= maxResults {
			break
		}
		if entry.IsDir {
			continue
		}

		if strings.Contains(strings.ToLower(entry.Name), textLower) {
			objects = append(objects, ab.createGridItem(entry, false)) // Use new item creator
			count++
		}
	}

	ab.grid.Objects = objects
	ab.grid.Refresh()
}

func (ab *AssetBrowser) navigateToPath(path string) {
	// TODO: Implement actual navigation into the asset tree/grid.
	// For now, this is a placeholder.
	// This would involve finding the entry for the path and calling loadGrid or expanding tree.
	ab.statusLabel.SetText("QuickNav to: " + path + " (Not yet fully implemented)")
}

func (ab *AssetBrowser) SetOnAssetSelected(f func(*AssetEntry)) { ab.onAssetSelected = f }
func (ab *AssetBrowser) SetOnAssetDouble(f func(*AssetEntry))   { ab.onAssetDouble = f } // New method

func (ab *AssetBrowser) GetSelectedAsset() *AssetEntry {
	// This method is primarily used by the custom file picker logic.
	// It should reflect the last single-clicked asset.
	return nil // To be implemented with actual selection tracking
}

func (ab *AssetBrowser) Refresh() { ab.scanPK3Files() }

func (ab *AssetBrowser) loadVFS(path string) {
	if ab.vfs == nil {
		return
	}

	// Ensure VFS is indexed (lazy load)
	if len(ab.vfs.Index) == 0 {
		ab.statusLabel.SetText("Indexing Game Assets...")
		ab.vfs.Refresh()
	}

	ab.currentPK3 = "VFS" // Marker
	ab.assets = make(map[string]*AssetEntry)
	ab.rootEntries = []*AssetEntry{}
	ab.grid.Objects = nil

	contents, ok := ab.vfs.Directories[path]
	if !ok && path != "" {
		ab.statusLabel.SetText("Path not found in VFS")
		return
	}

	// Create entries
	for _, src := range contents {
		entry := &AssetEntry{
			Name:      filepath.Base(src.Path),
			Path:      src.Path,
			Size:      src.Size,
			Type:      detectAssetType(src.Path),
			PK3Source: src.PK3Path,
			IsDir:     src.IsDirectory,
		}
		// If it's a directory, we need to populate children for tree view?
		// For now, tree view in VFS mode might be tricky if we don't build full tree.
		// Let's just handle current directory for Grid.
		ab.assets[entry.Path] = entry
		ab.rootEntries = append(ab.rootEntries, entry)
	}

	// Sort
	ab.sortAssets(ab.rootEntries)

	ab.statusLabel.SetText(fmt.Sprintf("VFS: %s (%d items)", path, len(ab.rootEntries)))

	// Dummy entry for grid loading
	dummyDir := &AssetEntry{Path: path, IsDir: true, Children: ab.rootEntries, PK3Source: "VFS"}
	ab.loadGrid(dummyDir)
}

func (ab *AssetBrowser) loadFS(path string) {
	path = filepath.Clean(path)
	LogInfo("Navigating to FS: %s", path)
	ab.currentPK3 = "" // Not in a PK3
	ab.assets = make(map[string]*AssetEntry)
	ab.rootEntries = []*AssetEntry{}

	ab.grid.Objects = nil
	ab.grid.Refresh()

	entries, err := os.ReadDir(path)
	if err != nil {
		ab.statusLabel.SetText("Error reading directory: " + err.Error())
		return
	}

	// Collect regular entries
	var dirEntries []*AssetEntry
	var fileEntries []*AssetEntry

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		} // Skip hidden

		fullPath := filepath.Join(path, e.Name())
		info, _ := e.Info()
		size := int64(0)
		isDir := e.IsDir()

		// Check symlinks
		if !isDir && (e.Type()&os.ModeSymlink != 0) {
			if targetInfo, err := os.Stat(fullPath); err == nil {
				isDir = targetInfo.IsDir()
			}
		}

		if info != nil {
			size = info.Size()
		}

		entry := &AssetEntry{
			Name:      e.Name(),
			Path:      fullPath,
			Size:      size,
			Type:      detectAssetType(fullPath),
			PK3Source: "",
			IsDir:     isDir,
		}

		ab.assets[fullPath] = entry

		if isDir {
			dirEntries = append(dirEntries, entry)
		} else {
			fileEntries = append(fileEntries, entry)
		}
	}

	// Sort separately
	// Use unified sorter
	ab.sortAssets(dirEntries)
	ab.sortAssets(fileEntries)

	// Add Parent ".." if not at root
	parent := filepath.Dir(path)
	if parent != path && parent != "." {
		parentEntry := &AssetEntry{
			Name: ".. (Up)", Path: parent, IsDir: true, PK3Source: "",
		}
		ab.rootEntries = append(ab.rootEntries, parentEntry)
	}

	ab.rootEntries = append(ab.rootEntries, dirEntries...)
	ab.rootEntries = append(ab.rootEntries, fileEntries...)

	ab.statusLabel.SetText(fmt.Sprintf("Loaded %d items", len(ab.rootEntries)))

	// Hack: Set currentDir to a dummy entry containing these children
	dummyDir := &AssetEntry{Path: path, IsDir: true, Children: ab.rootEntries}
	ab.loadGrid(dummyDir)
}

func detectAssetType(path string) AssetType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".glm", ".md3":
		return AssetTypeModel
	case ".jpg", ".tga", ".png":
		return AssetTypeTexture
	case ".mbch":
		return AssetTypeCharacter
	case ".sab":
		return AssetTypeSaber
	case ".veh":
		return AssetTypeVehicle
	default:
		return AssetTypeOther
	}
}

// assetTypeTag returns a short display tag for a known MBII asset type,
// used to prefix the filename in the grid/list view so users can
// distinguish files at a glance without needing per-type icons.
// Returns empty string for types that don't merit a tag (images/models
// already get dedicated icons; generic "other" files get no prefix).
func assetTypeTag(entry *AssetEntry) string {
	if entry == nil {
		return ""
	}
	switch entry.Type {
	case AssetTypeCharacter:
		return "MBCH"
	case AssetTypeSaber:
		return "SAB"
	case AssetTypeVehicle:
		return "VEH"
	}
	// Not yet a known type — fall back to the raw extension for
	// anything we explicitly recognize by suffix.
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(entry.Name)), ".")
	switch ext {
	case "siege":
		return "SIEGE"
	case "mbtc":
		return "MBTC"
	case "skin":
		return "SKIN"
	case "shader":
		return "SHADER"
	case "efx":
		return "EFX"
	}
	return ""
}
