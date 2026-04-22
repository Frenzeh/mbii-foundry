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
	pathLabel    *widget.Label
	sidebar      *widget.List
	selectButton *widget.Button
	cancelButton *widget.Button
	
	sources      []string
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

	cfp.pathLabel = widget.NewLabel("Current Path: /")
	cfp.pathLabel.TextStyle = fyne.TextStyle{Monospace: true}
	
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
				cfp.pathLabel.SetText("Current Path: " + cfp.browser.currentDir.Path)
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
		for _, e := range entries { if e.IsDir() { cfp.sources = append(cfp.sources, "Cloud: "+e.Name()) } }
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
	for i, p := range cfp.browser.pk3Files { pk3Names[i] = filepath.Base(p) }
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
		if strings.HasPrefix(s, "---") { return }
		
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
			if isFav { return }

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
		cfp.pathLabel.SetText("Selected: " + asset.Name)
		cfp.selectedFile = asset
		cfp.selectButton.Enable()
	})
	
	cfp.browser.SetOnAssetDouble(func(asset *AssetEntry) {
		if cfp.onSelected != nil {
			cfp.onSelected(asset.Path)
			cfp.window.Close()
		}
	})

	// Main Layout
	split := container.NewHSplit(
		container.NewBorder(widget.NewLabelWithStyle("Sources", fyne.TextAlignLeading, fyne.TextStyle{Bold:true}), nil, nil, nil, cfp.sidebar),
		cfp.browser.GetContent(),
	)
	split.SetOffset(0.25)

	bottomBar := container.NewBorder(nil, nil, cfp.pathLabel, container.NewHBox(cfp.cancelButton, cfp.selectButton))

	cfp.window.SetContent(container.NewBorder(nil, bottomBar, nil, nil, split))
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
			cfp.pathLabel.SetText("Current Path: " + cfp.initialPath)
		} else {
			home, _ := os.UserHomeDir()
			cfp.browser.loadFS(home)
		}
	}
}