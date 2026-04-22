package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	
	"github.com/mbii-holocron/fa_creator/parsers"
)

type VEHEditor struct {
	vehicle     *parsers.VehicleData
	currentPath string
	container   *fyne.Container
	fileManager *FileManager
	lastError   string
	onHover     func(string, string)

	nameEntry     *widget.Entry
	typeSelect    *widget.Select
	modelEntry    *widget.Entry
	skinEntry     *widget.Entry
	
speedEntry    *widget.Entry
turboEntry    *widget.Entry
accelEntry    *widget.Entry
decelEntry    *widget.Entry
strafeEntry   *widget.Entry
brakingEntry  *widget.Entry
	
armorEntry    *widget.Entry
shieldsEntry  *widget.Entry
	
weaponsEntry  *widget.Entry
	
assetBrowser  *AssetBrowser
holocronClient *HolocronClient
app           *App 
	
sourceView    *widget.Entry
}

var VehicleTypes = []string{"VH_SPEEDER", "VH_ANIMAL", "VH_WALKER", "VH_FIGHTER"}

func NewVEHEditor(app *App) *VEHEditor {
	e := &VEHEditor{
		vehicle:     parsers.NewVehicleData(),
		fileManager: app.fileManager,
		app: app,
	}
	e.createUI()
	return e
}

func (e *VEHEditor) SetOnHover(f func(string, string)) { e.onHover = f }
func (e *VEHEditor) SetAssetBrowser(ab *AssetBrowser) { e.assetBrowser = ab }
func (e *VEHEditor) SetHolocronClient(client *HolocronClient) { e.holocronClient = client }

func (e *VEHEditor) createUI() {
	e.nameEntry = widget.NewEntry()
	e.typeSelect = widget.NewSelect(VehicleTypes, func(s string) { e.vehicle.Type = s })
	e.modelEntry = widget.NewEntry()
	e.skinEntry = widget.NewEntry()
	
browseModelBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		if e.app != nil { e.app.showFilePickerForEntry(e.modelEntry, "Select Model", AssetTypeModel) }
	})
	identityForm := widget.NewForm(
		widget.NewFormItem("Vehicle Name", e.nameEntry),
		widget.NewFormItem("Type", e.typeSelect),
		widget.NewFormItem("Model", container.NewBorder(nil, nil, nil, browseModelBtn, e.modelEntry)),
		widget.NewFormItem("Skin", e.skinEntry),
	)
	
	e.speedEntry = widget.NewEntry(); e.turboEntry = widget.NewEntry(); e.accelEntry = widget.NewEntry()
	e.decelEntry = widget.NewEntry(); e.strafeEntry = widget.NewEntry(); e.brakingEntry = widget.NewEntry()
	
	statsForm := widget.NewForm(
		widget.NewFormItem("Max Speed", e.speedEntry),
		widget.NewFormItem("Turbo Speed", e.turboEntry),
		widget.NewFormItem("Acceleration", e.accelEntry),
		widget.NewFormItem("Deceleration", e.decelEntry),
		widget.NewFormItem("Strafe %", e.strafeEntry),
		widget.NewFormItem("Braking", e.brakingEntry),
	)
	
	e.armorEntry = widget.NewEntry(); e.shieldsEntry = widget.NewEntry()
	e.weaponsEntry = widget.NewEntry()
	
	combatForm := widget.NewForm(
		widget.NewFormItem("Armor", e.armorEntry),
		widget.NewFormItem("Shields", e.shieldsEntry),
		widget.NewFormItem("Weapons", e.weaponsEntry),
	)
	
	e.sourceView = widget.NewMultiLineEntry()
	e.sourceView.TextStyle = fyne.TextStyle{Monospace: true}
	sourceTab := container.NewMax(container.NewScroll(e.sourceView))

	tabs := container.NewAppTabs(
		container.NewTabItem("Specs", container.NewVScroll(container.NewVBox(
			widget.NewCard("Identity", "", identityForm),
			widget.NewCard("Movement", "", statsForm),
			widget.NewCard("Combat", "", combatForm),
		))),
		container.NewTabItem("Source", sourceTab),
	)
	
	tabs.OnSelected = func(tab *container.TabItem) {
		if tab.Text == "Source" {
			e.updateSourceView()
		}
	}

	e.container = container.NewMax(tabs)
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
func (e *VEHEditor) GetCurrentPath() string { return e.currentPath }
func (e *VEHEditor) GetRecentFiles() []RecentFile { return e.fileManager.GetRecentFiles() }

func (e *VEHEditor) LoadFile(path string) error {
	file, err := os.Open(path); if err != nil { return err }; defer file.Close()
	content, err := io.ReadAll(file); if err != nil { e.lastError = fmt.Sprintf("Failed to read file: %v", err); return err }
	
	veh, err := parsers.ParseVEH(string(content))
	if err != nil {
		dialog.ShowError(fmt.Errorf("Error parsing file: %v\nProceeding with partial data.", err), fyne.CurrentApp().Driver().AllWindows()[0])
		return err 
	}
	
e.vehicle = veh
	e.currentPath = path
	e.updateUI()
	if e.fileManager != nil { e.fileManager.AddRecentFile(path) }
	e.lastError = ""
	return nil
}

func (e *VEHEditor) SaveToWriter(w io.Writer) error {
	e.updateVehicleFromUI()
	content, err := parsers.GenerateVEH(e.vehicle)
	if err != nil { return err }
	_, err = w.Write([]byte(content))
	return err
}

func (e *VEHEditor) SaveFile(path string) error {
	file, err := os.Create(path); if err != nil { return err }; defer file.Close()
	
	if err := e.SaveToWriter(file); err != nil {
		return err
	}
	
	e.currentPath = path
	e.lastError = ""
	if e.fileManager != nil { e.fileManager.AddRecentFile(path) }
	return nil
}

func (e *VEHEditor) SetCurrentPath(path string) {
	e.currentPath = path
}

func (e *VEHEditor) MarkClean() {}
func (e *VEHEditor) SetOnDirtyChanged(f func(bool)) {}
func (e *VEHEditor) IsDirty() bool { return false }

func (e *VEHEditor) WriteContent(w io.Writer) {
	e.SaveToWriter(w)
}

func (e *VEHEditor) ExportJSON(path string) error { return nil } // Todo
func (e *VEHEditor) ImportJSON(path string) error { return nil } // Todo
func (e *VEHEditor) Validate() []string { return []string{} } // Todo