package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type Modpack struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	Path        string    `json:"path"` // Absolute path to project root
	LastEdited  time.Time `json:"last_edited"`
}

type ModpackManager struct {
	app       *App
	container *fyne.Container
	projects  []*Modpack

	// UI
	projectList *widget.List
	detailView  *fyne.Container
}

func NewModpackManager(app *App) *ModpackManager {
	mm := &ModpackManager{
		app:      app,
		projects: []*Modpack{},
	}
	mm.loadProjects()
	mm.createUI()
	return mm
}

func (mm *ModpackManager) loadProjects() {
	if mm.app.configPath == "" {
		return
	}
	configPath := filepath.Join(filepath.Dir(mm.app.configPath), "fa_projects.json")
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &mm.projects)
	}
}

func (mm *ModpackManager) saveProjects() {
	if mm.app.configPath == "" {
		return
	}
	configPath := filepath.Join(filepath.Dir(mm.app.configPath), "fa_projects.json")
	data, _ := json.MarshalIndent(mm.projects, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func (mm *ModpackManager) createUI() {
	// List
	mm.projectList = widget.NewList(
		func() int { return len(mm.projects) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.FolderIcon()), widget.NewLabel("Project Name"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			p := mm.projects[id]
			obj.(*fyne.Container).Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s (%s)", p.Name, p.Version))
		},
	)
	mm.projectList.OnSelected = func(id widget.ListItemID) {
		mm.showProjectDetails(mm.projects[id])
	}

	// Toolbar
	newBtn := widget.NewButtonWithIcon("New Project", theme.ContentAddIcon(), mm.showNewProjectDialog)
	importBtn := widget.NewButtonWithIcon("Import Folder", theme.FolderOpenIcon(), mm.importProject)

	// Details Placeholder
	mm.detailView = container.NewMax(widget.NewLabel("Select a project"))

	// Layout
	leftPane := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("My Modpacks", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), newBtn, importBtn),
		nil, nil, nil,
		mm.projectList,
	)

	split := container.NewHSplit(leftPane, mm.detailView)
	split.SetOffset(0.3)

	mm.container = container.NewMax(split)
}

func (mm *ModpackManager) GetContent() fyne.CanvasObject {
	return mm.container
}

func (mm *ModpackManager) showNewProjectDialog() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("MyCoolMod")

	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("C:/MBII_Dev/MyCoolMod")

	browseBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				pathEntry.SetText(filepath.Join(uri.Path(), nameEntry.Text))
			}
		}, mm.app.mainWindow)
	})

	dialog.ShowForm("Create New Modpack", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Location", container.NewBorder(nil, nil, nil, browseBtn, pathEntry)),
	}, func(confirm bool) {
		if confirm && nameEntry.Text != "" && pathEntry.Text != "" {
			mm.createProject(nameEntry.Text, pathEntry.Text)
		}
	}, mm.app.mainWindow)
}

func (mm *ModpackManager) createProject(name, path string) {
	// Create directory structure
	dirs := []string{
		"ext_data/mb2/character",
		"ext_data/sabers",
		"ext_data/vehicles",
		"maps",
		"shaders",
		"models/players",
		"gfx/hud",
	}

	for _, d := range dirs {
		os.MkdirAll(filepath.Join(path, d), 0755)
	}

	// Create project meta file
	p := &Modpack{
		Name:       name,
		Version:    "0.1",
		Path:       path,
		LastEdited: time.Now(),
	}

	mm.projects = append(mm.projects, p)
	mm.saveProjects()
	mm.projectList.Refresh()
	mm.projectList.Select(len(mm.projects) - 1)

	dialog.ShowInformation("Success", "Project created successfully!\nFolders initialized.", mm.app.mainWindow)
}

func (mm *ModpackManager) importProject() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if uri != nil {
			path := uri.Path()
			name := filepath.Base(path)
			p := &Modpack{
				Name:       name,
				Path:       path,
				LastEdited: time.Now(),
			}
			mm.projects = append(mm.projects, p)
			mm.saveProjects()
			mm.projectList.Refresh()
		}
	}, mm.app.mainWindow)
}

func (mm *ModpackManager) showProjectDetails(p *Modpack) {
	nameLabel := widget.NewLabelWithStyle(p.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	pathLabel := widget.NewLabel(p.Path)

	openBtn := widget.NewButtonWithIcon("Open in Editor", theme.LoginIcon(), func() {
		// Set app context to this project
		mm.app.config.LastOpenDir = filepath.Join(p.Path, "ext_data/mb2/character") // Default to char folder
		// This should probably refresh the editor's loaded file list based on new config
		// For now, just update the config. AssetBrowser needs to rescan.
		mm.app.config.GamedataPath = filepath.Join(p.Path, "GameData") // Assuming GameData is inside modpack
		mm.app.assetBrowser.gamedataPath = mm.app.config.GamedataPath
		mm.app.assetBrowser.Refresh() // Rescan PK3s
		mm.app.updateStatus("Switched workspace to: " + p.Name)
	})

	shareBtn := widget.NewButtonWithIcon("Share / Export Source", theme.MailAttachmentIcon(), func() {
		mm.shareProject(p)
	})

	buildBtn := widget.NewButtonWithIcon("Build PK3", theme.DownloadIcon(), func() {
		// Call main build function (not yet implemented in main.go but we can stub)
		pk3Path := filepath.Join(p.Path, "..", p.Name+".pk3")
		// mm.app.buildPK3(p.Path, pk3Path) // Stub
		fmt.Println("Building PK3 to", pk3Path)
	})

	form := widget.NewForm(
		widget.NewFormItem("Version", widget.NewEntry()), // Bind to p.Version
		widget.NewFormItem("Author", widget.NewEntry()),
		widget.NewFormItem("Description", widget.NewMultiLineEntry()),
	)
	// (Binding logic omitted for brevity, assume updates p)

	actions := container.NewHBox(openBtn, shareBtn, buildBtn)

	content := container.NewVBox(
		nameLabel,
		pathLabel,
		widget.NewSeparator(),
		actions,
		widget.NewSeparator(),
		form,
	)

	mm.detailView.Objects = []fyne.CanvasObject{container.NewPadded(content)}
	mm.detailView.Refresh()
}

func (mm *ModpackManager) shareProject(p *Modpack) {
	// Zip the SOURCE folder (not the PK3)
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			ShowError(fmt.Errorf("Failed to save file: %v", err), mm.app.mainWindow)
			return
		}
		if writer == nil {
			return
		} // Cancelled

		// destPath := writer.URI().Path()
		// Call python zip logic or internal zip
	}, mm.app.mainWindow)
}
