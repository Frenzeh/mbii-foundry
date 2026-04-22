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
	"fyne.io/fyne/v2/layout"
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
	newBtn := widget.NewButtonWithIcon("New", theme.ContentAddIcon(), mm.showNewProjectDialog)
	newBtn.Importance = widget.HighImportance
	importBtn := widget.NewButtonWithIcon("Import", theme.FolderOpenIcon(), mm.importProject)
	helpBtn := widget.NewButtonWithIcon("", theme.HelpIcon(), mm.showHelp)
	helpBtn.Importance = widget.LowImportance

	// Details Placeholder — compact empty state. The full primer lives
	// in a modal (Learn more button below, plus the ? in the toolbar).
	// Embedding the markdown inline made the sidebar a scrolling mess
	// because this whole Modpacks activity is itself narrow sidebar chrome.
	mm.detailView = container.NewStack(mm.emptyDetailView())

	// Layout. Action row only — the "MODPACKS" heading is supplied by
	// the sidebar wrapper in main.go so every activity gets the same
	// consistent header treatment. Duplicating it here meant users saw
	// MODPACKS printed twice.
	header := container.NewVBox(
		container.NewHBox(newBtn, importBtn, layout.NewSpacer(), helpBtn),
		widget.NewSeparator(),
	)
	leftPane := container.NewBorder(header, nil, nil, nil, mm.projectList)

	split := container.NewHSplit(leftPane, mm.detailView)
	split.SetOffset(0.3)

	mm.container = container.NewStack(split)
}

// emptyDetailView is the compact right-pane placeholder when no
// modpack is selected. Deliberately short — the Modpacks activity is
// rendered inside the narrow sidebar, so dumping the full primer here
// turns into an awkward vertical scroll. A "Learn more" button opens
// the modal for users who want the details.
func (mm *ModpackManager) emptyDetailView() fyne.CanvasObject {
	headline := widget.NewLabelWithStyle("No modpack selected", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	hint := widget.NewLabel("Pick one from the list, or create a new one above.")
	hint.Wrapping = fyne.TextWrapWord

	learnBtn := widget.NewButtonWithIcon("Learn more", theme.HelpIcon(), mm.showHelp)
	learnBtn.Importance = widget.LowImportance

	return container.NewPadded(container.NewVBox(
		headline,
		hint,
		widget.NewSeparator(),
		learnBtn,
	))
}

// helpContent renders the primer shown in the help modal. Long-form
// content — deliberately not used inline anywhere.
func (mm *ModpackManager) helpContent() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("What is a modpack?", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.TextStyle.Bold = true

	body := widget.NewRichTextFromMarkdown(`A **modpack** is a folder of custom MBII content you're working on —
characters, sabers, vehicles, and siege classes — bundled into a
single project so you can build it into a ` + "`.pk3`" + ` for the game.

**Typical layout** (Foundry creates this for you):

- ` + "`ext_data/mb2/character/`" + ` — ` + "`.mbch`" + ` class files
- ` + "`ext_data/sabers/`" + ` — ` + "`.sab`" + ` saber files
- ` + "`ext_data/vehicles/`" + ` — ` + "`.veh`" + ` vehicle files
- ` + "`models/`, `shaders/`, `gfx/`" + ` — art assets

**Workflow:**

1. **New** — pick a name + folder; Foundry scaffolds the dirs.
2. **Edit** — open files with Foundry's editors; the modpack folder
   becomes your working directory.
3. **Build PK3** — packs the folder into a ` + "`.pk3`" + ` you can drop
   into ` + "`GameData/MBII/`" + ` or share with testers.
4. **Share / Export Source** — zips the raw source for other devs to
   open in their own Foundry.

Modpacks are just folders on your disk. Deleting one from this list
only removes it from Foundry's project history — your files on disk
stay put. To remove the actual folder, do it in Finder/Explorer.`)
	body.Wrapping = fyne.TextWrapWord

	return container.NewPadded(container.NewVBox(title, widget.NewSeparator(), body))
}

// showHelp opens the primer as a modal dialog — same content as the
// default detail view, handy when a project is already selected.
func (mm *ModpackManager) showHelp() {
	d := dialog.NewCustom("About Modpacks", "Close", mm.helpContent(), mm.app.mainWindow)
	d.Resize(fyne.NewSize(560, 520))
	d.Show()
}

func (mm *ModpackManager) GetContent() fyne.CanvasObject {
	return mm.container
}

func (mm *ModpackManager) showNewProjectDialog() {
	nameEntry := NewInputEntry()
	nameEntry.SetPlaceHolder("MyCoolMod")

	pathEntry := NewInputEntry()
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

	// Remove-from-list button. Danger importance + explicit confirm
	// dialog so a stray click can't nuke a modpack. We only scrub the
	// entry from Foundry's project list — files on disk are untouched
	// (made explicit in the confirm text so users don't panic).
	removeBtn := widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), func() {
		mm.confirmRemove(p)
	})
	removeBtn.Importance = widget.DangerImportance

	form := widget.NewForm(
		widget.NewFormItem("Version", NewInputEntry()), // Bind to p.Version
		widget.NewFormItem("Author", NewInputEntry()),
		widget.NewFormItem("Description", NewMultiLineInputEntry()),
	)
	// (Binding logic omitted for brevity, assume updates p)

	actions := container.NewHBox(openBtn, shareBtn, buildBtn, layout.NewSpacer(), removeBtn)

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

// confirmRemove pops a confirmation dialog before scrubbing a modpack
// from Foundry's saved project list. We deliberately do NOT offer to
// delete the folder on disk — way too destructive for a single button,
// and users may still want the files even if they're done tracking
// the project in Foundry. Removing a project here is always
// reversible by re-importing the same folder.
func (mm *ModpackManager) confirmRemove(p *Modpack) {
	msg := fmt.Sprintf(
		"Remove %q from Foundry's modpack list?\n\n"+
			"Files on disk will NOT be deleted.\n"+
			"Folder: %s\n\n"+
			"To delete the actual folder, use Finder/Explorer after removing here.",
		p.Name, p.Path)
	dialog.ShowConfirm("Remove Modpack", msg, func(confirmed bool) {
		if !confirmed {
			return
		}
		// Find and splice the entry. We match by pointer — each Modpack
		// in the slice is a distinct *Modpack, so this is exact.
		for i, other := range mm.projects {
			if other == p {
				mm.projects = append(mm.projects[:i], mm.projects[i+1:]...)
				break
			}
		}
		mm.saveProjects()
		mm.projectList.Refresh()
		mm.projectList.UnselectAll()
		// Reset the right pane back to the empty state so the detail area
		// doesn't keep showing a now-gone project.
		mm.detailView.Objects = []fyne.CanvasObject{mm.emptyDetailView()}
		mm.detailView.Refresh()
		mm.app.updateStatus(fmt.Sprintf("Removed modpack %q from list", p.Name))
	}, mm.app.mainWindow)
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
