package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	
	"github.com/mbii-holocron/fa_creator/parsers"
)

type ForceInfoUI struct {
	editor *MBCHEditor
	container *container.Split
	
	forceList *widget.List
	detailForm *widget.Form
	
	forceToReplaceEntry *widget.Entry
	iconEntry *widget.Entry
	forcePowerNameEntry *widget.Entry
	startSoundEntry *widget.Entry
	loopSoundEntry *widget.Entry
	
	currentForceIndex int
}

func NewForceInfoUI(editor *MBCHEditor) *ForceInfoUI {
	ui := &ForceInfoUI{editor: editor, currentForceIndex: -1}
	ui.createUI()
	return ui
}

func (ui *ForceInfoUI) createUI() {
	ui.forceList = widget.NewList(
		func() int { return len(ui.editor.character.ForceOverrides) },
		func() fyne.CanvasObject {
			return container.NewHBox(widget.NewIcon(theme.ContentCopyIcon()), widget.NewLabel("Force Power Name"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*fyne.Container).Objects[1].(*widget.Label).SetText(ui.editor.character.ForceOverrides[id].ForceToReplace)
		},
	)
	ui.forceList.OnSelected = func(id widget.ListItemID) {
		ui.currentForceIndex = id
		ui.loadForceDetails(id)
	}
	
	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), ui.addForce)
	removeBtn := widget.NewButtonWithIcon("Remove", theme.ContentRemoveIcon(), ui.removeForce)
	
	listPane := container.NewBorder(
		container.NewHBox(addBtn, removeBtn),
		nil, nil, nil,
		ui.forceList,
	)

	ui.forceToReplaceEntry = widget.NewEntry(); ui.forceToReplaceEntry.OnChanged = ui.onDetailChanged
	ui.iconEntry = widget.NewEntry(); ui.iconEntry.OnChanged = ui.onDetailChanged
	ui.forcePowerNameEntry = widget.NewEntry(); ui.forcePowerNameEntry.OnChanged = ui.onDetailChanged
	ui.startSoundEntry = widget.NewEntry(); ui.startSoundEntry.OnChanged = ui.onDetailChanged
	ui.loopSoundEntry = widget.NewEntry(); ui.loopSoundEntry.OnChanged = ui.onDetailChanged
	
	ui.detailForm = widget.NewForm(
		widget.NewFormItem("Force To Replace", ui.forceToReplaceEntry),
		widget.NewFormItem("Icon", ui.iconEntry),
		widget.NewFormItem("Force Power Name", ui.forcePowerNameEntry),
		widget.NewFormItem("Start Sound", ui.startSoundEntry),
		widget.NewFormItem("Loop Sound", ui.loopSoundEntry),
	)
	
	ui.container = container.NewHSplit(listPane, container.NewVScroll(ui.detailForm))
	ui.container.SetOffset(0.3)
}

func (ui *ForceInfoUI) GetContent() fyne.CanvasObject {
	return ui.container
}

func (ui *ForceInfoUI) UpdateUI() {
	ui.forceList.Refresh()
	if ui.currentForceIndex != -1 && ui.currentForceIndex < len(ui.editor.character.ForceOverrides) {
		ui.loadForceDetails(ui.currentForceIndex)
	} else {
		ui.clearDetails()
	}
}

func (ui *ForceInfoUI) addForce() {
	ui.editor.character.ForceOverrides = append(ui.editor.character.ForceOverrides, parsers.ForceInfo{ExtraFields: make(map[string]string)})
	ui.forceList.Refresh()
	ui.forceList.Select(len(ui.editor.character.ForceOverrides) - 1)
}

func (ui *ForceInfoUI) removeForce() {
	if ui.currentForceIndex != -1 {
		ui.editor.character.ForceOverrides = append(ui.editor.character.ForceOverrides[:ui.currentForceIndex], ui.editor.character.ForceOverrides[ui.currentForceIndex+1:]...)
		ui.currentForceIndex = -1
		ui.forceList.Refresh()
		ui.clearDetails()
	}
}

func (ui *ForceInfoUI) loadForceDetails(index int) {
	if index < 0 || index >= len(ui.editor.character.ForceOverrides) {
		ui.clearDetails()
		return
	}
	
	fi := ui.editor.character.ForceOverrides[index]
	ui.forceToReplaceEntry.SetText(fi.ForceToReplace)
	ui.iconEntry.SetText(fi.Icon)
	ui.forcePowerNameEntry.SetText(fi.ForcePowerName)
	ui.startSoundEntry.SetText(fi.StartSound)
	ui.loopSoundEntry.SetText(fi.LoopSound)
}

func (ui *ForceInfoUI) clearDetails() {
	ui.forceToReplaceEntry.SetText("")
	ui.iconEntry.SetText("")
	ui.forcePowerNameEntry.SetText("")
	ui.startSoundEntry.SetText("")
	ui.loopSoundEntry.SetText("")
}

func (ui *ForceInfoUI) onDetailChanged(s string) {
	if ui.currentForceIndex == -1 || ui.currentForceIndex >= len(ui.editor.character.ForceOverrides) { return }
	
	fi := &ui.editor.character.ForceOverrides[ui.currentForceIndex]
	fi.ForceToReplace = ui.forceToReplaceEntry.Text
	fi.Icon = ui.iconEntry.Text
	fi.ForcePowerName = ui.forcePowerNameEntry.Text
	fi.StartSound = ui.startSoundEntry.Text
	fi.LoopSound = ui.loopSoundEntry.Text

	ui.forceList.RefreshItem(ui.currentForceIndex)
}
