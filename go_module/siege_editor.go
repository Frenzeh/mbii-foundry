package main

import (
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"io"
	"os"
	"strings"

	"github.com/Frenzeh/mbii-foundry/parsers"
	"github.com/Frenzeh/mbii-foundry/safeio"
)

type SiegeEditor struct {
	siege       *parsers.SiegeData
	currentPath string
	container   *fyne.Container
	fileManager *FileManager
	lastError   string
	onHover     func(string, string)

	// Session state — baseline, retained draft, undo/redo history.
	session *DocumentSession
	loading bool // silences markDirty/source pushes during programmatic sync

	isDirty        bool
	onDirtyChanged func(bool)
	sourceSubs     *sourceListeners

	// Global Fields
	missionNameEntry *widget.Entry
	mapGraphicEntry  *widget.Entry
	radarTLEntry     *widget.Entry
	radarBREntry     *widget.Entry
	modesEntry       *widget.Entry

	// Teams UI
	team1UI *TeamUI
	team2UI *TeamUI

	assetBrowser   *AssetBrowser
	holocronClient *HolocronClient
	app            *App
	sourceView     *widget.Entry
}

type TeamUI struct {
	editor *SiegeEditor
	team   *parsers.SiegeTeam

	nameEntry     *widget.Entry
	useTeamEntry  *widget.Entry
	iconEntry     *widget.Entry
	briefingEntry *widget.Entry

	objList   *widget.List
	container *fyne.Container
}

func NewTeamUI(editor *SiegeEditor, label string) *TeamUI {
	ui := &TeamUI{editor: editor}

	ui.nameEntry = NewInputEntry()
	ui.nameEntry.OnChanged = func(s string) { editor.markDirty() }
	ui.useTeamEntry = NewInputEntry()
	ui.useTeamEntry.OnChanged = func(s string) { editor.markDirty() }
	ui.iconEntry = NewInputEntry()
	ui.iconEntry.OnChanged = func(s string) { editor.markDirty() }
	ui.briefingEntry = NewMultiLineInputEntry()
	ui.briefingEntry.OnChanged = func(s string) { editor.markDirty() }

	form := widget.NewForm(
		widget.NewFormItem("Team Name", ui.nameEntry),
		widget.NewFormItem("Use Team (.mbtc)", ui.useTeamEntry),
		widget.NewFormItem("Icon", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { editor.app.showFilePickerForEntry(ui.iconEntry, "Select Team Icon", AssetTypeIcon) }, "Browse for Team Icon"), ui.iconEntry)),
		widget.NewFormItem("Briefing", ui.briefingEntry),
	)

	// Objectives List (Placeholder for now, full obj editing is complex)
	ui.objList = widget.NewList(
		func() int {
			if ui.team == nil {
				return 0
			}
			return len(ui.team.Objectives)
		},
		func() fyne.CanvasObject { return widget.NewLabel("Objective") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if ui.team != nil {
				obj.(*widget.Label).SetText(ui.team.Objectives[id].GoalName)
			}
		},
	)

	ui.container = container.NewBorder(
		widget.NewLabelWithStyle(label, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		container.NewVSplit(form, ui.objList),
	)
	return ui
}

func (ui *TeamUI) Update(team *parsers.SiegeTeam) {
	ui.team = team
	if team == nil {
		ui.nameEntry.SetText("")
		ui.useTeamEntry.SetText("")
		return
	}
	ui.nameEntry.SetText(team.Name)
	ui.useTeamEntry.SetText(team.UseTeam)
	ui.iconEntry.SetText(team.TeamIcon)
	ui.briefingEntry.SetText(team.Briefing)
	ui.objList.Refresh()
}

func (ui *TeamUI) ApplyTo(team *parsers.SiegeTeam) {
	if team == nil {
		return
	}
	team.Name = ui.nameEntry.Text
	team.UseTeam = ui.useTeamEntry.Text
	team.TeamIcon = ui.iconEntry.Text
	team.Briefing = ui.briefingEntry.Text
}

func NewSiegeEditor(app *App) *SiegeEditor {
	e := &SiegeEditor{
		siege:       parsers.NewSiegeData(),
		fileManager: app.fileManager,
		app:         app,
		sourceSubs:  &sourceListeners{},
	}
	e.session = NewDocumentSession(
		func() string { return e.sessionRender() },
		func(src string) error { return e.sessionRestore(src) },
	)
	e.session.SetOnDirtyChange(e.setDirtyState)
	e.createUI()
	return e
}

func (e *SiegeEditor) SetOnHover(f func(string, string))        { e.onHover = f }
func (e *SiegeEditor) SetAssetBrowser(ab *AssetBrowser)         { e.assetBrowser = ab }
func (e *SiegeEditor) SetHolocronClient(client *HolocronClient) { e.holocronClient = client }

// Dirty tracking — the flag mirrors the session's derived dirty
// state so tab titles and close/quit guards stay in sync.
func (e *SiegeEditor) IsDirty() bool                  { return e.isDirty }
func (e *SiegeEditor) SetOnDirtyChanged(f func(bool)) { e.onDirtyChanged = f }

func (e *SiegeEditor) setDirtyState(d bool) {
	if e.isDirty != d {
		e.isDirty = d
		if e.onDirtyChanged != nil {
			e.onDirtyChanged(d)
		}
	}
}

func (e *SiegeEditor) MarkClean() {
	e.session.MarkClean()
	e.setDirtyState(e.session.DerivedDirty())
}

// markDirty is invoked by every form OnChanged handler. The loading
// guard keeps programmatic UI syncs from dirtying the document.
func (e *SiegeEditor) markDirty() {
	if e.loading {
		return
	}
	e.setDirtyState(true)
	e.session.noteUserEdit()
	e.fireSourceChanged()
}

func (e *SiegeEditor) fireSourceChanged() {
	if !e.loading {
		e.sourceSubs.fire()
	}
}

// sessionRender snapshots the working state (UI folded in).
func (e *SiegeEditor) sessionRender() string { return e.GenerateSource() }

// sessionRestore swaps a snapshot back into the working model
// in-memory. Path, baseline and dirty bookkeeping are untouched.
func (e *SiegeEditor) sessionRestore(src string) error {
	siege, err := parsers.ParseSiege(src)
	if err != nil {
		return err
	}
	e.loading = true
	e.siege = siege
	e.updateUI()
	e.loading = false
	e.fireSourceChanged()
	return nil
}

func (e *SiegeEditor) Session() *DocumentSession { return e.session }
func (e *SiegeEditor) CurrentSource() string     { return e.sessionRender() }
func (e *SiegeEditor) Undo() bool                { return e.session.Undo() }
func (e *SiegeEditor) Redo() bool                { return e.session.Redo() }
func (e *SiegeEditor) CanUndo() bool             { return e.session.CanUndo() }
func (e *SiegeEditor) CanRedo() bool             { return e.session.CanRedo() }

// Definition navigation — .siege files are single-block documents,
// so the definition APIs are trivial.
func (e *SiegeEditor) DefinitionNames() []string {
	if name := strings.TrimSpace(e.siege.MissionName); name != "" {
		return []string{name}
	}
	return []string{"siege"}
}
func (e *SiegeEditor) SelectedDefinition() int { return 0 }
func (e *SiegeEditor) SelectDefinition(index int) error {
	if index != 0 {
		return fmt.Errorf("definition %d out of range (1 definition)", index+1)
	}
	if e.DefinitionNames()[0] == "" {
		return fmt.Errorf("no siege definition loaded")
	}
	return nil
}

// ApplySourceText parses src in-memory (no temp files, no LoadFile)
// and swaps it into the working model. A failed parse leaves the
// document untouched; the applied state becomes one undo step.
func (e *SiegeEditor) ApplySourceText(src string) error {
	if err := validateSourceStructure(src); err != nil {
		return err
	}
	siege, err := parsers.ParseSiege(src)
	if err != nil {
		return err
	}
	e.session.beginDiscreteChange()
	e.siege = siege
	e.loading = true
	e.updateUI()
	e.loading = false
	e.session.endDiscreteChange()
	e.fireSourceChanged()
	return nil
}

// SourceProvider impl — push-notified via sourceSubs; the SourcePanel
// ticker remains as a fallback.
func (e *SiegeEditor) GenerateSource() string {
	if e.siege == nil {
		return ""
	}
	if !e.loading {
		e.updateSiegeFromUI()
	}
	content, err := parsers.GenerateSiege(e.siege)
	if err != nil {
		return "// generate error: " + err.Error()
	}
	return content
}

// SetOnSourceChanged keeps legacy single-callback semantics — wired
// through the shared listener registry instead of being a stub.
func (e *SiegeEditor) SetOnSourceChanged(f func()) { e.sourceSubs.reset(f) }

// AddSourceListener lets mirrors and panels subscribe without
// stealing each other's push notifications.
func (e *SiegeEditor) AddSourceListener(fn func()) func() {
	return e.sourceSubs.add(fn)
}

func (e *SiegeEditor) createUI() {
	e.missionNameEntry = NewInputEntry()
	e.missionNameEntry.OnChanged = func(s string) { e.markDirty() }
	e.mapGraphicEntry = NewInputEntry()
	e.mapGraphicEntry.OnChanged = func(s string) { e.markDirty() }
	e.radarTLEntry = NewInputEntry()
	e.radarTLEntry.OnChanged = func(s string) { e.markDirty() }
	e.radarBREntry = NewInputEntry()
	e.radarBREntry.OnChanged = func(s string) { e.markDirty() }
	e.modesEntry = NewInputEntry()
	e.modesEntry.OnChanged = func(s string) { e.markDirty() }

	globalForm := widget.NewForm(
		widget.NewFormItem("Mission Name", e.missionNameEntry),
		widget.NewFormItem("Map Graphic", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { e.app.showFilePickerForEntry(e.mapGraphicEntry, "Select Map Graphic", AssetTypeGFX) }, "Browse for Map Graphic"), e.mapGraphicEntry)),
		widget.NewFormItem("Radar Top Left", e.radarTLEntry),
		widget.NewFormItem("Radar Bottom Right", e.radarBREntry),
		widget.NewFormItem("MB Modes Allowed", e.modesEntry),
	)

	e.team1UI = NewTeamUI(e, "Team 1 (Heroes/Imperials)")
	e.team2UI = NewTeamUI(e, "Team 2 (Villains/Rebels)")

	teamsSplit := container.NewHSplit(e.team1UI.container, e.team2UI.container)

	e.sourceView = NewMultiLineInputEntry()
	e.sourceView.TextStyle = fyne.TextStyle{Monospace: true}
	sourceTab := container.NewMax(container.NewScroll(e.sourceView))

	tabs := container.NewAppTabs(
		container.NewTabItem("Global", container.NewVScroll(globalForm)),
		container.NewTabItem("Teams", teamsSplit),
		container.NewTabItem("Source", sourceTab),
	)

	tabs.OnSelected = func(tab *container.TabItem) {
		if tab.Text == "Source" {
			e.updateSourceView()
		}
	}

	e.container = container.NewMax(tabs)
}

func (e *SiegeEditor) updateSourceView() {
	e.updateSiegeFromUI()
	content, err := parsers.GenerateSiege(e.siege)
	if err != nil {
		e.sourceView.SetText("Error generating source: " + err.Error())
		return
	}
	e.sourceView.SetText(content)
}

func (e *SiegeEditor) updateUI() {
	e.missionNameEntry.SetText(e.siege.MissionName)
	e.mapGraphicEntry.SetText(e.siege.MapGraphic)
	e.radarTLEntry.SetText(e.siege.RadarTopLeft)
	e.radarBREntry.SetText(e.siege.RadarBottomRight)
	e.modesEntry.SetText(e.siege.MBModesAllowed)

	if e.siege.Team1 != nil {
		e.team1UI.Update(e.siege.Team1)
	}
	if e.siege.Team2 != nil {
		e.team2UI.Update(e.siege.Team2)
	}
}

func (e *SiegeEditor) updateSiegeFromUI() {
	e.siege.MissionName = e.missionNameEntry.Text
	e.siege.MapGraphic = e.mapGraphicEntry.Text
	e.siege.RadarTopLeft = e.radarTLEntry.Text
	e.siege.RadarBottomRight = e.radarBREntry.Text
	e.siege.MBModesAllowed = e.modesEntry.Text

	e.team1UI.ApplyTo(e.siege.Team1)
	e.team2UI.ApplyTo(e.siege.Team2)
}

func (e *SiegeEditor) GetContent() fyne.CanvasObject { return e.container }
func (e *SiegeEditor) GetCurrentPath() string        { return e.currentPath }
func (e *SiegeEditor) SetCurrentPath(path string)    { e.currentPath = path }

// LoadFile parses and installs the siege document, then anchors the
// session at the loaded bytes so the file opens clean.
func (e *SiegeEditor) LoadFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read file: %v", err)
		return err
	}
	src := string(content)
	siege, err := parsers.ParseSiege(src)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Error parsing file: %v", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return err
	}

	e.loading = true
	e.siege = siege
	e.currentPath = path
	e.updateUI()
	e.loading = false
	e.fireSourceChanged()

	if e.fileManager != nil {
		e.fileManager.AddRecentFile(path)
	}
	e.lastError = ""
	e.session.Reset()
	e.session.SetBaseline(path, src)
	e.setDirtyState(false)
	return nil
}

func (e *SiegeEditor) SaveToWriter(w io.Writer) error {
	e.updateSiegeFromUI()
	content, err := parsers.GenerateSiege(e.siege)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

// PrepareSave returns the exact bytes a save would write without
// mutating any state (the save-review dialog displays these).
func (e *SiegeEditor) PrepareSave(path string) (string, error) {
	return parsers.GenerateSiege(e.siege)
}

// CommitSave publishes reviewed bytes, then refreshes document state.
// Every failure before publication leaves model/path/baseline intact.
func (e *SiegeEditor) CommitSave(path string, candidate string) error {
	current, err := e.PrepareSave(path)
	if err != nil {
		return err
	}
	if current != candidate {
		return fmt.Errorf("reviewed candidate no longer matches the current document")
	}
	if err := publishEditorCandidate(e.fileManager, path, candidate); err != nil {
		e.lastError = fmt.Sprintf("Failed to save file: %v", err)
		return err
	}
	e.currentPath = path
	e.lastError = ""
	e.session.SetBaseline(path, candidate)
	e.session.MarkClean()
	e.setDirtyState(false)
	e.fireSourceChanged()
	return nil
}

// SaveFile is the programmatic (no-review) entry point; UI saves
// route through PrepareSave/CommitSave via the review dialog.
func (e *SiegeEditor) SaveFile(path string) error {
	candidate, err := e.PrepareSave(path)
	if err != nil {
		return err
	}
	return e.CommitSave(path, candidate)
}

func (e *SiegeEditor) ExportJSON(path string) error {
	e.updateSiegeFromUI()
	data, err := json.MarshalIndent(e.siege, "", "  ")
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to marshal JSON: %v", err)
		return err
	}
	if err := safeio.WriteFile(path, data, 0644); err != nil {
		e.lastError = fmt.Sprintf("Failed to write JSON: %v", err)
		return err
	}
	e.lastError = ""
	return nil
}

// ImportJSON swaps the JSON payload in-memory; undoable as one step
// and marked dirty since the working state diverges from the baseline.
func (e *SiegeEditor) ImportJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read JSON: %v", err)
		return err
	}
	siege := parsers.NewSiegeData()
	if err := json.Unmarshal(data, siege); err != nil {
		e.lastError = fmt.Sprintf("Failed to parse JSON: %v", err)
		return err
	}
	e.session.beginDiscreteChange()
	e.siege = siege
	e.loading = true
	e.updateUI()
	e.loading = false
	e.session.endDiscreteChange()
	e.fireSourceChanged()
	e.lastError = ""
	return nil
}

// Validate checks the structural and engine-facing invariants of the
// siege document. Pure — no dialogs, returns human-readable issues.
func (e *SiegeEditor) Validate() []string {
	e.updateSiegeFromUI()
	var issues []string

	// The document must round-trip: generate → parse is exactly the
	// journey the engine's tokenizer takes at load time.
	gen, err := parsers.GenerateSiege(e.siege)
	if err != nil {
		issues = append(issues, fmt.Sprintf("source generation failed: %v", err))
		return issues
	}
	if _, err := parsers.ParseSiege(gen); err != nil {
		issues = append(issues, fmt.Sprintf("generated source does not re-parse: %v", err))
	}

	// Structure: the engine loads exactly two teams; blocks are
	// mandatory, and each must point at its team config (.mbtc).
	missing := []string{}
	if e.siege.Team1 == nil {
		missing = append(missing, "team1")
	}
	if e.siege.Team2 == nil {
		missing = append(missing, "team2")
	}
	if len(missing) > 0 {
		issues = append(issues, fmt.Sprintf("missing %s block(s) — BG_SiegeManagerClient requires both teams", strings.Join(missing, " and ")))
	} else {
		teams := []*parsers.SiegeTeam{e.siege.Team1, e.siege.Team2}
		for i, team := range teams {
			label := fmt.Sprintf("team%d", i+1)
			if strings.TrimSpace(team.UseTeam) == "" {
				issues = append(issues, label+" has no useTeam — the engine cannot load the team without its .mbtc config")
			}
			if strings.TrimSpace(team.Name) == "" {
				issues = append(issues, label+" has no name — team labels render blank in the HUD")
			}
			if len(team.Objectives) == 0 {
				issues = append(issues, label+" has no objectives — the round cannot be won")
			}
		}
		if t1, t2 := strings.TrimSpace(e.siege.Team1.UseTeam), strings.TrimSpace(e.siege.Team2.UseTeam); t1 != "" && t1 == t2 {
			issues = append(issues, fmt.Sprintf("both teams use %q — likely a copy-paste mistake", t1))
		}
	}

	if strings.TrimSpace(e.siege.MissionName) == "" {
		issues = append(issues, "missionname is empty — siege maps are listed without a title")
	}

	// Budget guard, consistent with the source panel's byte counter:
	// 16384 is the documented engine ceiling for these config files.
	if len(gen) > 16384 {
		issues = append(issues, fmt.Sprintf("file is %d bytes — over the 16384-byte config limit", len(gen)))
	} else if len(gen) > 15000 {
		issues = append(issues, fmt.Sprintf("file is %d bytes — approaching the 16384-byte config limit", len(gen)))
	}

	return issues
}
