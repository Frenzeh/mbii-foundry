package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

var KnownForcePowerTargets = []string{
	"FP_HEAL",
	"FP_LEVITATION",
	"FP_SPEED",
	"FP_PUSH",
	"FP_PULL",
	"FP_TELEPATHY",
	"FP_GRIP",
	"FP_LIGHTNING",
	"FP_RAGE",
	"FP_PROTECT",
	"FP_ABSORB",
	"FP_TEAM_HEAL",
	"FP_TEAM_FORCE",
	"FP_DRAIN",
	"FP_SEE",
	"FP_SABER_OFFENSE",
	"FP_SABER_DEFENSE",
	"FP_SABERTHROW",
	"FP_DESTRUCTION",
	"FP_DEADLYSIGHT",
	"FP_BLIND",
	"FP_STASIS",
	"FP_REPULSE",
}

type ForceInfoUI struct {
	editor    *MBCHEditor
	container *container.Split

	forceList  *widget.List
	emptyState *fyne.Container
	detailPane *fyne.Container

	forceToReplaceSelect *widget.Select
	iconPreview          *canvas.Image
	iconEntry            *widget.Entry
	forcePowerNameEntry  *widget.Entry
	startSoundEntry      *widget.Entry
	loopSoundEntry       *widget.Entry

	currentForceIndex int
}

func NewForceInfoUI(editor *MBCHEditor) *ForceInfoUI {
	ui := &ForceInfoUI{editor: editor, currentForceIndex: -1}
	ui.createUI()
	return ui
}

func (ui *ForceInfoUI) browseAsset(entry *widget.Entry, assetType AssetType) {
	if ui.editor.app != nil {
		ui.editor.app.showFilePickerForEntry(entry, fmt.Sprintf("Select %s", assetType), assetType)
	}
}

func (ui *ForceInfoUI) createUI() {
	// --- Left pane: force list ---
	ui.forceList = widget.NewList(
		func() int { return len(ui.editor.character.ForceOverrides) },
		func() fyne.CanvasObject {
			iconTile := canvas.NewImageFromResource(theme.FileImageIcon())
			iconTile.FillMode = canvas.ImageFillContain
			iconTile.ScaleMode = canvas.ImageScaleSmooth
			iconTile.SetMinSize(fyne.NewSize(28, 28))

			target := widget.NewLabelWithStyle("FP_...",
				fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
			subtitle := widget.NewLabelWithStyle("",
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

			return container.NewBorder(nil, nil,
				container.NewGridWrap(fyne.NewSize(28, 28), iconTile),
				nil,
				container.NewVBox(target, subtitle),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(ui.editor.character.ForceOverrides) {
				return
			}
			fi := ui.editor.character.ForceOverrides[id]
			border := obj.(*fyne.Container)

			// Left cell: icon
			if len(border.Objects) > 1 {
				if leftCell, ok := border.Objects[1].(*fyne.Container); ok && len(leftCell.Objects) > 0 {
					if iconTile, ok := leftCell.Objects[0].(*canvas.Image); ok {
						iconTile.Resource = ui.resolveRowIcon(fi)
						iconTile.Refresh()
					}
				}
			}

			// Center cell: text stack
			if len(border.Objects) > 0 {
				if center, ok := border.Objects[0].(*fyne.Container); ok && len(center.Objects) >= 2 {
					targetText := fi.ForceToReplace
					if targetText == "" {
						targetText = "FP_..."
					}
					center.Objects[0].(*widget.Label).SetText(targetText)
					name := fi.ForcePowerName
					if name == "" {
						name = "(no custom name)"
					}
					center.Objects[1].(*widget.Label).SetText(name)
				}
			}
		},
	)
	ui.forceList.OnSelected = func(id widget.ListItemID) {
		ui.currentForceIndex = id
		ui.loadForceDetails(id)
		ui.showDetailPane()
	}

	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), ui.addForce)
	dupBtn := widget.NewButtonWithIcon("Duplicate", theme.ContentCopyIcon(), ui.duplicateForce)
	removeBtn := widget.NewButtonWithIcon("Remove", theme.ContentRemoveIcon(), ui.removeForce)
	dupBtn.Importance = widget.LowImportance

	actionsRow := container.NewHBox(addBtn, dupBtn, removeBtn)

	listPane := container.NewBorder(
		container.NewPadded(actionsRow),
		nil, nil, nil,
		ui.forceList,
	)

	// --- Right pane: detail form ---
	ui.forceToReplaceSelect = widget.NewSelect(KnownForcePowerTargets, func(s string) {
		ui.refreshIconPreview(ui.iconEntry.Text)
		ui.onDetailChanged(s)
	})
	ui.forceToReplaceSelect.PlaceHolder = "Which FP_ to override"

	ui.iconPreview = canvas.NewImageFromResource(theme.FileImageIcon())
	ui.iconPreview.FillMode = canvas.ImageFillContain
	ui.iconPreview.ScaleMode = canvas.ImageScaleSmooth
	ui.iconPreview.SetMinSize(fyne.NewSize(48, 48))

	ui.iconEntry = NewInputEntry()
	ui.iconEntry.OnChanged = func(s string) {
		ui.refreshIconPreview(s)
		ui.onDetailChanged(s)
	}
	browseIconBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		ui.browseAsset(ui.iconEntry, AssetTypeIcon)
	})

	ui.forcePowerNameEntry = NewInputEntry()
	ui.forcePowerNameEntry.OnChanged = ui.onDetailChanged

	ui.startSoundEntry = NewInputEntry()
	ui.startSoundEntry.OnChanged = ui.onDetailChanged
	browseStartSoundBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		ui.browseAsset(ui.startSoundEntry, AssetTypeSound)
	})

	ui.loopSoundEntry = NewInputEntry()
	ui.loopSoundEntry.OnChanged = ui.onDetailChanged
	browseLoopSoundBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		ui.browseAsset(ui.loopSoundEntry, AssetTypeSound)
	})

	identityHeader := widget.NewLabelWithStyle("Identity & Visuals", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	soundsHeader := widget.NewLabelWithStyle("Audio FX", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	iconRow := container.NewBorder(nil, nil, nil, browseIconBtn, ui.iconEntry)
	startSoundRow := container.NewBorder(nil, nil, nil, browseStartSoundBtn, ui.startSoundEntry)
	loopSoundRow := container.NewBorder(nil, nil, nil, browseLoopSoundBtn, ui.loopSoundEntry)

	identityCard := NewTilePanel(container.NewVBox(
		identityHeader,
		container.NewHBox(ui.iconPreview, container.NewVBox(
			widget.NewLabelWithStyle("Live Icon Preview", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}),
		)),
		widget.NewForm(
			widget.NewFormItem("Force To Replace", ui.forceToReplaceSelect),
			widget.NewFormItem("Icon Path", iconRow),
			widget.NewFormItem("Force Power Name", ui.forcePowerNameEntry),
		),
	), TileOpts{FillAlpha: 18, StrokeAlpha: 60, Padded: true})

	soundsCard := NewTilePanel(container.NewVBox(
		soundsHeader,
		widget.NewForm(
			widget.NewFormItem("Start Sound", startSoundRow),
			widget.NewFormItem("Loop Sound", loopSoundRow),
		),
	), TileOpts{FillAlpha: 18, StrokeAlpha: 60, Padded: true})

	formBody := container.NewVBox(identityCard, soundsCard)
	detailScroll := container.NewVScroll(formBody)

	ui.emptyState = container.NewCenter(
		NewEmptyStateTile("NO FORCE MOD SELECTED",
			"Select a Force Power override from the list on the left, or click Add to customize force power names, HUD icons, and audio.",
			"Add Force Override", ui.addForce),
	)

	ui.detailPane = container.NewStack(ui.emptyState, detailScroll)

	ui.container = container.NewHSplit(listPane, ui.detailPane)
	ui.container.SetOffset(0.3)
}

func (ui *ForceInfoUI) resolveRowIcon(fi parsers.ForceInfo) fyne.Resource {
	// 1. Explicit Icon field
	if fi.Icon != "" {
		if ui.editor.assetBrowser != nil {
			if res := ui.editor.assetBrowser.LoadIconResource(fi.Icon); res != nil {
				return res
			}
		}
		if img, ok := LoadGameIcon(nil, fi.Icon); ok {
			return staticPNGResource("fi_icon.png", img)
		}
		if img, ok := LoadGameIcon(nil, "gfx/mp/"+fi.Icon); ok {
			return staticPNGResource("fi_icon.png", img)
		}
		if img, ok := LoadGameIcon(nil, "gfx/hud/"+fi.Icon); ok {
			return staticPNGResource("fi_icon.png", img)
		}
	}

	// 2. Base ForceToReplace
	if fi.ForceToReplace != "" && ui.editor.iconResolver != nil {
		path := ui.editor.iconResolver.ResolveAttributeIcon(fi.ForceToReplace)
		if path == "" || strings.HasPrefix(path, "gfx/menus/alpha/") {
			path = ui.editor.iconResolver.ResolveAttributeIcon("MB_ATT_" + fi.ForceToReplace)
		}
		if ui.editor.assetBrowser != nil && path != "" {
			if res := ui.editor.assetBrowser.LoadIconResource(path); res != nil {
				return res
			}
		}
		if path != "" {
			if img, ok := LoadGameIcon(nil, path); ok {
				return staticPNGResource("fp_base.png", img)
			}
		}
	}

	return theme.FileImageIcon()
}

func (ui *ForceInfoUI) showDetailPane() {
	if ui.detailPane == nil || len(ui.detailPane.Objects) != 2 {
		return
	}
	ui.detailPane.Objects[0].Hide()
	ui.detailPane.Objects[1].Show()
	ui.detailPane.Refresh()
}

func (ui *ForceInfoUI) hideDetailPane() {
	if ui.detailPane == nil || len(ui.detailPane.Objects) != 2 {
		return
	}
	ui.detailPane.Objects[0].Show()
	ui.detailPane.Objects[1].Hide()
	ui.detailPane.Refresh()
}

func (ui *ForceInfoUI) refreshIconPreview(path string) {
	if path != "" {
		if ui.editor.assetBrowser != nil {
			if res := ui.editor.assetBrowser.LoadIconResource(path); res != nil {
				ui.iconPreview.Resource = res
				ui.iconPreview.Refresh()
				return
			}
		}
		if img, ok := LoadGameIcon(nil, path); ok {
			ui.iconPreview.Resource = staticPNGResource("fi_prev.png", img)
			ui.iconPreview.Refresh()
			return
		}
		if img, ok := LoadGameIcon(nil, "gfx/mp/"+path); ok {
			ui.iconPreview.Resource = staticPNGResource("fi_prev.png", img)
			ui.iconPreview.Refresh()
			return
		}
		if img, ok := LoadGameIcon(nil, "gfx/hud/"+path); ok {
			ui.iconPreview.Resource = staticPNGResource("fi_prev.png", img)
			ui.iconPreview.Refresh()
			return
		}
	} else if ui.currentForceIndex >= 0 && ui.currentForceIndex < len(ui.editor.character.ForceOverrides) {
		fi := ui.editor.character.ForceOverrides[ui.currentForceIndex]
		ui.iconPreview.Resource = ui.resolveRowIcon(fi)
		ui.iconPreview.Refresh()
		return
	}

	ui.iconPreview.Resource = theme.FileImageIcon()
	ui.iconPreview.Refresh()
}

func (ui *ForceInfoUI) GetContent() fyne.CanvasObject {
	return ui.container
}

func (ui *ForceInfoUI) UpdateUI() {
	ui.forceList.Refresh()
	if ui.currentForceIndex != -1 && ui.currentForceIndex < len(ui.editor.character.ForceOverrides) {
		ui.loadForceDetails(ui.currentForceIndex)
		ui.showDetailPane()
	} else {
		ui.clearDetails()
		ui.hideDetailPane()
	}
}

func (ui *ForceInfoUI) addForce() {
	ui.editor.character.ForceOverrides = append(ui.editor.character.ForceOverrides, parsers.ForceInfo{
		ForceToReplace: "FP_PUSH",
		ExtraFields:    make(map[string]string),
	})
	ui.editor.markDirty()
	ui.forceList.Refresh()
	ui.forceList.Select(len(ui.editor.character.ForceOverrides) - 1)
}

func (ui *ForceInfoUI) duplicateForce() {
	if ui.currentForceIndex == -1 || ui.currentForceIndex >= len(ui.editor.character.ForceOverrides) {
		return
	}
	src := ui.editor.character.ForceOverrides[ui.currentForceIndex]
	clone := src
	clone.ExtraFields = map[string]string{}
	for k, v := range src.ExtraFields {
		clone.ExtraFields[k] = v
	}
	ui.editor.character.ForceOverrides = append(ui.editor.character.ForceOverrides, clone)
	ui.editor.markDirty()
	ui.forceList.Refresh()
	ui.forceList.Select(len(ui.editor.character.ForceOverrides) - 1)
}

func (ui *ForceInfoUI) removeForce() {
	if ui.currentForceIndex != -1 && ui.currentForceIndex < len(ui.editor.character.ForceOverrides) {
		ui.editor.character.ForceOverrides = append(
			ui.editor.character.ForceOverrides[:ui.currentForceIndex],
			ui.editor.character.ForceOverrides[ui.currentForceIndex+1:]...,
		)
		ui.currentForceIndex = -1
		ui.editor.markDirty()
		ui.forceList.Refresh()
		ui.clearDetails()
		ui.hideDetailPane()
	}
}

func (ui *ForceInfoUI) loadForceDetails(index int) {
	if index < 0 || index >= len(ui.editor.character.ForceOverrides) {
		ui.clearDetails()
		return
	}

	fi := ui.editor.character.ForceOverrides[index]
	ui.forceToReplaceSelect.SetSelected(fi.ForceToReplace)
	ui.iconEntry.SetText(fi.Icon)
	ui.forcePowerNameEntry.SetText(fi.ForcePowerName)
	ui.startSoundEntry.SetText(fi.StartSound)
	ui.loopSoundEntry.SetText(fi.LoopSound)
	ui.refreshIconPreview(fi.Icon)
}

func (ui *ForceInfoUI) clearDetails() {
	ui.forceToReplaceSelect.ClearSelected()
	ui.iconEntry.SetText("")
	ui.forcePowerNameEntry.SetText("")
	ui.startSoundEntry.SetText("")
	ui.loopSoundEntry.SetText("")
	ui.iconPreview.Resource = theme.FileImageIcon()
	ui.iconPreview.Refresh()
}

func (ui *ForceInfoUI) onDetailChanged(s string) {
	_ = s
	if ui.currentForceIndex == -1 || ui.currentForceIndex >= len(ui.editor.character.ForceOverrides) {
		return
	}

	fi := &ui.editor.character.ForceOverrides[ui.currentForceIndex]
	fi.ForceToReplace = ui.forceToReplaceSelect.Selected
	fi.Icon = ui.iconEntry.Text
	fi.ForcePowerName = ui.forcePowerNameEntry.Text
	fi.StartSound = ui.startSoundEntry.Text
	fi.LoopSound = ui.loopSoundEntry.Text

	ui.editor.markDirty()
	ui.forceList.RefreshItem(ui.currentForceIndex)
}
