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
	onSelected   func(string)
	currentPath  string
	selectedFile *AssetEntry

	// UI elements
	pathBar      *fyne.Container // clickable breadcrumb row
	sidebar      *widget.List
	selectButton *widget.Button
	cancelButton *widget.Button

	sources     []string
	initialPath string // New field to store the initial path
}

func NewCustomFilePicker(win fyne.Window, ab *AssetBrowser) *CustomFilePicker {
	cfp := &CustomFilePicker{
		window:  win,
		browser: ab,
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
				if cfp.selectedFile.PK3Source == "" {
					cfp.browser.loadFS(cfp.selectedFile.Path)
				} else {
					cfp.browser.loadGrid(cfp.selectedFile)
				}
				cfp.selectedFile = nil
				cfp.refreshPathBar()
				cfp.selectButton.Disable()
			} else {
				LogInfo("Returning file...")
				// Open file
				if cfp.onSelected != nil {
					cfp.onSelected(cfp.selectedFile.Path)
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

	// Sidebar Sources
	cfp.sources = []string{"Home", "Computer", "--- Locations ---"}

	// Helper to add sources from AssetBrowser's logic (reusing detection logic for consistent UI)
	homeDir, _ := os.UserHomeDir()
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

		homeDir, _ := os.UserHomeDir() // Ensure homeDir is available here too

		if s == "Home" {
			cfp.browser.loadFS(homeDir)
		} else if s == "Computer" {
			cfp.browser.loadFS("/")
		} else if strings.HasPrefix(s, "Cloud: ") {
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
		cfp.selectedFile = asset
		cfp.selectButton.Enable()
	})

	cfp.browser.SetOnAssetDouble(func(asset *AssetEntry) {
		if cfp.onSelected != nil {
			cfp.onSelected(asset.Path)
			cfp.window.Close()
		}
	})

	// Navigation toolbar — explicit Up / Home / Refresh buttons so users
	// aren't hunting for a ".. (Up)" folder entry in the grid. Uses Fyne's
	// built-in theme icons for portability across macOS/Windows/Linux.
	upBtn := widget.NewButtonWithIcon("Up", theme.NavigateBackIcon(), cfp.goUp)
	upBtn.Importance = widget.LowImportance
	homeBtn := widget.NewButtonWithIcon("Home", theme.HomeIcon(), func() {
		home, _ := os.UserHomeDir()
		cfp.browser.loadFS(home)
		cfp.refreshPathBar()
	})
	homeBtn.Importance = widget.LowImportance
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		if cfp.browser.currentDir != nil {
			cfp.browser.loadFS(cfp.browser.currentDir.Path)
			cfp.refreshPathBar()
		}
	})
	refreshBtn.Importance = widget.LowImportance

	// View mode toggle (grid/list) exposed here since ShowTopBar(false)
	// hid the in-browser control.
	viewToggle := widget.NewSelect([]string{"Grid", "List"}, func(mode string) {
		if mode == "List" {
			cfp.browser.viewMode = ViewModeList
		} else {
			cfp.browser.viewMode = ViewModeGrid
		}
		if cfp.browser.currentDir != nil {
			cfp.browser.loadGrid(cfp.browser.currentDir)
		}
	})
	viewToggle.Selected = "Grid"

	topNav := container.NewBorder(
		nil, nil,
		container.NewHBox(upBtn, homeBtn, refreshBtn),
		viewToggle,
		container.NewScroll(cfp.pathBar),
	)

	// Main Layout
	split := container.NewHSplit(
		container.NewBorder(widget.NewLabelWithStyle("Sources", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), nil, nil, nil, cfp.sidebar),
		container.NewBorder(topNav, nil, nil, nil, cfp.browser.GetContent()),
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
	path := "/"
	if cfp.browser.currentDir != nil && cfp.browser.currentDir.Path != "" {
		path = cfp.browser.currentDir.Path
	}

	// Build cumulative segments so each breadcrumb button knows the
	// full path up to and including itself.
	segments := strings.Split(strings.Trim(filepath.ToSlash(path), "/"), "/")
	cumulative := ""
	if strings.HasPrefix(path, "/") || len(segments) == 0 {
		// Root marker (Unix '/' or empty path).
		rootBtn := widget.NewButtonWithIcon("", theme.ComputerIcon(), func() {
			cfp.browser.loadFS("/")
			cfp.refreshPathBar()
		})
		rootBtn.Importance = widget.LowImportance
		cfp.pathBar.Add(rootBtn)
	}
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if cumulative == "" && !strings.HasPrefix(path, "/") {
			// Windows drive letter etc.
			cumulative = seg
		} else {
			cumulative = cumulative + "/" + seg
		}
		segPath := cumulative
		if !strings.HasPrefix(path, "/") && cumulative == seg {
			segPath = seg + string(filepath.Separator)
		}
		cfp.pathBar.Add(widget.NewLabel("›"))
		btn := widget.NewButton(seg, func() {
			cfp.browser.loadFS(segPath)
			cfp.refreshPathBar()
		})
		btn.Importance = widget.LowImportance
		cfp.pathBar.Add(btn)
	}
	cfp.pathBar.Refresh()
}

// goUp navigates one level up from the current directory, wrapping the
// various "up" cases (FS, PK3, VFS) so the button does the right thing
// regardless of where the user is.
func (cfp *CustomFilePicker) goUp() {
	if cfp.browser.currentDir == nil {
		return
	}
	parent := filepath.Dir(cfp.browser.currentDir.Path)
	if parent == "" || parent == "." {
		return
	}
	cfp.browser.loadFS(parent)
	cfp.refreshPathBar()
}

func (cfp *CustomFilePicker) Show(onSelected func(string)) {
	cfp.onSelected = onSelected
	cfp.selectedFile = nil
	cfp.selectButton.Disable()

	// Ensure decent initial size
	cfp.window.Resize(fyne.NewSize(900, 600))
	cfp.window.Show()

	// Default to Home or Computer if nothing loaded
	if cfp.browser.currentDir == nil {
		if cfp.initialPath != "" {
			cfp.browser.loadFS(cfp.initialPath)
		} else {
			home, _ := os.UserHomeDir()
			cfp.browser.loadFS(home)
		}
	}
	cfp.refreshPathBar()
}
