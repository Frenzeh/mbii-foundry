package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
	"github.com/Frenzeh/mbii-foundry/safeio"
)

type VEHEditor struct {
	vehicle     *parsers.VehicleData
	currentPath string
	container   *fyne.Container
	fileManager *FileManager
	lastError   string
	onHover     func(string, string)

	// Session state — baseline, retained draft, undo/redo history and
	// the selected definition for multi-definition .veh files.
	session   *DocumentSession
	docSource string
	defNames  []string
	loading   bool // silences markDirty/source pushes during programmatic sync

	// Dirty tracking — mirrors SABEditor's pattern. Without this,
	// closing a tab with unsaved .veh edits silently discarded the
	// work because the close-guard's IsDirty() check returned false
	// unconditionally.
	isDirty         bool
	onDirtyChanged  func(bool)
	onSourceChanged func()
	sourceSubs      *sourceListeners

	nameEntry  *widget.Entry
	typeSelect *widget.Select
	modelEntry *widget.Entry
	skinEntry  *widget.Entry

	speedEntry   *widget.Entry
	turboEntry   *widget.Entry
	accelEntry   *widget.Entry
	decelEntry   *widget.Entry
	strafeEntry  *widget.Entry
	brakingEntry *widget.Entry

	armorEntry   *widget.Entry
	shieldsEntry *widget.Entry

	weaponsEntry *widget.Entry

	assetBrowser   *AssetBrowser
	holocronClient *HolocronClient
	app            *App

	sourceView *widget.Entry

	// Definition navigator widgets (summary strip).
	defSelect  *widget.Select
	defSummary *widget.Label
}

var VehicleTypes = []string{"VH_SPEEDER", "VH_ANIMAL", "VH_WALKER", "VH_FIGHTER"}

func NewVEHEditor(app *App) *VEHEditor {
	e := &VEHEditor{
		vehicle:     parsers.NewVehicleData(),
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

func (e *VEHEditor) SetOnHover(f func(string, string))        { e.onHover = f }
func (e *VEHEditor) SetAssetBrowser(ab *AssetBrowser)         { e.assetBrowser = ab }
func (e *VEHEditor) SetHolocronClient(client *HolocronClient) { e.holocronClient = client }

func (e *VEHEditor) createUI() {
	// dirty wires markDirty into every text/select OnChanged handler.
	// Closure form lets us pass it as the OnChanged itself for simple
	// cases; for entries that update vehicle state on edit, the handler
	// chains: update + dirty.
	dirty := func() { e.markDirty() }

	e.nameEntry = NewInputEntry()
	e.nameEntry.OnChanged = func(s string) { dirty() }
	e.typeSelect = widget.NewSelect(VehicleTypes, func(s string) {
		e.vehicle.Type = s
		dirty()
	})
	e.modelEntry = NewInputEntry()
	e.modelEntry.OnChanged = func(s string) { dirty() }
	e.skinEntry = NewInputEntry()
	e.skinEntry.OnChanged = func(s string) { dirty() }

	browseModelBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		if e.app != nil {
			e.app.showFilePickerForEntry(e.modelEntry, "Select Model", AssetTypeModel)
		}
	})
	identityForm := widget.NewForm(
		widget.NewFormItem("Vehicle Name", e.nameEntry),
		widget.NewFormItem("Type", e.typeSelect),
		widget.NewFormItem("Model", container.NewBorder(nil, nil, nil, browseModelBtn, e.modelEntry)),
		widget.NewFormItem("Skin", e.skinEntry),
	)

	e.speedEntry = NewInputEntry()
	e.speedEntry.OnChanged = func(s string) { dirty() }
	e.turboEntry = NewInputEntry()
	e.turboEntry.OnChanged = func(s string) { dirty() }
	e.accelEntry = NewInputEntry()
	e.accelEntry.OnChanged = func(s string) { dirty() }
	e.decelEntry = NewInputEntry()
	e.decelEntry.OnChanged = func(s string) { dirty() }
	e.strafeEntry = NewInputEntry()
	e.strafeEntry.OnChanged = func(s string) { dirty() }
	e.brakingEntry = NewInputEntry()
	e.brakingEntry.OnChanged = func(s string) { dirty() }

	statsForm := widget.NewForm(
		widget.NewFormItem("Max Speed", e.speedEntry),
		widget.NewFormItem("Turbo Speed", e.turboEntry),
		widget.NewFormItem("Acceleration", e.accelEntry),
		widget.NewFormItem("Deceleration", e.decelEntry),
		widget.NewFormItem("Strafe %", e.strafeEntry),
		widget.NewFormItem("Braking", e.brakingEntry),
	)

	e.armorEntry = NewInputEntry()
	e.armorEntry.OnChanged = func(s string) { dirty() }
	e.shieldsEntry = NewInputEntry()
	e.shieldsEntry.OnChanged = func(s string) { dirty() }
	e.weaponsEntry = NewInputEntry()
	e.weaponsEntry.OnChanged = func(s string) { dirty() }

	combatForm := widget.NewForm(
		widget.NewFormItem("Armor", e.armorEntry),
		widget.NewFormItem("Shields", e.shieldsEntry),
		widget.NewFormItem("Weapons", e.weaponsEntry),
	)

	e.sourceView = NewMultiLineInputEntry()
	e.sourceView.TextStyle = fyne.TextStyle{Monospace: true}
	sourceTab := container.NewMax(container.NewScroll(e.sourceView))

	tabs := container.NewAppTabs(
		container.NewTabItem("Specs", container.NewVScroll(container.NewVBox(
			NewFormSection("Identity", "Name the vehicle and connect its model assets.", identityForm),
			NewFormSection("Movement", "Engine movement values; decimals are supported.", statsForm),
			NewFormSection("Combat", "Durability and weapon loadout.", combatForm),
		))),
		container.NewTabItem("Source", sourceTab),
	)

	tabs.OnSelected = func(tab *container.TabItem) {
		if tab.Text == "Source" {
			e.updateSourceView()
		}
	}

	// Definition summary strip: navigates multi-definition .veh files
	// and always names the definition being edited.
	e.defSelect = widget.NewSelect(nil, func(name string) {
		for i, n := range e.defNames {
			if n == name && i != e.session.SelectedDef {
				if err := e.SelectDefinition(i); err != nil {
					dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
				}
				return
			}
		}
	})
	e.defSelect.PlaceHolder = "(single definition)"
	e.defSummary = widget.NewLabel("")
	e.defSummary.TextStyle = fyne.TextStyle{Italic: true}
	defBar := NewFormSection("Active definition", "Choose which block in this file is being edited.",
		container.NewBorder(nil, nil, nil, e.defSummary, e.defSelect))

	e.container = container.NewBorder(container.NewPadded(defBar), nil, nil, nil, tabs)
	e.refreshDefinitionBar()
}

func (e *VEHEditor) updateSourceView() {
	e.updateVehicleFromUI()
	content, err := parsers.GenerateVEH(e.vehicle)
	if err != nil {
		e.sourceView.SetText("Error generating source: " + err.Error())
		return
	}
	e.sourceView.SetText(content)
}

func (e *VEHEditor) updateUI() {
	e.nameEntry.SetText(e.vehicle.Name)
	e.typeSelect.SetSelected(e.vehicle.Type)
	e.modelEntry.SetText(e.vehicle.Model)
	e.skinEntry.SetText(e.vehicle.Skin)

	e.speedEntry.SetText(fmt.Sprintf("%.1f", e.vehicle.SpeedMax))
	e.turboEntry.SetText(fmt.Sprintf("%.1f", e.vehicle.TurboSpeed))
	e.accelEntry.SetText(fmt.Sprintf("%.1f", e.vehicle.Accel))
	e.decelEntry.SetText(fmt.Sprintf("%.1f", e.vehicle.Decel))
	e.strafeEntry.SetText(fmt.Sprintf("%.1f", e.vehicle.StrafePerc))
	e.brakingEntry.SetText(fmt.Sprintf("%.1f", e.vehicle.Braking))

	e.armorEntry.SetText(strconv.Itoa(e.vehicle.Armor))
	e.shieldsEntry.SetText(strconv.Itoa(e.vehicle.Shields))
	e.weaponsEntry.SetText(e.vehicle.Weapons)
}

func (e *VEHEditor) updateVehicleFromUI() {
	e.vehicle.Name = e.nameEntry.Text
	e.vehicle.Type = e.typeSelect.Selected
	e.vehicle.Model = e.modelEntry.Text
	e.vehicle.Skin = e.skinEntry.Text

	e.vehicle.SpeedMax, _ = strconv.ParseFloat(e.speedEntry.Text, 64)
	e.vehicle.TurboSpeed, _ = strconv.ParseFloat(e.turboEntry.Text, 64)
	e.vehicle.Accel, _ = strconv.ParseFloat(e.accelEntry.Text, 64)
	e.vehicle.Decel, _ = strconv.ParseFloat(e.decelEntry.Text, 64)
	e.vehicle.StrafePerc, _ = strconv.ParseFloat(e.strafeEntry.Text, 64)
	e.vehicle.Braking, _ = strconv.ParseFloat(e.brakingEntry.Text, 64)

	e.vehicle.Armor, _ = strconv.Atoi(e.armorEntry.Text)
	e.vehicle.Shields, _ = strconv.Atoi(e.shieldsEntry.Text)
	e.vehicle.Weapons = e.weaponsEntry.Text
}

func (e *VEHEditor) GetContent() fyne.CanvasObject { return e.container }
func (e *VEHEditor) GetCurrentPath() string        { return e.currentPath }
func (e *VEHEditor) GetRecentFiles() []RecentFile  { return e.fileManager.GetRecentFiles() }

// LoadFile reads a possibly multi-definition .veh file. The whole
// source is kept so definition switches re-parse sibling blocks
// byte-for-byte; the selected block drives the form.
func (e *VEHEditor) LoadFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read file: %v", err)
		return err
	}
	src := string(content)
	veh, err := parsers.ParseVEHDefinition(src, 0)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Error parsing file: %v\nProceeding with partial data.", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return err
	}

	e.loading = true
	e.docSource = src
	e.vehicle = veh
	e.session.SelectedDef = 0
	if names, nerr := parsers.DefinitionNames(src); nerr == nil && len(names) > 0 {
		e.defNames = names
	} else {
		e.defNames = []string{}
	}
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
	e.refreshDefinitionBar()
	return nil
}

func (e *VEHEditor) SaveToWriter(w io.Writer) error {
	e.updateVehicleFromUI()
	content, err := parsers.GenerateVEH(e.vehicle)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

// PrepareSave returns the exact bytes a save would write without
// mutating any state (the save-review dialog displays these).
func (e *VEHEditor) PrepareSave(path string) (string, error) {
	return parsers.GenerateVEH(e.vehicle)
}

// CommitSave publishes reviewed bytes, then refreshes document state.
// Every failure before publication leaves model/path/baseline intact.
func (e *VEHEditor) CommitSave(path string, candidate string) error {
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
	e.docSource = candidate
	e.lastError = ""
	e.session.SetBaseline(path, candidate)
	e.session.MarkClean()
	e.setDirtyState(false)
	e.fireSourceChanged()
	return nil
}

// SaveFile is the programmatic (no-review) entry point; UI saves
// route through PrepareSave/CommitSave via the review dialog.
func (e *VEHEditor) SaveFile(path string) error {
	candidate, err := e.PrepareSave(path)
	if err != nil {
		return err
	}
	return e.CommitSave(path, candidate)
}

func (e *VEHEditor) SetCurrentPath(path string) {
	e.currentPath = path
}

// Dirty tracking — the flag mirrors the session's derived dirty
// state so tab titles and close/quit guards stay in sync.
func (e *VEHEditor) SetOnDirtyChanged(f func(bool)) { e.onDirtyChanged = f }
func (e *VEHEditor) IsDirty() bool                  { return e.isDirty }

func (e *VEHEditor) setDirtyState(d bool) {
	if e.isDirty != d {
		e.isDirty = d
		if e.onDirtyChanged != nil {
			e.onDirtyChanged(d)
		}
	}
}

func (e *VEHEditor) MarkClean() {
	e.session.MarkClean()
	e.setDirtyState(e.session.DerivedDirty())
}

// markDirty propagates the dirty flag + fires source-changed pushes
// so the SourcePanel re-renders. The loading guard keeps programmatic
// UI syncs (load / restore / undo) from dirtying the document or
// mutating models through generate-from-UI paths. Wired into every
// Entry OnChanged + Select OnChanged handler in createUI.
func (e *VEHEditor) markDirty() {
	if e.loading {
		return
	}
	e.setDirtyState(true)
	e.session.noteUserEdit()
	e.fireSourceChanged()
}

func (e *VEHEditor) fireSourceChanged() {
	if !e.loading {
		e.sourceSubs.fire()
	}
}

func (e *VEHEditor) GenerateSource() string {
	if e.vehicle == nil {
		return ""
	}
	if !e.loading {
		e.updateVehicleFromUI()
	}
	content, err := parsers.GenerateVEH(e.vehicle)
	if err != nil {
		return "// generate error: " + err.Error()
	}
	return content
}

// SetOnSourceChanged keeps legacy single-callback semantics.
func (e *VEHEditor) SetOnSourceChanged(f func()) { e.sourceSubs.reset(f) }

// AddSourceListener lets mirrors and panels subscribe without
// stealing each other's push notifications.
func (e *VEHEditor) AddSourceListener(fn func()) func() {
	return e.sourceSubs.add(fn)
}

// sessionRender snapshots the working state (UI folded in).
func (e *VEHEditor) sessionRender() string { return e.GenerateSource() }

// sessionRestore swaps a snapshot back into the working model
// in-memory. Path, baseline and dirty bookkeeping are untouched.
func (e *VEHEditor) sessionRestore(src string) error {
	names, err := parsers.DefinitionNames(src)
	if err != nil || len(names) == 0 {
		return fmt.Errorf("restore vehicle definitions: %v", err)
	}
	if e.session.SelectedDef < 0 || e.session.SelectedDef >= len(names) {
		return fmt.Errorf("restore vehicle definition %d: source has %d definitions", e.session.SelectedDef+1, len(names))
	}
	veh, err := parsers.ParseVEHDefinition(src, e.session.SelectedDef)
	if err != nil {
		return err
	}
	e.loading = true
	e.docSource = src
	e.defNames = names
	e.vehicle = veh
	e.updateUI()
	e.loading = false
	e.refreshDefinitionBar()
	e.fireSourceChanged()
	return nil
}

// and marked dirty since the working state diverges from the baseline.
func (e *VEHEditor) ImportJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		e.lastError = fmt.Sprintf("Failed to read JSON: %v", err)
		return err
	}
	veh := parsers.NewVehicleData()
	if err := json.Unmarshal(data, veh); err != nil {
		e.lastError = fmt.Sprintf("Failed to parse JSON: %v", err)
		return err
	}
	e.session.beginDiscreteChange()
	e.vehicle = veh
	e.docSource = ""
	e.defNames = []string{}
	e.session.SelectedDef = 0
	e.loading = true
	e.updateUI()
	e.loading = false
	e.session.endDiscreteChange()
	e.refreshDefinitionBar()
	e.fireSourceChanged()
	e.lastError = ""
	return nil
}

// Validate checks the current definition round-trips through the
// parser and flags identity problems the engine cares about.
func (e *VEHEditor) Validate() []string {
	e.updateVehicleFromUI()
	var issues []string
	gen, err := parsers.GenerateVEH(e.vehicle)
	if err != nil {
		issues = append(issues, fmt.Sprintf("source generation failed: %v", err))
		return issues
	}
	if _, err := parsers.ParseVEHDefinition(gen, e.session.SelectedDef); err != nil {
		issues = append(issues, fmt.Sprintf("generated source does not re-parse: %v", err))
	}
	if strings.TrimSpace(e.vehicle.Name) == "" {
		issues = append(issues, "vehicle has no name — the engine needs `name` to register the vehicle")
	}
	return issues
}
func (e *VEHEditor) Session() *DocumentSession { return e.session }
func (e *VEHEditor) CurrentSource() string     { return e.sessionRender() }
func (e *VEHEditor) Undo() bool                { return e.session.Undo() }
func (e *VEHEditor) Redo() bool                { return e.session.Redo() }
func (e *VEHEditor) CanUndo() bool             { return e.session.CanUndo() }
func (e *VEHEditor) CanRedo() bool             { return e.session.CanRedo() }

// ApplySourceText parses src in-memory (no temp files, no LoadFile)
// and swaps it into the working model. A failed parse leaves the
func (e *VEHEditor) ApplySourceText(src string) error {
	if err := validateSourceStructure(src); err != nil {
		return err
	}
	names, err := parsers.DefinitionNames(src)
	if err != nil || len(names) == 0 {
		return fmt.Errorf("parse vehicle definitions: %v", err)
	}
	if e.session.SelectedDef < 0 || e.session.SelectedDef >= len(names) {
		return fmt.Errorf("definition %d out of range (%d definitions)", e.session.SelectedDef+1, len(names))
	}
	veh, err := parsers.ParseVEHDefinition(src, e.session.SelectedDef)
	if err != nil {
		return err
	}
	e.session.beginDiscreteChange()
	e.docSource = src
	e.defNames = names
	e.vehicle = veh
	e.loading = true
	e.updateUI()
	e.loading = false
	e.session.endDiscreteChange()
	e.refreshDefinitionBar()
	e.fireSourceChanged()
	return nil
}
func (e *VEHEditor) DefinitionNames() []string { return e.defNames }

// SelectedDefinition reports the block currently driving the form.
func (e *VEHEditor) SelectedDefinition() int { return e.session.SelectedDef }

// SelectDefinition switches the edited block, folding the current
// block's edits back into docSource first so sibling definitions are
// preserved byte-for-byte by the AST-backed generator. Undoable.
func (e *VEHEditor) SelectDefinition(index int) error {
	if index < 0 || index >= len(e.defNames) {
		return fmt.Errorf("definition %d out of range (%d definitions)", index+1, len(e.defNames))
	}
	if index == e.session.SelectedDef {
		return nil
	}
	cur, err := parsers.GenerateVEH(e.vehicle)
	if err != nil {
		return fmt.Errorf("cannot switch definition: %v", err)
	}
	veh, err := parsers.ParseVEHDefinition(cur, index)
	if err != nil {
		return fmt.Errorf("cannot parse definition %q: %v", e.defNames[index], err)
	}
	e.session.beginDiscreteChangeFrom(cur)
	e.docSource = cur
	e.session.SelectedDef = index
	e.vehicle = veh
	e.loading = true
	e.updateUI()
	e.loading = false
	e.session.endDiscreteChange()
	e.refreshDefinitionBar()
	e.fireSourceChanged()
	return nil
}

func (e *VEHEditor) refreshDefinitionBar() {
	if e.defSelect == nil || e.defSummary == nil {
		return
	}
	if len(e.defNames) <= 1 {
		e.defSelect.Options = e.defNames
		e.defSelect.ClearSelected()
		e.defSummary.SetText("")
		e.defSelect.Refresh()
		return
	}
	e.defSelect.Options = e.defNames
	e.defSelect.SetSelected(e.defNames[e.session.SelectedDef])
	e.defSummary.SetText(fmt.Sprintf("editing %d of %d", e.session.SelectedDef+1, len(e.defNames)))
	e.defSelect.Refresh()
}

func (e *VEHEditor) WriteContent(w io.Writer) {
	e.SaveToWriter(w)
}

func (e *VEHEditor) ExportJSON(path string) error {
	e.updateVehicleFromUI()
	data, err := json.MarshalIndent(e.vehicle, "", "  ")
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
