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
	"image/png"
	"io"
	"os"
	pathpkg "path"
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
	container         *fyne.Container
	tree              *widget.Tree
	grid              *fyne.Container
	sourceSelect      *widget.Select
	quickNavSelect    *widget.Select
	searchEntry       *widget.Entry
	statusLabel       *widget.Label
	breadcrumbLabel   *widget.Label
	topBar            *fyne.Container // Exposed for visibility toggling
	emptyState        fyne.CanvasObject
	emptyHeadline     *widget.Label
	emptySelectButton *widget.Button

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
	selectedItem  *GridItem
	selectedAsset *AssetEntry
	treeSyncing   bool

	md3viewPath string
	loadLock    sync.Mutex

	favorites     []string
	favoritesFile string

	viewMode ViewMode
	sortMode SortMode
	iconSize float32

	vfs *VirtualFileSystem

	// shaderResolver lazily parses .shader files in the VFS so a
	// `uishader models/players/X/mb2_icon_Y` reference resolves to
	// the texture path the engine would actually render. Invalidated
	// on every VFS refresh.
	shaderResolver *ShaderResolver

	// Registered before indexing starts; invoked on the Fyne thread.
	onVFSReady func()
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

func NewAssetBrowser(gamedataPath, textAssetsPath string, onReady func()) *AssetBrowser {
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
		onVFSReady:     onReady,
	}

	configDir, err := AppConfigDir()
	if err != nil {
		LogInfo("Diagnostics: Config dir migration warning or error: " + err.Error())
	}
	if configDir != "" {
		ab.favoritesFile = filepath.Join(configDir, "favorites.json")
		ab.loadFavorites()
	} else {
		LogInfo("Diagnostics: Favorites unavailable, staying memory-only.")
	}

	ab.loadConfig()
	ab.scanPK3Files()
	ab.shaderResolver = NewShaderResolver(ab.vfs)
	ab.refreshVFS()
	ab.createUI()
	return ab
}

func (ab *AssetBrowser) SetPaths(gamedata, textAssets string) {
	ab.gamedataPath = gamedata
	ab.textAssetsPath = textAssets
	ab.vfs = NewVirtualFileSystem(gamedata, textAssets)
	// Each scan owns this source pair, even if paths change again before it finishes.
	ab.shaderResolver = NewShaderResolver(ab.vfs)
	ab.scanPK3Files() // Re-scan if gamedata changed
	ab.refreshSources()
	ab.refreshVFS()
}

func (ab *AssetBrowser) refreshVFS() {
	vfs, resolver := ab.vfs, ab.shaderResolver
	go func() {
		if err := vfs.Refresh(); err != nil {
			LogInfo("VFS index failed: %v", err)
			return
		}
		resolver.Prebuild()
		if ab.onVFSReady != nil {
			fyne.Do(func() {
				if ab.vfs == vfs {
					ab.onVFSReady()
				}
			})
		}
	}()
}

func (ab *AssetBrowser) loadFavorites() {
	if ab.favoritesFile == "" {
		return
	}
	data, err := os.ReadFile(ab.favoritesFile)
	if err == nil {
		json.Unmarshal(data, &ab.favorites)
	}
}

func (ab *AssetBrowser) saveFavorites() {
	if ab.favoritesFile == "" {
		return
	}
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

// removeFromFavorites drops a path from the favorites list + persists.
// No-op when the path isn't currently favorited — so UI callers can
// fire this without checking first.
func (ab *AssetBrowser) removeFromFavorites(path string) {
	for i, f := range ab.favorites {
		if f == path {
			ab.favorites = append(ab.favorites[:i], ab.favorites[i+1:]...)
			ab.saveFavorites()
			ab.refreshSources()
			return
		}
	}
}

// IsFavorited reports whether a path is currently in the favorites
// list. Used by the file-picker to render the correct toggle state
// on its "favorite this folder" button.
func (ab *AssetBrowser) IsFavorited(path string) bool {
	for _, f := range ab.favorites {
		if f == path {
			return true
		}
	}
	return false
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
	ab.sourceSelect.PlaceHolder = "Select source"
	ab.refreshSources() // Populate initial options

	favBtn := NewTooltipButton("", theme.ContentAddIcon(), func() {
		if ab.currentDir != nil && ab.currentDir.PK3Source == "" && ab.currentDir.Path != "" {
			ab.addToFavorites(ab.currentDir.Path)
		}
	}, "Add the current folder to favorites")
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
	ab.quickNavSelect.PlaceHolder = "Choose common path"

	ab.searchEntry = NewInputEntry()
	ab.searchEntry.SetPlaceHolder("Search...")
	ab.searchEntry.OnChanged = func(s string) { ab.filterGrid(s) }

	// View Controls — icon toggle (grid ↔ list) instead of a dropdown.
	// The dropdown took two clicks and a full label's worth of space
	// to swap between two states; an icon toggle is the natural shape.
	// Icon shown reflects the CURRENT mode so the user sees "I am in
	// grid view" at a glance.
	viewModeBtn := NewTooltipButton("", theme.GridIcon(), nil, "Switch to list view")
	viewModeBtn.Importance = widget.LowImportance
	// Icon shows the mode you'd SWITCH TO on click — standard toolbar
	// toggle convention. Currently in grid → button shows list (tap
	// me to see a list). Currently in list → button shows grid.
	syncViewModeIcon := func() {
		if ab.viewMode == ViewModeGrid {
			viewModeBtn.SetIcon(theme.ListIcon())
			viewModeBtn.SetTooltip("Switch to list view")
		} else {
			viewModeBtn.SetIcon(theme.GridIcon())
			viewModeBtn.SetTooltip("Switch to grid view")
		}
	}
	viewModeBtn.OnTapped = func() {
		if ab.viewMode == ViewModeGrid {
			ab.viewMode = ViewModeList
		} else {
			ab.viewMode = ViewModeGrid
		}
		syncViewModeIcon()
		if ab.currentDir != nil {
			ab.loadGrid(ab.currentDir)
		}
	}
	syncViewModeIcon()

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
	// MoveUpIcon (↑) — the action is "go up a directory", not
	// history-back. The back-arrow was reading as a browser back
	// button, which is semantically wrong here.
	upBtn := NewTooltipButton("", theme.MoveUpIcon(), func() {
		if ab.currentDir != nil && ab.currentDir.PK3Source == "" && ab.currentDir.Path != "" {
			parent := filepath.Dir(ab.currentDir.Path)
			if parent != "" && parent != "." {
				ab.loadFS(parent)
			}
		}
	}, "Go to parent folder")
	upBtn.Importance = widget.LowImportance

	homeBtn := NewTooltipButton("", theme.HomeIcon(), func() {
		home, _ := os.UserHomeDir()
		ab.loadFS(home)
	}, "Go to home folder")
	homeBtn.Importance = widget.LowImportance

	refreshBtn := NewTooltipButton("", theme.ViewRefreshIcon(), func() {
		if ab.currentDir != nil && ab.currentDir.PK3Source == "" && ab.currentDir.Path != "" {
			ab.loadFS(ab.currentDir.Path)
		}
	}, "Refresh the current folder")
	refreshBtn.Importance = widget.LowImportance

	navButtons := container.NewHBox(upBtn, homeBtn, refreshBtn)

	// Control row: sort dropdown + zoom slider take the left/center,
	// view-mode icon toggle pinned to the far right (Border's trailing
	// slot). Feels like a standard file-browser toolbar where display
	// controls live on the right edge.
	controlsLeft := container.New(layout.NewGridLayoutWithColumns(2), ab.sortSelect, ab.zoomSlider)
	controlsRow := container.NewBorder(nil, nil, nil, viewModeBtn, controlsLeft)

	ab.breadcrumbLabel = widget.NewLabel("VFS /")
	ab.breadcrumbLabel.Truncation = fyne.TextTruncateEllipsis

	sourceCaption := widget.NewLabelWithStyle("Source", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	quickNavCaption := widget.NewLabelWithStyle("Go to asset path", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	quickNavHelp := NewTooltipButton("", theme.HelpIcon(), nil,
		"Jump to a common asset directory in the selected virtual filesystem")
	quickNavHelp.Importance = widget.LowImportance
	quickNavHeader := container.NewBorder(nil, nil, quickNavCaption, quickNavHelp)
	sourceNav := container.NewGridWithColumns(2,
		container.NewVBox(sourceCaption, ab.sourceSelect),
		container.NewVBox(quickNavHeader, ab.quickNavSelect),
	)
	locationSearch := container.NewGridWithColumns(2, ab.breadcrumbLabel, ab.searchEntry)
	ab.topBar = container.NewVBox(
		container.NewBorder(nil, nil, navButtons, favBtn, sourceNav),
		locationSearch,
		controlsRow,
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
	// Shared dispatcher — used by both OnSelected (row tap) and
	// OnBranchOpened (disclosure-arrow tap). Fyne's Tree fires one
	// or the other depending on exactly where the user clicked on
	// the row; wiring both to the same action means "click a folder
	// in the tree" reliably navigates the grid into it regardless
	// of which pixel the click landed on.
	navigateTreeEntry := func(id widget.TreeNodeID) {
		if ab.treeSyncing {
			return
		}
		entry, ok := ab.assets[id]
		if !ok {
			return
		}
		if entry.IsDir {
			switch entry.PK3Source {
			case "":
				ab.loadFS(entry.Path)
			case "VFS":
				ab.loadVFS(entry.Path)
			default:
				ab.loadGrid(entry)
			}
			return
		}
		if ab.onAssetSelected != nil {
			ab.onAssetSelected(entry)
		}
	}
	ab.tree.OnSelected = navigateTreeEntry
	ab.tree.OnBranchOpened = navigateTreeEntry

	ab.grid = container.NewGridWrap(fyne.NewSize(ab.iconSize, ab.iconSize+30)) // Init with size
	// Empty by default — status text alone looked like a failed render.
	// The action opens the same source chooser used by the toolbar control.
	ab.emptySelectButton = widget.NewButtonWithIcon("Select Asset Root", theme.FolderOpenIcon(), func() {
		ab.sourceSelect.Tapped(&fyne.PointEvent{})
	})
	ab.emptySelectButton.Importance = widget.HighImportance
	ab.emptyHeadline = widget.NewLabelWithStyle("No asset root selected", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	emptyBody := widget.NewLabel("Choose an asset root: GameData VFS, TextAssets, a favorite folder, or a PK3 archive.")
	emptyBody.Alignment = fyne.TextAlignCenter
	emptyBody.Wrapping = fyne.TextWrapWord
	emptyContent := container.NewVBox(
		ab.emptyHeadline,
		emptyBody,
		Gap(SpaceSM),
		container.NewCenter(ab.emptySelectButton),
	)
	ab.emptyState = container.NewPadded(container.NewCenter(NewTilePanel(emptyContent, TileOpts{
		FillAlpha: 7, StrokeAlpha: 28, Padded: true,
	})))

	// This label mirrors directory load/error results below the browser.
	ab.statusLabel = widget.NewLabel("")

	// Tree scroll gets an explicit MinSize. Without it, Fyne's HSplit
	// clamps the offset against child MinSizes.
	treeScroll := container.NewScroll(ab.tree)
	treeScroll.SetMinSize(fyne.NewSize(160, 0))
	gridHost := container.NewStack(container.NewScroll(ab.grid), ab.emptyState)
	split := container.NewHSplit(treeScroll, gridHost)
	split.SetOffset(0.3)

	ab.container = container.NewBorder(ab.topBar, ab.statusLabel, nil, nil, split)
	ab.updateEmptyState()
}

func (ab *AssetBrowser) updateEmptyState() {
	if ab.emptyState == nil {
		return
	}
	if ab.currentDir == nil && len(ab.assets) == 0 {
		ab.emptyState.Show()
	} else {
		ab.emptyState.Hide()
	}
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
	ab.currentDir = nil
	ab.grid.Objects = nil
	ab.updateEmptyState()

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
	if ab.breadcrumbLabel != nil {
		ab.breadcrumbLabel.SetText("PK3 / " + filepath.Base(pk3Path))
	}
	ab.tree.Refresh()
	ab.updateEmptyState()
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
	ab.updateEmptyState()
	// Clear any selected-item pointer — the widget's about to be
	// destroyed when we rebuild the grid.
	ab.selectedItem = nil
	ab.selectedAsset = nil

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
	Entry          *AssetEntry
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
	item.Entry = entry
	item.TypeBadge = typeBadge
	item.OnTapped = func() {
		// Highlight this item; clear any previous selection so only
		// one item shows the primary-tinted background.
		if ab.selectedItem != nil && ab.selectedItem != item {
			ab.selectedItem.SetSelected(false)
		}
		item.SetSelected(true)
		ab.selectedItem = item
		ab.selectedAsset = entry
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
	// Priority: embedded > on-disk PNG cache > VFS decode + cache.
	//
	// The embedded check comes first so a fresh Foundry install —
	// where the user hasn't pointed gamedata at any PK3s yet —
	// still renders class/weapon/attribute icons out of the box.
	// LoadGameIcon keys on the basename, so paths like
	// "gfx/hud/w_icon_a280" or "gfx/menus/alpha/icon_stats_a280"
	// both resolve by picking up a280.png (weapons/ or attributes/
	// subdir) from the embedded FS.
	if img, ok := LoadGameIcon(nil, path); ok {
		return staticPNGResource(filepath.Base(path)+".png", img)
	}

	// Callers sometimes hand us a base path without extension (e.g.
	// IconResolver.ResolveAttributeIcon returns "gfx/hud/chk_stealth"
	// and expects us to figure out the ext). When we get one, search
	// the VFS index for the first matching extension so the real
	// game asset renders instead of silently falling through.
	if filepath.Ext(path) == "" && ab.vfs != nil {
		for _, ext := range []string{".tga", ".png", ".jpg", ".jpeg"} {
			candidate := strings.ToLower(path + ext)
			if ab.vfs.Lookup(candidate) != nil {
				path = path + ext
				break
			}
		}
		// If still extension-less, the path is likely a SHADER name
		// (e.g. `models/players/t_yoda/mb2_icon_default`). Ask the
		// shader resolver to map it to the texture path the engine
		// would actually use, then probe extensions on THAT.
		if filepath.Ext(path) == "" && ab.shaderResolver != nil {
			if mapped := ab.shaderResolver.Resolve(path); mapped != "" {
				probe := mapped
				if filepath.Ext(probe) == "" {
					for _, ext := range []string{".tga", ".png", ".jpg", ".jpeg"} {
						candidate := strings.ToLower(probe + ext)
						if ab.vfs.Lookup(candidate) != nil {
							probe = probe + ext
							break
						}
					}
				}
				if filepath.Ext(probe) != "" {
					path = probe
				}
			}
		}
	}

	if ab.vfs == nil || filepath.Ext(path) == "" {
		return nil
	}
	source := ab.vfs.Lookup(path)
	if source == nil {
		return nil
	}
	// Namespace both caches by the winning asset, not its basename or
	// logical path alone. Source switches and re-indexed replacements
	// must not reuse another model/installation's pixels.
	identity := fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d\x00%d",
		source.FullPath, source.PK3Path, source.EntryName,
		source.Size, source.ModTime.UnixNano(), source.CRC32)
	hash := md5.Sum([]byte(identity))
	hashStr := hex.EncodeToString(hash[:])
	cachePath := filepath.Join(ab.ensureCacheDir(), hashStr+".png")

	if data, err := os.ReadFile(cachePath); err == nil {
		return fyne.NewStaticResource(hashStr+".png", data)
	}

	// 2. Load from VFS
	rc, err := ab.vfs.ReadFile(path)
	if err != nil {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	data, readErr := readRasterBytes(rc)
	closeErr := rc.Close()
	if readErr != nil || closeErr != nil {
		return nil
	}
	img, err := decodeByExt(ext, data)
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

	pngData := buf.Bytes()
	_ = os.WriteFile(cachePath, pngData, 0644)

	return fyne.NewStaticResource(hashStr+".png", pngData)
}

func (ab *AssetBrowser) isImageAsset(asset *AssetEntry) bool {
	return asset.Type == AssetTypeTexture || asset.Type == AssetTypeIcon || asset.Type == AssetTypeGFX
}

func (ab *AssetBrowser) loadImage(asset *AssetEntry) image.Image {
	if ab.vfs != nil && asset.PK3Source != "" && asset.PK3Source != "VFS" {
		// Try using VFS helper if it's a known PK3.
		rc, err := ab.vfs.ReadFile(asset.Path)
		if err == nil {
			return decodeImageReadCloser(rc, asset.Name)
		}
	} else if asset.PK3Source != "" && asset.PK3Source != "VFS" {
		// Legacy PK3 loading.
		reader, err := zip.OpenReader(asset.PK3Source)
		if err != nil {
			return nil
		}
		for _, f := range reader.File {
			if f.Name != asset.Path {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				_ = reader.Close()
				return nil
			}
			img := decodeImageReadCloser(rc, asset.Name)
			if err := reader.Close(); err != nil {
				return nil
			}
			return img
		}
		_ = reader.Close()
	} else if asset.PK3Source == "VFS" && ab.vfs != nil {
		rc, err := ab.vfs.ReadFile(asset.Path)
		if err == nil {
			return decodeImageReadCloser(rc, asset.Name)
		}
	}
	return nil
}

func decodeImageReadCloser(rc io.ReadCloser, filename string) image.Image {
	img := decodeImage(rc, filename)
	if err := rc.Close(); err != nil {
		return nil
	}
	return img
}

func decodeImage(r io.Reader, filename string) image.Image {
	data, err := readRasterBytes(r)
	if err != nil {
		return nil
	}
	img, _ := decodeByExt(filepath.Ext(filename), data)
	return img
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

func canonicalAssetPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	value = pathpkg.Clean("/" + strings.TrimLeft(value, "/"))
	if value == "/" {
		return ""
	}
	return strings.TrimPrefix(value, "/")
}

// resolveVFSPath returns the VFS-preserved casing and whether the target is a
// directory. Resolution is case-insensitive because Quake asset references
// are case-insensitive even when the host filesystem is not.
func (ab *AssetBrowser) resolveVFSPath(requested string) (string, bool, bool) {
	if ab.vfs == nil {
		return "", false, false
	}
	clean := canonicalAssetPath(requested)
	ab.vfs.mu.RLock()
	defer ab.vfs.mu.RUnlock()
	if clean == "" {
		return "", true, true
	}
	if source := ab.vfs.Index[strings.ToLower(clean)]; source != nil {
		return source.Path, false, true
	}
	for dir := range ab.vfs.Directories {
		if strings.EqualFold(dir, clean) {
			return dir, true, true
		}
	}
	return clean, false, false
}

func (ab *AssetBrowser) vfsHasIndex() bool {
	if ab.vfs == nil {
		return false
	}
	ab.vfs.mu.RLock()
	defer ab.vfs.mu.RUnlock()
	return len(ab.vfs.Index) > 0
}

func (ab *AssetBrowser) navigateToPath(requested string) {
	if ab.vfs == nil {
		ab.statusLabel.SetText("Path not found in VFS: " + canonicalAssetPath(requested))
		return
	}
	if !ab.vfsHasIndex() {
		if err := ab.vfs.Refresh(); err != nil {
			ab.statusLabel.SetText("Unable to index VFS: " + err.Error())
			return
		}
	}

	canonical, isDir, ok := ab.resolveVFSPath(requested)
	if !ok {
		ab.statusLabel.SetText("Path not found in VFS: " + canonicalAssetPath(requested))
		return
	}
	if isDir {
		ab.loadVFS(canonical)
		return
	}

	parent := canonicalAssetPath(pathpkg.Dir(canonical))
	ab.loadVFS(parent)
	for _, object := range ab.grid.Objects {
		item, ok := object.(*GridItem)
		if !ok || item.Entry == nil || !strings.EqualFold(item.Entry.Path, canonical) {
			continue
		}
		item.SetSelected(true)
		ab.selectedItem = item
		ab.selectedAsset = item.Entry
		if ab.onAssetSelected != nil {
			ab.onAssetSelected(item.Entry)
		}
		break
	}
}

func (ab *AssetBrowser) SetOnAssetSelected(f func(*AssetEntry)) { ab.onAssetSelected = f }
func (ab *AssetBrowser) SetOnAssetDouble(f func(*AssetEntry))   { ab.onAssetDouble = f } // New method

func (ab *AssetBrowser) GetSelectedAsset() *AssetEntry {
	return ab.selectedAsset
}

func (ab *AssetBrowser) Refresh() { ab.scanPK3Files() }

func (ab *AssetBrowser) loadVFS(requested string) {
	if ab.vfs == nil {
		return
	}

	// Ensure VFS is indexed (lazy load).
	if !ab.vfsHasIndex() {
		ab.statusLabel.SetText("Indexing Game Assets...")
		if err := ab.vfs.Refresh(); err != nil {
			ab.statusLabel.SetText("Unable to index VFS: " + err.Error())
			return
		}
	}

	path, isDir, ok := ab.resolveVFSPath(requested)
	if !ok || !isDir {
		ab.statusLabel.SetText("Path not found in VFS: " + canonicalAssetPath(requested))
		return
	}

	// Snapshot the VFS directory graph while it is stable, then build the
	// complete tree. Keeping all directory entries (rather than only the
	// current folder) makes tree expansion and Quick Nav selection agree.
	ab.vfs.mu.RLock()
	directories := make(map[string][]*AssetSource, len(ab.vfs.Directories))
	for dir, contents := range ab.vfs.Directories {
		directories[dir] = append([]*AssetSource(nil), contents...)
	}
	ab.vfs.mu.RUnlock()

	ab.currentPK3 = "VFS"
	ab.assets = make(map[string]*AssetEntry)
	ab.rootEntries = nil
	ab.grid.Objects = nil

	for _, contents := range directories {
		for _, src := range contents {
			if _, exists := ab.assets[src.Path]; exists {
				continue
			}
			pk3Source := src.PK3Path
			if src.IsDirectory {
				pk3Source = "VFS"
			}
			ab.assets[src.Path] = &AssetEntry{
				Name:      pathpkg.Base(src.Path),
				Path:      src.Path,
				Size:      src.Size,
				Type:      detectAssetType(src.Path),
				PK3Source: pk3Source,
				IsDir:     src.IsDirectory,
			}
		}
	}

	for dir, contents := range directories {
		children := make([]*AssetEntry, 0, len(contents))
		for _, src := range contents {
			if child := ab.assets[src.Path]; child != nil {
				children = append(children, child)
			}
		}
		ab.sortAssets(children)
		if dir == "" {
			ab.rootEntries = children
		} else if entry := ab.assets[dir]; entry != nil {
			entry.Children = children
		}
	}

	current := &AssetEntry{
		Name:      pathpkg.Base(path),
		Path:      path,
		IsDir:     true,
		PK3Source: "VFS",
		Children:  ab.rootEntries,
	}
	if path != "" {
		if entry := ab.assets[path]; entry != nil {
			current = entry
		} else {
			current.Children = nil
			for dir, children := range directories {
				if !strings.EqualFold(dir, path) {
					continue
				}
				for _, src := range children {
					if child := ab.assets[src.Path]; child != nil {
						current.Children = append(current.Children, child)
					}
				}
				break
			}
		}
	}

	ab.tree.Refresh()
	ab.loadGrid(current)
	if ab.breadcrumbLabel != nil {
		location := "VFS /"
		if path != "" {
			location += " " + path
		}
		ab.breadcrumbLabel.SetText(location)
	}
	ab.statusLabel.SetText(fmt.Sprintf("VFS: %s (%d items)", path, len(current.Children)))

	// Opening and selecting branches emits Tree callbacks. Suppress the
	// navigation dispatcher while reflecting this programmatic selection.
	ab.treeSyncing = true
	ab.tree.UnselectAll()
	if path != "" {
		parts := strings.Split(path, "/")
		for i := range parts {
			ab.tree.OpenBranch(strings.Join(parts[:i+1], "/"))
		}
		ab.tree.Select(path)
	}
	ab.treeSyncing = false
}

func (ab *AssetBrowser) loadFS(path string) {
	path = filepath.Clean(path)
	LogInfo("Navigating to FS: %s", path)
	ab.currentPK3 = "" // Not in a PK3
	ab.assets = make(map[string]*AssetEntry)
	ab.rootEntries = []*AssetEntry{}
	ab.currentDir = nil

	ab.grid.Objects = nil
	ab.grid.Refresh()
	ab.updateEmptyState()

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
	if ab.breadcrumbLabel != nil {
		ab.breadcrumbLabel.SetText("Files / " + path)
	}

	// Tell the tree its data changed — otherwise it sits empty until
	// the user drags the split divider (which forces a relayout).
	ab.tree.Refresh()

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
