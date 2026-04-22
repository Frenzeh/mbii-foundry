package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	
	"github.com/mbii-holocron/fa_creator/parsers"
)

type WeaponInfoUI struct {
	editor *MBCHEditor
	container *container.Split
	
	weaponList *widget.List
	detailForm *widget.Form
	
	weaponToReplaceEntry *widget.Entry
	weaponBasedOffEntry *widget.Entry
	newWorldModelEntry *widget.Entry
	newViewModelEntry *widget.Entry
	iconEntry *widget.Entry
	weaponNameEntry *widget.Entry
	muzzleEffectEntry *widget.Entry
	altMuzzleEffectEntry *widget.Entry
	missileEffectEntry *widget.Entry
	altMissileEffectEntry *widget.Entry
	flashSound0Entry *widget.Entry
	altFlashSound0Entry *widget.Entry
	chargeSoundEntry *widget.Entry
	customAmmoEntry *widget.Entry
	clipSizeEntry *widget.Entry
	reloadTimeModifierEntry *widget.Entry
	
	currentWeaponIndex int
}

func NewWeaponInfoUI(editor *MBCHEditor) *WeaponInfoUI {
	ui := &WeaponInfoUI{editor: editor, currentWeaponIndex: -1}
	ui.createUI()
	return ui
}

func (ui *WeaponInfoUI) browseAsset(entry *widget.Entry, assetType AssetType) {
	if ui.editor.app != nil {
		ui.editor.app.showFilePickerForEntry(entry, fmt.Sprintf("Select %s", assetType), assetType)
	}
}

func (ui *WeaponInfoUI) createUI() {
	ui.weaponList = widget.NewList(
		func() int { return len(ui.editor.character.WeaponOverrides) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.ContentCopyIcon()), widget.NewLabel("Weapon Name"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*fyne.Container).Objects[1].(*widget.Label).SetText(ui.editor.character.WeaponOverrides[id].WeaponToReplace)
		},
	)
	ui.weaponList.OnSelected = func(id widget.ListItemID) {
		ui.currentWeaponIndex = id
		ui.loadWeaponDetails(id)
	}
	
	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), ui.addWeapon)
	removeBtn := widget.NewButtonWithIcon("Remove", theme.ContentRemoveIcon(), ui.removeWeapon)
	
	listPane := container.NewBorder(
		container.NewHBox(addBtn, removeBtn),
		nil, nil, nil,
		ui.weaponList,
	)

	ui.weaponToReplaceEntry = widget.NewEntry(); ui.weaponToReplaceEntry.OnChanged = ui.onDetailChanged
	ui.weaponBasedOffEntry = widget.NewEntry(); ui.weaponBasedOffEntry.OnChanged = ui.onDetailChanged
	ui.newWorldModelEntry = widget.NewEntry(); ui.newWorldModelEntry.OnChanged = ui.onDetailChanged
	ui.newViewModelEntry = widget.NewEntry(); ui.newViewModelEntry.OnChanged = ui.onDetailChanged
	ui.iconEntry = widget.NewEntry(); ui.iconEntry.OnChanged = ui.onDetailChanged
	ui.weaponNameEntry = widget.NewEntry(); ui.weaponNameEntry.OnChanged = ui.onDetailChanged
	ui.muzzleEffectEntry = widget.NewEntry(); ui.muzzleEffectEntry.OnChanged = ui.onDetailChanged
	ui.altMuzzleEffectEntry = widget.NewEntry(); ui.altMuzzleEffectEntry.OnChanged = ui.onDetailChanged
	ui.missileEffectEntry = widget.NewEntry(); ui.missileEffectEntry.OnChanged = ui.onDetailChanged
	ui.altMissileEffectEntry = widget.NewEntry(); ui.altMissileEffectEntry.OnChanged = ui.onDetailChanged
	ui.flashSound0Entry = widget.NewEntry(); ui.flashSound0Entry.OnChanged = ui.onDetailChanged
	ui.altFlashSound0Entry = widget.NewEntry(); ui.altFlashSound0Entry.OnChanged = ui.onDetailChanged
	ui.chargeSoundEntry = widget.NewEntry(); ui.chargeSoundEntry.OnChanged = ui.onDetailChanged
	ui.customAmmoEntry = widget.NewEntry(); ui.customAmmoEntry.OnChanged = ui.onDetailChanged
	ui.clipSizeEntry = widget.NewEntry(); ui.clipSizeEntry.OnChanged = ui.onDetailChanged
	ui.reloadTimeModifierEntry = widget.NewEntry(); ui.reloadTimeModifierEntry.OnChanged = ui.onDetailChanged
	
	ui.detailForm = widget.NewForm(
		widget.NewFormItem("Weapon To Replace", ui.weaponToReplaceEntry),
		widget.NewFormItem("Weapon Based Off", ui.weaponBasedOffEntry),
		widget.NewFormItem("New World Model", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.newWorldModelEntry, AssetTypeModel) }, "Browse for World Model"), ui.newWorldModelEntry)),
		widget.NewFormItem("New View Model", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.newViewModelEntry, AssetTypeModel) }, "Browse for View Model"), ui.newViewModelEntry)),
		widget.NewFormItem("Icon", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.iconEntry, AssetTypeIcon) }, "Browse for Icon"), ui.iconEntry)),
		widget.NewFormItem("Weapon Name", ui.weaponNameEntry),
		widget.NewFormItem("Muzzle Effect", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.muzzleEffectEntry, AssetTypeEffect) }, "Browse for Muzzle Effect"), ui.muzzleEffectEntry)),
		widget.NewFormItem("Alt Muzzle Effect", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.altMuzzleEffectEntry, AssetTypeEffect) }, "Browse for Alt Muzzle Effect"), ui.altMuzzleEffectEntry)),
		widget.NewFormItem("Missile Effect", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.missileEffectEntry, AssetTypeEffect) }, "Browse for Missile Effect"), ui.missileEffectEntry)),
		widget.NewFormItem("Alt Missile Effect", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.altMissileEffectEntry, AssetTypeEffect) }, "Browse for Alt Missile Effect"), ui.altMissileEffectEntry)),
		widget.NewFormItem("Flash Sound 0", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.flashSound0Entry, AssetTypeSound) }, "Browse for Flash Sound"), ui.flashSound0Entry)),
		widget.NewFormItem("Alt Flash Sound 0", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.altFlashSound0Entry, AssetTypeSound) }, "Browse for Alt Flash Sound"), ui.altFlashSound0Entry)),
		widget.NewFormItem("Charge Sound", container.NewBorder(nil, nil, nil, NewTooltipButton("", theme.FolderOpenIcon(), func() { ui.browseAsset(ui.chargeSoundEntry, AssetTypeSound) }, "Browse for Charge Sound"), ui.chargeSoundEntry)),
		widget.NewFormItem("Custom Ammo", ui.customAmmoEntry),
		widget.NewFormItem("Clip Size", ui.clipSizeEntry),
		widget.NewFormItem("Reload Time Modifier", ui.reloadTimeModifierEntry),
	)

	
	ui.container = container.NewHSplit(listPane, container.NewVScroll(ui.detailForm))
	ui.container.SetOffset(0.3)
}

func (ui *WeaponInfoUI) GetContent() fyne.CanvasObject {
	return ui.container
}

func (ui *WeaponInfoUI) UpdateUI() {
	ui.weaponList.Refresh()
	if ui.currentWeaponIndex != -1 && ui.currentWeaponIndex < len(ui.editor.character.WeaponOverrides) {
		ui.loadWeaponDetails(ui.currentWeaponIndex)
	} else {
		ui.clearDetails()
	}
}

func (ui *WeaponInfoUI) addWeapon() {
	ui.editor.character.WeaponOverrides = append(ui.editor.character.WeaponOverrides, parsers.WeaponInfo{ExtraFields: make(map[string]string)})
	ui.weaponList.Refresh()
	ui.weaponList.Select(len(ui.editor.character.WeaponOverrides) - 1)
}

func (ui *WeaponInfoUI) removeWeapon() {
	if ui.currentWeaponIndex != -1 {
		ui.editor.character.WeaponOverrides = append(ui.editor.character.WeaponOverrides[:ui.currentWeaponIndex], ui.editor.character.WeaponOverrides[ui.currentWeaponIndex+1:]...)
		ui.currentWeaponIndex = -1
		ui.weaponList.Refresh()
		ui.clearDetails()
	}
}

func (ui *WeaponInfoUI) loadWeaponDetails(index int) {
	if index < 0 || index >= len(ui.editor.character.WeaponOverrides) {
		ui.clearDetails()
		return
	}
	
	wi := ui.editor.character.WeaponOverrides[index]
	ui.weaponToReplaceEntry.SetText(wi.WeaponToReplace)
	ui.weaponBasedOffEntry.SetText(wi.WeaponBasedOff)
	ui.newWorldModelEntry.SetText(wi.NewWorldModel)
	ui.newViewModelEntry.SetText(wi.NewViewModel)
	ui.iconEntry.SetText(wi.Icon)
	ui.weaponNameEntry.SetText(wi.WeaponName)
	ui.muzzleEffectEntry.SetText(wi.MuzzleEffect)
	ui.altMuzzleEffectEntry.SetText(wi.AltMuzzleEffect)
	ui.missileEffectEntry.SetText(wi.MissileEffect)
	ui.altMissileEffectEntry.SetText(wi.AltMissileEffect)
	ui.flashSound0Entry.SetText(wi.FlashSound0)
	ui.altFlashSound0Entry.SetText(wi.AltFlashSound0)
	ui.chargeSoundEntry.SetText(wi.ChargeSound)
	ui.customAmmoEntry.SetText(strconv.Itoa(wi.CustomAmmo))
	ui.clipSizeEntry.SetText(strconv.Itoa(wi.ClipSize))
	ui.reloadTimeModifierEntry.SetText(fmt.Sprintf("%.1f", wi.ReloadTimeModifier))
}

func (ui *WeaponInfoUI) clearDetails() {
	ui.weaponToReplaceEntry.SetText("")
	ui.weaponBasedOffEntry.SetText("")
	ui.newWorldModelEntry.SetText("")
	ui.newViewModelEntry.SetText("")
	ui.iconEntry.SetText("")
	ui.weaponNameEntry.SetText("")
	ui.muzzleEffectEntry.SetText("")
	ui.altMuzzleEffectEntry.SetText("")
	ui.missileEffectEntry.SetText("")
	ui.altMissileEffectEntry.SetText("")
	ui.flashSound0Entry.SetText("")
	ui.altFlashSound0Entry.SetText("")
	ui.chargeSoundEntry.SetText("")
	ui.customAmmoEntry.SetText("")
	ui.clipSizeEntry.SetText("")
	ui.reloadTimeModifierEntry.SetText("")
}

func (ui *WeaponInfoUI) onDetailChanged(s string) {
	if ui.currentWeaponIndex == -1 || ui.currentWeaponIndex >= len(ui.editor.character.WeaponOverrides) { return }
	
	wi := &ui.editor.character.WeaponOverrides[ui.currentWeaponIndex]
	wi.WeaponToReplace = ui.weaponToReplaceEntry.Text
	wi.WeaponBasedOff = ui.weaponBasedOffEntry.Text
	wi.NewWorldModel = ui.newWorldModelEntry.Text
	wi.NewViewModel = ui.newViewModelEntry.Text
	wi.Icon = ui.iconEntry.Text
	wi.WeaponName = ui.weaponNameEntry.Text
	wi.MuzzleEffect = ui.muzzleEffectEntry.Text
	wi.AltMuzzleEffect = ui.altMuzzleEffectEntry.Text
	wi.MissileEffect = ui.missileEffectEntry.Text
	wi.AltMissileEffect = ui.altMissileEffectEntry.Text
	wi.FlashSound0 = ui.flashSound0Entry.Text
	wi.AltFlashSound0 = ui.altFlashSound0Entry.Text
	wi.ChargeSound = ui.chargeSoundEntry.Text
	wi.CustomAmmo, _ = strconv.Atoi(ui.customAmmoEntry.Text)
	wi.ClipSize, _ = strconv.Atoi(ui.clipSizeEntry.Text)
	wi.ReloadTimeModifier, _ = strconv.ParseFloat(ui.reloadTimeModifierEntry.Text, 64)

	ui.weaponList.RefreshItem(ui.currentWeaponIndex)
}
