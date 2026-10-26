package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/safeio"
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
	loadErr   error

	// UI
	projectList *widget.List
	detailView  *fyne.Container
}

func NewModpackManager(app *App) *ModpackManager {
	mm := &ModpackManager{
		app:      app,
		projects: []*Modpack{},
	}
	loadErr := mm.loadProjects()
	mm.createUI()
	if loadErr != nil {
		app.updateStatus("Modpack metadata could not be loaded: " + loadErr.Error())
	}
	return mm
}

func (mm *ModpackManager) loadProjects() error {
	if mm.app.configPath == "" {
		mm.loadErr = nil
		return nil
	}
	configPath := filepath.Join(filepath.Dir(mm.app.configPath), "fa_projects.json")
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		mm.loadErr = nil
		return nil
	}
	if err != nil {
		mm.loadErr = fmt.Errorf("read projects: %w", err)
		return mm.loadErr
	}
	var projects []*Modpack
	if err := json.Unmarshal(data, &projects); err != nil {
		mm.loadErr = fmt.Errorf("decode projects: %w", err)
		return mm.loadErr
	}
	mm.projects = projects
	mm.loadErr = nil
	return nil
}

func (mm *ModpackManager) saveProjects() error {
	if mm.loadErr != nil {
		return fmt.Errorf("project metadata is unavailable; original preserved: %w", mm.loadErr)
	}
	if mm.app.configPath == "" {
		return fmt.Errorf("configuration directory unavailable")
	}
	configPath := filepath.Join(filepath.Dir(mm.app.configPath), "fa_projects.json")
	data, err := json.MarshalIndent(mm.projects, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal projects: %w", err)
	}
	return safeio.WriteFile(configPath, data, 0644)
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
	pathEntry.SetPlaceHolder("/path/to/MyCoolMod")

	authorEntry := NewInputEntry()
	authorEntry.SetPlaceHolder("Your name")

	errorLabel := widget.NewLabel("")
	errorLabel.Wrapping = fyne.TextWrapWord

	browseBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				pathEntry.SetText(filepath.Join(uri.Path(), nameEntry.Text))
			}
		}, mm.app.mainWindow)
	})

	dialog.ShowForm("Create New Modpack", "Create", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Author", authorEntry),
		widget.NewFormItem("Location", container.NewBorder(nil, nil, nil, browseBtn, pathEntry)),
		widget.NewFormItem("", errorLabel),
	}, func(confirm bool) {
		if !confirm {
			return
		}
		name := strings.TrimSpace(nameEntry.Text)
		path := strings.TrimSpace(pathEntry.Text)
		if name == "" {
			errorLabel.SetText("Name is required")
			return
		}
		if path == "" {
			errorLabel.SetText("Location is required")
			return
		}
		mm.createProject(name, path, strings.TrimSpace(authorEntry.Text))
	}, mm.app.mainWindow)
}

// createProject scaffolds a modpack folder and registers it. Only
// directories actually created by this attempt are rollback candidates;
// pre-existing paths are never removed. A failed rollback reports the
// exact partial scaffold still present.
func (mm *ModpackManager) createProject(name, path, author string) {
	// Duplicate guard: same folder tracked twice would double-export it.
	for _, existing := range mm.projects {
		if existing.Path == path {
			dialog.ShowInformation("Already Tracked",
				fmt.Sprintf("The folder is already tracked as project %q.", existing.Name), mm.app.mainWindow)
			return
		}
	}

	dirs := []string{
		"ext_data/mb2/character",
		"ext_data/sabers",
		"ext_data/vehicles",
		"maps",
		"shaders",
		"models/players",
		"gfx/hud",
	}

	var created []string
	preexistingLeaves := 0
	for _, d := range dirs {
		full := filepath.Join(path, d)
		if info, statErr := os.Stat(full); statErr == nil && info.IsDir() {
			preexistingLeaves++
			continue
		}
		if err := makeTrackedDirectories(full, &created); err != nil {
			remaining := rollbackCreatedDirectories(created)
			detail := fmt.Sprintf("failed to create %s: %v", d, err)
			if len(remaining) == 0 {
				detail += fmt.Sprintf("\n\nRemoved all %d directories created by this attempt; pre-existing content was untouched.", len(created))
			} else {
				detail += fmt.Sprintf("\n\nRollback left %d newly-created directorie(s) because they are no longer empty or removable: %s",
					len(remaining), strings.Join(remaining, ", "))
			}
			mm.showRetryable("Create Modpack Failed", fmt.Errorf("%s", detail),
				func() { mm.createProject(name, path, author) })
			return
		}
	}

	p := &Modpack{
		Name:       name,
		Version:    "0.1",
		Author:     author,
		Path:       path,
		LastEdited: time.Now(),
	}

	if err := mm.saveProjectEntry(p); err != nil {
		remaining := rollbackCreatedDirectories(created)
		detail := fmt.Sprintf("could not persist project list: %v", err)
		if len(remaining) == 0 {
			detail += fmt.Sprintf("\n\nRemoved all %d directories created by this attempt; pre-existing content was untouched.", len(created))
		} else {
			detail += fmt.Sprintf("\n\nProject metadata was not saved. %d newly-created directorie(s) remain because they are no longer empty or removable: %s",
				len(remaining), strings.Join(remaining, ", "))
		}
		mm.showRetryable("Create Modpack Failed", fmt.Errorf("%s", detail),
			func() { mm.createProject(name, path, author) })
		return
	}

	mm.projectList.Refresh()
	mm.projectList.Select(len(mm.projects) - 1)

	dialog.ShowInformation("Modpack Created",
		fmt.Sprintf("Created %q: %d new directories, %d scaffold folders already existed.\n%s",
			name, len(created), preexistingLeaves, path),
		mm.app.mainWindow)
}

// makeTrackedDirectories is MkdirAll with ownership accounting. Every
// path appended to created was absent immediately before this call made
// it, so rollback never guesses from a final directory tree.
func makeTrackedDirectories(path string, created *[]string) error {
	var missing []string
	for cursor := filepath.Clean(path); ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("%s exists and is not a directory", cursor)
			}
			break
		}
		if !os.IsNotExist(err) {
			return err
		}
		missing = append(missing, cursor)
		parent := filepath.Dir(cursor)
		if parent == cursor {
			return fmt.Errorf("no existing parent for %s", path)
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		dir := missing[i]
		if err := os.Mkdir(dir, 0755); err != nil {
			if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
				continue // concurrently created: usable, but not ours
			}
			return err
		}
		*created = append(*created, dir)
	}
	return nil
}

// rollbackCreatedDirectories removes only paths recorded by
// makeTrackedDirectories, deepest-first. Non-empty directories are kept
// and returned so the failure message can report the real partial output.
func rollbackCreatedDirectories(created []string) []string {
	var remaining []string
	for i := len(created) - 1; i >= 0; i-- {
		if err := os.Remove(created[i]); err != nil && !os.IsNotExist(err) {
			remaining = append(remaining, created[i])
		}
	}
	return remaining
}

// saveProjectEntry persists the new project list. If the write fails,
// the in-memory entry is never appended, keeping memory and disk
// consistent.
func (mm *ModpackManager) saveProjectEntry(p *Modpack) error {
	for _, existing := range mm.projects {
		if existing.Path == p.Path {
			return fmt.Errorf("already tracked: %s", p.Path)
		}
	}

	newList := make([]*Modpack, len(mm.projects), len(mm.projects)+1)
	copy(newList, mm.projects)
	newList = append(newList, p)

	orig := mm.projects
	mm.projects = newList
	if err := mm.saveProjects(); err != nil {
		mm.projects = orig
		return err
	}
	return nil
}

func (mm *ModpackManager) showRetryable(title string, cause error, retry func()) {
	dialog.ShowConfirm(title,
		fmt.Sprintf("%v\n\nRetry?", cause),
		func(confirmed bool) {
			if confirmed {
				retry()
			}
		}, mm.app.mainWindow)
}

func (mm *ModpackManager) importProject() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if uri == nil {
			return
		}
		mm.importProjectAt(uri.Path())
	}, mm.app.mainWindow)
}

// importProjectAt registers the folder at path (chosen via the folder
// picker) as a modpack, rejecting empties, unreadable folders, and
// duplicates; the list entry persists atomically via saveProjectEntry.
func (mm *ModpackManager) importProjectAt(path string) {
	name := filepath.Base(path)

	// Validate: folder must exist and contain at least one file
	entries, readErr := os.ReadDir(path)
	if readErr != nil {
		dialog.ShowError(fmt.Errorf("cannot read folder: %w", readErr), mm.app.mainWindow)
		return
	}
	if len(entries) == 0 {
		dialog.ShowError(fmt.Errorf("folder %q is empty; import a folder that contains modpack content", name), mm.app.mainWindow)
		return
	}

	// Check for duplicates
	for _, existing := range mm.projects {
		if existing.Path == path {
			dialog.ShowInformation("Already Imported",
				fmt.Sprintf("%q is already in the project list.", name), mm.app.mainWindow)
			return
		}
	}

	p := &Modpack{
		Name:       name,
		Path:       path,
		Version:    "1.0",
		LastEdited: time.Now(),
	}
	if err := mm.saveProjectEntry(p); err != nil {
		// The in-memory entry was reverted; the folder on disk is
		// untouched, so importing again is always possible.
		mm.showRetryable("Import Failed",
			fmt.Errorf("could not persist project list: %w", err),
			func() { mm.importProject() })
		return
	}
	mm.projectList.Refresh()
	mm.projectList.Select(len(mm.projects) - 1)
}

func (mm *ModpackManager) showProjectDetails(p *Modpack) {
	nameLabel := widget.NewLabelWithStyle(p.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	pathLabel := widget.NewLabel(p.Path)
	pathLabel.Wrapping = fyne.TextWrapWord

	// Editable metadata fields remain drafts until the checked metadata
	// write succeeds; a failure cannot silently mutate the tracked model.
	versionEntry := NewInputEntry()
	versionEntry.SetText(p.Version)
	authorEntry := NewInputEntry()
	authorEntry.SetText(p.Author)
	descEntry := NewMultiLineInputEntry()
	descEntry.SetText(p.Description)
	descEntry.SetMinRowsVisible(3)

	var commitMetadata func()
	commitMetadata = func() {
		oldVersion, oldAuthor, oldDescription, oldEdited := p.Version, p.Author, p.Description, p.LastEdited
		p.Version = versionEntry.Text
		p.Author = authorEntry.Text
		p.Description = descEntry.Text
		p.LastEdited = time.Now()
		if err := mm.saveProjects(); err != nil {
			p.Version, p.Author, p.Description, p.LastEdited = oldVersion, oldAuthor, oldDescription, oldEdited
			mm.showRetryable("Metadata Save Failed", err, commitMetadata)
			return
		}
		mm.projectList.Refresh()
		mm.app.updateStatus(fmt.Sprintf("Metadata saved for %q", p.Name))
	}
	saveMetaBtn := widget.NewButtonWithIcon("Save Metadata", theme.DocumentSaveIcon(), commitMetadata)

	openBtn := widget.NewButtonWithIcon("Open in Editor", theme.LoginIcon(), func() {
		// Set app context to this project's character folder
		charDir := filepath.Join(p.Path, "ext_data/mb2/character")
		if info, err := os.Stat(charDir); err == nil && info.IsDir() {
			mm.app.config.LastOpenDir = charDir
		} else {
			mm.app.config.LastOpenDir = p.Path
		}
		mm.app.updateStatus("Switched workspace to: " + p.Name)
	})

	// Build PK3 — uses a non-writing SavePath chooser, then
	// BuildExportManifest + WritePK3 via safeio for atomic output.
	buildBtn := widget.NewButtonWithIcon("Build PK3", theme.DownloadIcon(), func() {
		mm.buildPK3Dialog(p)
	})
	buildBtn.Importance = widget.HighImportance

	// Manifest preview — shows what would be included before building
	previewBtn := widget.NewButtonWithIcon("Preview Manifest", theme.VisibilityIcon(), func() {
		mm.showManifestPreview(p)
	})
	previewBtn.Importance = widget.LowImportance

	shareBtn := widget.NewButtonWithIcon("Share / Export Source", theme.MailAttachmentIcon(), func() {
		mm.shareProject(p)
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
		widget.NewFormItem("Version", versionEntry),
		widget.NewFormItem("Author", authorEntry),
		widget.NewFormItem("Description", descEntry),
	)

	actions := container.NewVBox(
		container.NewHBox(openBtn, buildBtn, previewBtn),
		container.NewHBox(shareBtn, saveMetaBtn, layout.NewSpacer(), removeBtn),
	)

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

// buildPK3Dialog prompts for a destination path (non-writing chooser)
// then validates the manifest and writes via BuildExportManifest + WritePK3.
func (mm *ModpackManager) buildPK3Dialog(p *Modpack) {
	// Validate source exists
	if _, err := os.Stat(p.Path); err != nil {
		dialog.ShowError(fmt.Errorf("project folder not found: %w", err), mm.app.mainWindow)
		return
	}

	// Generate manifest first to validate before asking for destination
	manifest, err := BuildExportManifest(p.Path, "")
	if err != nil {
		dialog.ShowError(fmt.Errorf("manifest validation failed: %w", err), mm.app.mainWindow)
		return
	}
	if len(manifest) == 0 {
		dialog.ShowError(fmt.Errorf("project folder is empty — nothing to build"), mm.app.mainWindow)
		return
	}

	// Non-writing save path chooser: the destination file is not opened
	// or truncated while choosing; failed/empty attempts keep the dialog
	// (and the chosen folder/name) intact for correction and retry.
	ShowSavePathDialog(mm.app.mainWindow, "Build PK3", mm.defaultExportPath(p.Name+".pk3"), ".pk3", func(destPath string) error {
		count, warning, err := mm.buildPK3At(p, destPath)
		if err != nil {
			return err
		}
		// The warning (e.g. stale project list) must stay visible next to
		// the success line, never be overwritten by it.
		status := fmt.Sprintf("Built %s (%d files)", filepath.Base(destPath), count)
		if warning != "" {
			status += " — " + warning
		}
		mm.app.updateStatus(status)
		detail := fmt.Sprintf("Wrote %s\n%d files archived.", filepath.Base(destPath), count)
		if warning != "" {
			detail += "\n\nWarning: " + warning
		}
		dialog.ShowInformation("PK3 Built", detail, mm.app.mainWindow)
		return nil
	})
}

// defaultExportPath seeds chooser defaults from the current workspace
// so exports land next to the user's content instead of $HOME.
func (mm *ModpackManager) defaultExportPath(name string) string {
	if dir := strings.TrimSpace(mm.app.config.LastOpenDir); dir != "" {
		return filepath.Join(dir, name)
	}
	return name
}

// buildPK3At writes the PK3 for p to destPath (a fully resolved
// destination from the chooser). Returns the archived file count and a
// non-fatal warning (e.g. a stale project list); the caller must show
// the warning, not overwrite it with the success line.
func (mm *ModpackManager) buildPK3At(p *Modpack, destPath string) (count int, warning string, err error) {
	// Rebuild manifest with the real destination so BuildExportManifest
	// can exclude the output file from the archive. The project content
	// may have changed since the pre-flight — an empty manifest must
	// refuse the write instead of publishing an empty archive.
	finalManifest, err := BuildExportManifest(p.Path, destPath)
	if err != nil {
		return 0, "", fmt.Errorf("manifest failed: %w", err)
	}
	if len(finalManifest) == 0 {
		return 0, "", fmt.Errorf("project folder has no exportable files — refusing to write an empty archive")
	}

	if err := WritePK3(p.Path, destPath, finalManifest); err != nil {
		return 0, "", fmt.Errorf("build failed: %w", err)
	}

	p.LastEdited = time.Now()
	if err := mm.saveProjects(); err != nil {
		// The archive itself is fine; the project-list timestamp is
		// cosmetic. The chooser must not treat this as a failed build.
		warning = fmt.Sprintf("project list not updated: %v", err)
	}
	return len(finalManifest), warning, nil
}

// showManifestPreview shows a read-only list of all files that would
// be included in a PK3 build for this project.
func (mm *ModpackManager) showManifestPreview(p *Modpack) {
	manifest, err := BuildExportManifest(p.Path, "")
	if err != nil {
		dialog.ShowError(fmt.Errorf("manifest scan failed: %w", err), mm.app.mainWindow)
		return
	}
	if len(manifest) == 0 {
		dialog.ShowInformation("Empty Manifest",
			"No files found in the project folder.", mm.app.mainWindow)
		return
	}

	// Build list of archive paths
	var paths []string
	for _, e := range manifest {
		paths = append(paths, e.ArchivePath)
	}

	list := widget.NewList(
		func() int { return len(paths) },
		func() fyne.CanvasObject { return widget.NewLabel("archive/path") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(paths[id])
		},
	)
	summary := widget.NewLabel(fmt.Sprintf("%d files would be included in the PK3 archive.", len(paths)))
	summary.Wrapping = fyne.TextWrapWord

	content := container.NewBorder(summary, nil, nil, nil, list)

	d := dialog.NewCustom("Manifest Preview — "+p.Name, "Close", content, mm.app.mainWindow)
	d.Resize(fyne.NewSize(550, 450))
	d.Show()
}

// removeProject persists the filtered list transactionally. The original
// slice (including its exact pointer order) is restored on any save failure.
func (mm *ModpackManager) removeProject(p *Modpack) error {
	index := -1
	for i, other := range mm.projects {
		if other == p {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("modpack is no longer tracked")
	}

	original := mm.projects
	filtered := make([]*Modpack, 0, len(original)-1)
	filtered = append(filtered, original[:index]...)
	filtered = append(filtered, original[index+1:]...)
	mm.projects = filtered
	if err := mm.saveProjects(); err != nil {
		mm.projects = original
		return err
	}
	return nil
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
		if err := mm.removeProject(p); err != nil {
			dialog.ShowError(fmt.Errorf("could not remove from list: %w", err), mm.app.mainWindow)
			return
		}
		mm.projectList.Refresh()
		mm.projectList.UnselectAll()
		// Reset the right pane back to the empty state so the detail area
		// doesn't keep showing a now-gone project.
		mm.detailView.Objects = []fyne.CanvasObject{mm.emptyDetailView()}
		mm.detailView.Refresh()
		mm.app.updateStatus(fmt.Sprintf("Removed modpack %q from list", p.Name))
	}, mm.app.mainWindow)
}

// exportSourceArchive writes the raw source zip for p to destPath using
// the shared archive pipeline (BuildExportManifest + WritePK3): one
// writer, OpenRoot containment, symlink revalidation, self-output
// exclusion, and Close-then-publish via safeio. Returns the archived
// file count. An empty manifest refuses the write — never publish an
// empty archive over an existing destination.
func (mm *ModpackManager) exportSourceArchive(p *Modpack, destPath string) (int, error) {
	// Rebuild the manifest against the real destination so the archive
	// can never include itself.
	finalManifest, err := BuildExportManifest(p.Path, destPath)
	if err != nil {
		return 0, fmt.Errorf("scan failed: %w", err)
	}
	if len(finalManifest) == 0 {
		return 0, fmt.Errorf("project folder has no exportable files — refusing to write an empty archive")
	}
	if err := WritePK3(p.Path, destPath, finalManifest); err != nil {
		return 0, fmt.Errorf("export failed: %w", err)
	}
	return len(finalManifest), nil
}

// shareProject creates a zip archive of the raw source folder so
// other developers can import it into their own Foundry instance.
func (mm *ModpackManager) shareProject(p *Modpack) {
	if _, err := os.Stat(p.Path); err != nil {
		dialog.ShowError(fmt.Errorf("project folder not found: %w", err), mm.app.mainWindow)
		return
	}

	// Pre-flight manifest so an empty/broken project is diagnosed before
	// any chooser interaction.
	manifest, err := BuildExportManifest(p.Path, "")
	if err != nil {
		dialog.ShowError(fmt.Errorf("scan failed: %w", err), mm.app.mainWindow)
		return
	}
	if len(manifest) == 0 {
		dialog.ShowError(fmt.Errorf("project folder is empty — nothing to export"), mm.app.mainWindow)
		return
	}

	// Non-writing chooser: cancel/failure leaves the destination and the
	// chosen name untouched; the extension is appended by the resolver.
	// The commit uses the SAME exportSourceArchive production path the
	// tests exercise — no parallel writer.
	ShowSavePathDialog(mm.app.mainWindow, "Export Source Archive", mm.defaultExportPath(p.Name+".zip"), ".zip", func(destPath string) error {
		count, err := mm.exportSourceArchive(p, destPath)
		if err != nil {
			return err
		}
		mm.app.updateStatus(fmt.Sprintf("Exported source archive: %s (%d files)", filepath.Base(destPath), count))
		dialog.ShowInformation("Source Exported",
			fmt.Sprintf("Wrote %s\n%d files archived.", filepath.Base(destPath), count),
			mm.app.mainWindow)
		return nil
	})
}
