package main

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type CustomFilePicker struct {
	window       fyne.Window
	browser      *AssetBrowser
	onSelected   func(*AssetEntry)
	selectedFile *AssetEntry

	// UI elements
	pathBar      *fyne.Container // clickable breadcrumb row
	pathScroll   *container.Scroll
	sidebar      *widget.List
	selectButton *widget.Button
	cancelButton *widget.Button

	// syncFavBtn keeps the "Favorite/Unfavorite" button in the nav
	// bar in step with whatever folder the browser is currently in.
	// Called from refreshPathBar on every folder change + directly
	// from the button's own handler after toggling state.
	syncFavBtn func()

	sources     []string
	initialPath string // New field to store the initial path
	virtualOnly bool
}

func NewCustomFilePicker(win fyne.Window, ab *AssetBrowser) *CustomFilePicker {
	return newCustomFilePicker(win, ab, false)
}

// NewVirtualFilePicker creates the standard asset picker restricted to the
// merged game VFS. The mode is fixed before UI construction so it never
// enumerates or exposes host filesystem locations.
func NewVirtualFilePicker(win fyne.Window, ab *AssetBrowser) *CustomFilePicker {
	return newCustomFilePicker(win, ab, true)
}

func newCustomFilePicker(win fyne.Window, ab *AssetBrowser, virtualOnly bool) *CustomFilePicker {
	cfp := &CustomFilePicker{
		window:      win,
		browser:     ab,
		virtualOnly: virtualOnly,
	}
	cfp.createUI()
	return cfp
}

func (cfp *CustomFilePicker) SetInitialPath(path string) {
	cfp.initialPath = path
}

func (cfp *CustomFilePicker) createUI() {
	cfp.browser.ShowTopBar(false) // Hide internal navigation

	cfp.pathBar = container.NewHBox()
	cfp.refreshPathBar()

	cfp.selectButton = widget.NewButton("Open", func() {
		if cfp.selectedFile != nil {
			LogInfo("Open Clicked. Selected: %s, IsDir: %v, Path: %s", cfp.selectedFile.Name, cfp.selectedFile.IsDir, cfp.selectedFile.Path)

			if cfp.selectedFile.IsDir || cfp.selectedFile.Name == ".." || cfp.selectedFile.Name == "(Parent)" {
				LogInfo("Navigating directory...")
				// Navigate into directory
				cfp.browser.openDirectory(cfp.selectedFile)
			} else {
				LogInfo("Returning file...")
				// Open file — pass the whole AssetEntry so the caller
				// can tell a filesystem file apart from a PK3-embedded
				// one (different read strategies). Passing just Path
				// lost the PK3Source and caused "file does not exist"
				// when the picker was sitting inside a .pk3.
				if cfp.onSelected != nil {
					cfp.onSelected(cfp.selectedFile)
					cfp.window.Close()
				}
			}
		}
	})
	cfp.selectButton.Disable()
	cfp.selectButton.Importance = widget.HighImportance

	cfp.cancelButton = widget.NewButton("Cancel", func() {
		cfp.window.Close()
	})

	// Initial source list. Rebuilt in-place by rebuildSources so we
	// don't double-maintain the construction logic.
	cfp.rebuildSources()

	cfp.sidebar = widget.NewList(
		func() int { return len(cfp.sources) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.FolderIcon()), widget.NewLabel("Template"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			text := cfp.sources[id]
			label := obj.(*fyne.Container).Objects[1].(*widget.Label)
			icon := obj.(*fyne.Container).Objects[0].(*widget.Icon)

			label.SetText(text)

			if strings.HasPrefix(text, "---") {
				label.TextStyle = fyne.TextStyle{Bold: true}
				icon.Hide()
			} else {
				label.TextStyle = fyne.TextStyle{}
				icon.Show()
				if text == "Home" {
					icon.SetResource(theme.HomeIcon())
				} else if text == "Computer" {
					icon.SetResource(theme.ComputerIcon())
				} else if strings.HasPrefix(text, "Cloud:") {
					icon.SetResource(theme.StorageIcon())
				} else if strings.HasPrefix(text, "Volume:") {
					icon.SetResource(theme.StorageIcon())
				} else if strings.HasPrefix(text, "Workspace:") {
					icon.SetResource(theme.FolderOpenIcon())
				} else {
					icon.SetResource(theme.FolderIcon())
				}
			}
		},
	)

	cfp.sidebar.OnSelected = func(id widget.ListItemID) {
		s := cfp.sources[id]
		if strings.HasPrefix(s, "---") {
			return
		}

		if s == "All Game Data (Virtual)" {
			cfp.browser.loadVFS("")
		} else if cfp.virtualOnly {
			return
		} else if s == "TextAssets" {
			cfp.browser.loadFS(cfp.browser.textAssetsPath)
		} else if s == "Home" {
			homeDir, _ := os.UserHomeDir()
			cfp.browser.loadFS(homeDir)
		} else if s == "Computer" {
			cfp.browser.loadFS("/")
		} else if strings.HasPrefix(s, "Cloud: ") {
			homeDir, _ := os.UserHomeDir()
			name := strings.TrimPrefix(s, "Cloud: ")
			cfp.browser.loadFS(filepath.Join(homeDir, "Library", "CloudStorage", name))
		} else if strings.HasPrefix(s, "Volume: ") {
			name := strings.TrimPrefix(s, "Volume: ")
			cfp.browser.loadFS(filepath.Join("/Volumes", name))
		} else if strings.HasPrefix(s, "Workspace: ") {
			if cfp.browser.gamedataPath != "" {
				cfp.browser.loadFS(filepath.Dir(cfp.browser.gamedataPath))
			}
		} else {
			// Check Favorites
			isFav := false
			for _, f := range cfp.browser.favorites {
				if f == s {
					cfp.browser.loadFS(f)
					isFav = true
					break
				}
			}
			if isFav {
				return
			}

			// Check PK3s
			for _, p := range cfp.browser.pk3Files {
				if filepath.Base(p) == s {
					cfp.browser.loadPK3(p)
					return
				}
			}
		}
	}

	// Configure Browser interactions
	cfp.browser.SetOnAssetSelected(func(asset *AssetEntry) {
		if cfp.virtualOnly && cfp.browser.currentPK3 != "VFS" {
			asset = nil
		}
		cfp.selectedFile = asset
		if asset == nil {
			cfp.selectButton.Disable()
		} else {
			cfp.selectButton.Enable()
		}
	})

	cfp.browser.SetOnAssetDouble(func(asset *AssetEntry) {
		if asset == nil || asset.IsDir || cfp.virtualOnly && cfp.browser.currentPK3 != "VFS" {
			return
		}
		if cfp.onSelected != nil {
			cfp.onSelected(asset)
			cfp.window.Close()
		}
	})
	cfp.browser.onDirectoryChanged = cfp.refreshPathBar

	// Navigation toolbar — explicit Up / Home / Refresh buttons so users
	// aren't hunting for a ".. (Up)" folder entry in the grid. Uses Fyne's
	// built-in theme icons for portability across macOS/Windows/Linux.
	// Up uses MoveUpIcon (↑) — NavigateBackIcon (←) previously confused
	// the direction since "up a directory" and "back through history"
	// are distinct concepts.
	upBtn := widget.NewButtonWithIcon("Up", theme.MoveUpIcon(), cfp.goUp)
	upBtn.Importance = widget.LowImportance
	homeLabel, homeIcon := "Home", theme.HomeIcon()
	homeAction := func() {
		home, _ := os.UserHomeDir()
		cfp.browser.loadFS(home)
	}
	if cfp.virtualOnly {
		homeLabel, homeIcon = "Root", theme.StorageIcon()
		homeAction = func() { cfp.browser.loadVFS("") }
	}
	homeBtn := widget.NewButtonWithIcon(homeLabel, homeIcon, homeAction)
	homeBtn.Importance = widget.LowImportance
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), cfp.browser.refreshCurrentDirectory)
	refreshBtn.Importance = widget.LowImportance

	// View mode: icon toggle (grid ↔ list) instead of a dropdown.
	// Matches the pattern the main sidebar uses. We keep a single
	// button whose icon swaps to reflect the *current* state, so
	// users see "I'm in grid mode" at a glance. Clicking flips the
	// mode and the icon together.
	var viewBtn *widget.Button
	syncViewBtn := func() {
		if cfp.browser.viewMode == ViewModeList {
			viewBtn.SetIcon(theme.GridIcon()) // icon shows what pressing would switch TO
			viewBtn.SetText("Grid")
		} else {
			viewBtn.SetIcon(theme.ListIcon())
			viewBtn.SetText("List")
		}
	}
	viewBtn = widget.NewButtonWithIcon("", theme.ListIcon(), func() {
		if cfp.browser.viewMode == ViewModeList {
			cfp.browser.viewMode = ViewModeGrid
		} else {
			cfp.browser.viewMode = ViewModeList
		}
		if cfp.browser.currentDir != nil {
			cfp.browser.loadGrid(cfp.browser.currentDir)
		}
		syncViewBtn()
	})
	viewBtn.Importance = widget.LowImportance
	syncViewBtn()

	// Sort bar — exposes the same sort modes the main asset browser
	// has so users can order by name/size/type without leaving the
	// file picker. The picker hides ShowTopBar(false) so we re-
	// expose sort here explicitly. Reload the grid on change so the
	// new order applies immediately.
	sortSelect := widget.NewSelect([]string{
		string(SortNameAsc), string(SortNameDesc),
		string(SortSizeAsc), string(SortSizeDesc),
		string(SortType),
	}, func(s string) {
		cfp.browser.sortMode = SortMode(s)
		if cfp.browser.currentDir != nil {
			cfp.browser.loadGrid(cfp.browser.currentDir)
		}
	})
	sortSelect.SetSelected(string(cfp.browser.sortMode))

	sortLabel := widget.NewLabelWithStyle("Sort",
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// Favorite toggle — adds the current directory to the sidebar's
	// Favorites section (AssetBrowser.addToFavorites persists via
	// favorites.json), or removes it if already pinned. Icon swaps
	// between filled (favorited) and outline (not) to communicate
	// current state. Persisted list surfaces in the sidebar + also
	// in the main editor's browser, so pinning here benefits both.
	var favBtn *widget.Button
	syncFavBtn := func() {
		if cfp.virtualOnly {
			favBtn.Disable()
			return
		}
		if cfp.browser.currentPK3 != "" || cfp.browser.currentDir == nil || cfp.browser.currentDir.Path == "" {
			favBtn.Disable()
			favBtn.SetIcon(theme.FolderOpenIcon())
			favBtn.SetText("")
			return
		}
		favBtn.Enable()
		if cfp.browser.IsFavorited(cfp.browser.currentDir.Path) {
			favBtn.SetIcon(theme.ConfirmIcon())
			favBtn.SetText("Unfavorite")
		} else {
			favBtn.SetIcon(theme.ContentAddIcon())
			favBtn.SetText("Favorite")
		}
	}
	favBtn = widget.NewButtonWithIcon("Favorite", theme.ContentAddIcon(), func() {
		if cfp.browser.currentPK3 != "" || cfp.browser.currentDir == nil || cfp.browser.currentDir.Path == "" {
			return
		}
		path := cfp.browser.currentDir.Path
		if cfp.browser.IsFavorited(path) {
			cfp.browser.removeFromFavorites(path)
		} else {
			cfp.browser.addToFavorites(path)
		}
		cfp.rebuildSources()
		syncFavBtn()
	})
	favBtn.Importance = widget.LowImportance
	syncFavBtn()
	// Directory changes from the tree, grid, toolbar, and sources all
	// refresh the breadcrumb and favorite toggle through the browser.
	cfp.syncFavBtn = syncFavBtn

	actions := []fyne.CanvasObject{
		upBtn, homeBtn, refreshBtn,
		container.NewHBox(sortLabel, sortSelect),
	}
	if !cfp.virtualOnly {
		actions = append(actions, favBtn)
	}
	actions = append(actions, viewBtn)
	actionRow := newSourceActionRow(actions...)

	// Keep the path on a dedicated horizontal viewport so the action controls
	// cannot consume its width. The copy button stays outside the viewport and
	// reads the browser's live, untruncated location label on every tap.
	cfp.pathScroll = container.NewHScroll(cfp.pathBar)
	pathRow := container.NewBorder(nil, nil, nil,
		assetBrowserCopyTextButton(cfp.browser.breadcrumbLabel, "Copy full picker location"),
		cfp.pathScroll,
	)
	topNav := newFormColumns(1,
		actionRow,
		pathRow,
	)

	// Main Layout
	split := container.NewHSplit(
		container.NewBorder(widget.NewLabelWithStyle("Sources", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, cfp.sidebar),
		newEditorContent(topNav, cfp.browser.GetContent()),
	)
	split.SetOffset(0.25)

	bottomBar := container.NewBorder(nil, nil, nil, container.NewHBox(cfp.cancelButton, cfp.selectButton))

	cfp.window.SetContent(container.NewBorder(nil, bottomBar, nil, nil, split))
}

// refreshPathBar rebuilds the breadcrumb from the browser's current
// directory. Each path segment becomes a clickable button so users can
// jump up any number of levels in one click.
func (cfp *CustomFilePicker) refreshPathBar() {
	cfp.pathBar.Objects = nil
	addButton := func(label string, icon fyne.Resource, tapped func()) {
		if len(cfp.pathBar.Objects) > 0 {
			cfp.pathBar.Add(widget.NewLabel("›"))
		}
		btn := widget.NewButtonWithIcon(label, icon, tapped)
		btn.Importance = widget.LowImportance
		cfp.pathBar.Add(btn)
	}

	source := cfp.browser.currentPK3
	if source != "" {
		label := filepath.Base(source)
		if source == "VFS" {
			label = "All Game Data"
		}
		addButton(label, theme.StorageIcon(), func() {
			cfp.browser.openDirectory(&AssetEntry{IsDir: true, PK3Source: source})
		})
		if dir := cfp.browser.currentDir; dir != nil {
			cumulative := ""
			for _, segment := range strings.Split(dir.Path, "/") {
				if segment == "" {
					continue
				}
				if cumulative != "" {
					cumulative += "/"
				}
				cumulative += segment
				path := cumulative
				addButton(segment, nil, func() {
					cfp.browser.openDirectory(&AssetEntry{Path: path, IsDir: true, PK3Source: source})
				})
			}
		}
	} else if cfp.virtualOnly {
		addButton("All Game Data", theme.StorageIcon(), func() { cfp.browser.loadVFS("") })
	} else if dir := cfp.browser.currentDir; dir != nil {
		path := filepath.Clean(dir.Path)
		volume := filepath.VolumeName(path)
		root := volume + string(filepath.Separator)
		addButton(root, theme.ComputerIcon(), func() { cfp.browser.loadFS(root) })
		cumulative := root
		for _, segment := range strings.Split(filepath.ToSlash(strings.TrimPrefix(path, root)), "/") {
			if segment == "" {
				continue
			}
			cumulative = filepath.Join(cumulative, segment)
			target := cumulative
			addButton(segment, nil, func() { cfp.browser.loadFS(target) })
		}
	} else {
		cfp.pathBar.Add(widget.NewLabel("Select a source"))
	}
	cfp.pathBar.Refresh()
	if cfp.pathScroll != nil {
		// New locations should open on the current (trailing) crumb. Ancestors
		// remain reachable by scrolling the viewport back toward the start.
		maxOffset := fyne.Max(0, cfp.pathBar.MinSize().Width-cfp.pathScroll.Size().Width)
		cfp.pathScroll.ScrollToOffset(fyne.NewPos(maxOffset, 0))
	}
	if cfp.syncFavBtn != nil {
		cfp.syncFavBtn()
	}
}

// rebuildSources regenerates the sidebar's Sources list from current
// state — standard locations + cloud drives + volumes + workspace +
// favorites + PK3s. Called at init and whenever favorites change so
// a newly-pinned folder shows up in the sidebar immediately.
func (cfp *CustomFilePicker) rebuildSources() {
	if cfp.virtualOnly {
		cfp.sources = []string{"All Game Data (Virtual)"}
		if cfp.sidebar != nil {
			cfp.sidebar.Refresh()
		}
		return
	}
	homeDir, _ := os.UserHomeDir()

	cfp.sources = []string{"All Game Data (Virtual)", "Home", "Computer"}
	if cfp.browser.textAssetsPath != "" {
		cfp.sources = append(cfp.sources, "TextAssets")
	}
	cfp.sources = append(cfp.sources, "--- Locations ---")

	cloudStoragePath := filepath.Join(homeDir, "Library", "CloudStorage")
	if entries, err := os.ReadDir(cloudStoragePath); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				cfp.sources = append(cfp.sources, "Cloud: "+e.Name())
			}
		}
	}
	if entries, err := os.ReadDir("/Volumes"); err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "Macintosh HD" {
				cfp.sources = append(cfp.sources, "Volume: "+e.Name())
			}
		}
	}
	if cfp.browser.gamedataPath != "" {
		workspace := filepath.Dir(cfp.browser.gamedataPath)
		cfp.sources = append(cfp.sources, "Workspace: "+filepath.Base(workspace))
	}

	if len(cfp.browser.favorites) > 0 {
		cfp.sources = append(cfp.sources, "--- Favorites ---")
		cfp.sources = append(cfp.sources, cfp.browser.favorites...)
	}

	cfp.sources = append(cfp.sources, "--- PK3s ---")
	pk3Names := make([]string, len(cfp.browser.pk3Files))
	for i, p := range cfp.browser.pk3Files {
		pk3Names[i] = filepath.Base(p)
	}
	cfp.sources = append(cfp.sources, pk3Names...)

	if cfp.sidebar != nil {
		cfp.sidebar.Refresh()
	}
}

// goUp navigates one level up from the current directory, wrapping the
// various "up" cases (FS, PK3, VFS) so the button does the right thing
// regardless of where the user is.
func (cfp *CustomFilePicker) goUp() {
	if cfp.virtualOnly && cfp.browser.currentPK3 != "VFS" {
		cfp.browser.loadVFS("")
		return
	}
	cfp.browser.goUp()
}

func (cfp *CustomFilePicker) Show(onSelected func(*AssetEntry)) {
	cfp.onSelected = onSelected
	cfp.browser.clearSelection()

	// Ensure decent initial size
	cfp.window.Resize(fyne.NewSize(1200, 780))
	cfp.window.Show()

	// Default to Home or Computer if nothing loaded
	if cfp.browser.currentDir == nil {
		if cfp.virtualOnly {
			initial := cfp.initialPath
			if filepath.IsAbs(initial) || strings.HasPrefix(strings.ReplaceAll(initial, "\\", "/"), "/") || strings.Contains(initial, ":") || hasParentPathSegment(initial) {
				initial = ""
			}
			cfp.browser.navigateToPath(initial)
			if cfp.browser.currentPK3 != "VFS" {
				cfp.browser.loadVFS("")
			}
		} else if cfp.initialPath != "" {
			if filepath.IsAbs(cfp.initialPath) {
				cfp.browser.loadFS(cfp.initialPath)
			} else {
				cfp.browser.navigateToPath(cfp.initialPath)
			}
		} else {
			home, _ := os.UserHomeDir()
			cfp.browser.loadFS(home)
		}
	}
	cfp.refreshPathBar()
}

func hasParentPathSegment(value string) bool {
	for _, segment := range strings.Split(strings.ReplaceAll(value, "\\", "/"), "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}
