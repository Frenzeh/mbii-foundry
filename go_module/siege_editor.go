package main

import (
	"fmt"
	"io"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	
	"github.com/Frenzeh/mbii-foundry/parsers"
)

type SiegeEditor struct {
	siege       *parsers.SiegeData
	currentPath string
	container   *fyne.Container
	fileManager *FileManager
	lastError   string
	onHover     func(string, string)
	isDirty bool
	onDirtyChanged func(bool)

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
	
	nameEntry *widget.Entry
	useTeamEntry *widget.Entry
	iconEntry *widget.Entry
	briefingEntry *widget.Entry
	
	objList *widget.List
	container *fyne.Container
}

func NewSiegeEditor(app *App) *SiegeEditor {
	e := &SiegeEditor{
		siege:       parsers.NewSiegeData(),
		fileManager: app.fileManager,
		app:         app,
	}
	e.createUI()
	return e
}

func (e *SiegeEditor) SetOnHover(f func(string, string)) { e.onHover = f }
func (e *SiegeEditor) SetAssetBrowser(ab *AssetBrowser) { e.assetBrowser = ab }
func (e *SiegeEditor) SetHolocronClient(client *HolocronClient) { e.holocronClient = client }
func (e *SiegeEditor) IsDirty() bool { return e.isDirty }
func (e *SiegeEditor) MarkClean() { e.isDirty = false; if e.onDirtyChanged != nil { e.onDirtyChanged(false) } }
func (e *SiegeEditor) SetOnDirtyChanged(f func(bool)) { e.onDirtyChanged = f }

func (e *SiegeEditor) markDirty() {
	if !e.isDirty {
		e.isDirty = true
		if e.onDirtyChanged != nil {
			e.onDirtyChanged(true)
		}
	}
}

func (e *SiegeEditor) createUI() {
	e.missionNameEntry = widget.NewEntry(); e.missionNameEntry.OnChanged = func(s string) { e.markDirty() }
	e.mapGraphicEntry = widget.NewEntry(); e.mapGraphicEntry.OnChanged = func(s string) { e.markDirty() }
	e.radarTLEntry = widget.NewEntry(); e.radarTLEntry.OnChanged = func(s string) { e.markDirty() }
	e.radarBREntry = widget.NewEntry(); e.radarBREntry.OnChanged = func(s string) { e.markDirty() }
	e.modesEntry = widget.NewEntry(); e.modesEntry.OnChanged = func(s string) { e.markDirty() }
	
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
	
	e.sourceView = widget.NewMultiLineEntry()
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

func NewTeamUI(editor *SiegeEditor, label string) *TeamUI {
	ui := &TeamUI{editor: editor}
	
	ui.nameEntry = widget.NewEntry(); ui.nameEntry.OnChanged = func(s string) { editor.markDirty() }
	ui.useTeamEntry = widget.NewEntry(); ui.useTeamEntry.OnChanged = func(s string) { editor.markDirty() }
	ui.iconEntry = widget.NewEntry(); ui.iconEntry.OnChanged = func(s string) { editor.markDirty() }
	ui.briefingEntry = widget.NewMultiLineEntry(); ui.briefingEntry.OnChanged = func(s string) { editor.markDirty() }
	
	form := widget.NewForm(
		widget.NewFormItem("Team Name", ui.nameEntry),
		widget.NewFormItem("Use Team (.mbtc)", ui.useTeamEntry),
		widget.NewFormItem("Icon", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { editor.app.showFilePickerForEntry(ui.iconEntry, "Select Team Icon", AssetTypeIcon) }, "Browse for Team Icon"), ui.iconEntry)),
		widget.NewFormItem("Briefing", ui.briefingEntry),
	)
	
	// Objectives List (Placeholder for now, full obj editing is complex)
	ui.objList = widget.NewList(
		func() int { 
			if ui.team == nil { return 0 }
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
	if team == nil { return }
	team.Name = ui.nameEntry.Text
	team.UseTeam = ui.useTeamEntry.Text
	team.TeamIcon = ui.iconEntry.Text
	team.Briefing = ui.briefingEntry.Text
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
	
	if e.siege.Team1 != nil { e.team1UI.Update(e.siege.Team1) }
	if e.siege.Team2 != nil { e.team2UI.Update(e.siege.Team2) }
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
func (e *SiegeEditor) GetCurrentPath() string { return e.currentPath }
func (e *SiegeEditor) SetCurrentPath(path string) { e.currentPath = path }

func (e *SiegeEditor) LoadFile(path string) error {
	file, err := os.Open(path); if err != nil { return err }; defer file.Close()
	content, err := io.ReadAll(file); if err != nil { e.lastError = fmt.Sprintf("Failed to read file: %v", err); return err }
	
	siege, err := parsers.ParseSiege(string(content))
	if err != nil {
		dialog.ShowError(fmt.Errorf("Error parsing file: %v", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return err 
	}
	
	e.siege = siege
	e.currentPath = path
	e.updateUI()
	if e.fileManager != nil { e.fileManager.AddRecentFile(path) }
	e.lastError = ""
	e.MarkClean() // Mark clean after loading
	return nil
}

func (e *SiegeEditor) SaveToWriter(w io.Writer) error {
	e.updateSiegeFromUI()
	content, err := parsers.GenerateSiege(e.siege)
	if err != nil { return err }
	_, err = w.Write([]byte(content))
	return err
}

func (e *SiegeEditor) SaveFile(path string) error {
	file, err := os.Create(path); if err != nil { return err }; defer file.Close()
	
	if err := e.SaveToWriter(file); err != nil {
		return err
	}
	
	e.currentPath = path
	e.lastError = ""
	if e.fileManager != nil { e.fileManager.AddRecentFile(path) }
	e.MarkClean() // Mark clean after saving
	return nil
}

func (e *SiegeEditor) ExportJSON(path string) error { return nil }
func (e *SiegeEditor) ImportJSON(path string) error { return nil }
func (e *SiegeEditor) Validate() []string { return []string{} }
