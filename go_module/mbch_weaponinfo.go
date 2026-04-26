package main

// WeaponInfo override editor — per-block customizer for MBII's
// `WeaponInfoN { WeaponToReplace ... }` feature, which lets a class
// swap any weapon's model, icon, name, sounds, effects, ammo, and
// reload timing without editing the global weapon defs.
//
// Layout is master/detail:
//   LEFT:  list of WeaponInfo blocks defined on the character,
//          each row showing the override's WP_* target + custom
//          name + a 28×28 icon preview.
//   RIGHT: grouped form for the selected block — Identity / Visuals
//          / Sounds / Ammo sections keep the ~20 fields legible
//          instead of one 700px tall stacked form.
//
// The override list supports add / duplicate / remove. Duplicate
// is useful because a class often swaps several variants of the
// same weapon family (h1_Talz swaps WP_SABER → spear, WP_T21 → bow,
// WP_REPEATER → snowballs, each with nearly-identical animation
// overrides).

import (
	"fmt"
	"sort"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// WeaponInfoUI is the composite widget living in the MBCH editor's
// "Weapon Mods" tab.
type WeaponInfoUI struct {
	editor    *MBCHEditor
	container *container.Split

	weaponList *widget.List
	listIcon   *canvas.Image // shared list-row icon template (replaced per row)

	// Detail form field handles — kept so UpdateUI and onDetailChanged
	// can read/write them. Organized by functional group in the form.
	weaponToReplaceSelect *widget.Select
	weaponBasedOffSelect  *widget.Select

	iconPreview       *canvas.Image // live render of the override's Icon field
	newWorldModelEntry *widget.Entry
	newViewModelEntry *widget.Entry
	iconEntry         *widget.Entry
	weaponNameEntry   *widget.Entry

	muzzleEffectEntry     *widget.Entry
	altMuzzleEffectEntry  *widget.Entry
	missileEffectEntry    *widget.Entry
	altMissileEffectEntry *widget.Entry

	flashSound0Entry    *widget.Entry
	altFlashSound0Entry *widget.Entry
	chargeSoundEntry    *widget.Entry

	customAmmoEntry         *widget.Entry
	clipSizeEntry           *widget.Entry
	reloadTimeModifierEntry *widget.Entry

	// Empty-state card shown when no override is selected. Keeps the
	// right pane from looking busted before the user clicks a row.
	emptyState *fyne.Container
	detailPane *fyne.Container

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

// weaponPickOptions builds the WP_* dropdown list for WeaponToReplace
// and WeaponBasedOff. Sourced from MBIIWeapons (the canonical weapon
// catalog) unioned with weaponIconAliases — earlier the list was
// built only from weaponIconAliases (a subset), so weapons in
// MBIIWeapons that lacked a custom HUD icon (e.g. WP_BLASTER_PISTOL)
// were silently missing from the override target picker. Filtered to
// live weapons only — no sentinels, no #ifdef-guarded entries — so
// authors don't accidentally override a weapon that doesn't ship.
func weaponPickOptions() []string {
	seen := map[string]bool{}
	for _, w := range MBIIWeapons {
		if w.ID == "" || w.ID == "WP_NONE" || w.Hidden {
			continue
		}
		seen[w.ID] = true
	}
	for id := range weaponIconAliases {
		if id == "" || id == "WP_NONE" {
			continue
		}
		seen[id] = true
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (ui *WeaponInfoUI) createUI() {
	// --- Left pane: override list ---
	ui.weaponList = widget.NewList(
		func() int { return len(ui.editor.character.WeaponOverrides) },
		func() fyne.CanvasObject {
			// Two-line row: bold target WP_* on top, weapon name below.
			// Icon tile on the left keeps the list visually scannable.
			iconTile := canvas.NewImageFromResource(theme.FileImageIcon())
			iconTile.FillMode = canvas.ImageFillContain
			iconTile.ScaleMode = canvas.ImageScaleSmooth
			iconTile.SetMinSize(fyne.NewSize(28, 28))
			target := widget.NewLabelWithStyle("WP_...",
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
			wi := ui.editor.character.WeaponOverrides[id]
			border := obj.(*fyne.Container)
			// Left cell: icon
			leftCell := border.Objects[1].(*fyne.Container)
			iconTile := leftCell.Objects[0].(*canvas.Image)
			iconTile.Resource = ui.resolveRowIcon(wi)
			iconTile.Refresh()
			// Center cell: text stack
			center := border.Objects[0].(*fyne.Container)
			center.Objects[0].(*widget.Label).SetText(wi.WeaponToReplace)
			name := wi.WeaponName
			if name == "" {
				name = "(no custom name)"
			}
			center.Objects[1].(*widget.Label).SetText(name)
		},
	)
	ui.weaponList.OnSelected = func(id widget.ListItemID) {
		ui.currentWeaponIndex = id
		ui.loadWeaponDetails(id)
		ui.showDetailPane()
	}

	addBtn := widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), ui.addWeapon)
	dupBtn := widget.NewButtonWithIcon("Duplicate", theme.ContentCopyIcon(), ui.duplicateWeapon)
	removeBtn := widget.NewButtonWithIcon("Remove", theme.ContentRemoveIcon(), ui.removeWeapon)
	dupBtn.Importance = widget.LowImportance

	actionsRow := container.NewHBox(addBtn, dupBtn, removeBtn)

	listPane := container.NewBorder(
		container.NewPadded(actionsRow),
		nil, nil, nil,
		ui.weaponList,
	)

	// --- Right pane: detail form ---
	wpOptions := weaponPickOptions()

	ui.weaponToReplaceSelect = widget.NewSelect(wpOptions, func(string) { ui.onDetailChanged("") })
	ui.weaponToReplaceSelect.PlaceHolder = "Which WP_ to override"
	ui.weaponBasedOffSelect = widget.NewSelect(wpOptions, func(string) { ui.onDetailChanged("") })
	ui.weaponBasedOffSelect.PlaceHolder = "Base weapon to clone from"

	ui.iconPreview = canvas.NewImageFromResource(theme.FileImageIcon())
	ui.iconPreview.FillMode = canvas.ImageFillContain
	ui.iconPreview.ScaleMode = canvas.ImageScaleSmooth
	ui.iconPreview.SetMinSize(fyne.NewSize(48, 48))

	ui.newWorldModelEntry = NewInputEntry()
	ui.newWorldModelEntry.OnChanged = ui.onDetailChanged
	ui.newViewModelEntry = NewInputEntry()
	ui.newViewModelEntry.OnChanged = ui.onDetailChanged
	ui.iconEntry = NewInputEntry()
	ui.iconEntry.OnChanged = func(s string) {
		ui.refreshIconPreview(s)
		ui.onDetailChanged(s)
	}
	ui.weaponNameEntry = NewInputEntry()
	ui.weaponNameEntry.OnChanged = ui.onDetailChanged

	ui.muzzleEffectEntry = NewInputEntry()
	ui.muzzleEffectEntry.OnChanged = ui.onDetailChanged
	ui.altMuzzleEffectEntry = NewInputEntry()
	ui.altMuzzleEffectEntry.OnChanged = ui.onDetailChanged
	ui.missileEffectEntry = NewInputEntry()
	ui.missileEffectEntry.OnChanged = ui.onDetailChanged
	ui.altMissileEffectEntry = NewInputEntry()
	ui.altMissileEffectEntry.OnChanged = ui.onDetailChanged

	ui.flashSound0Entry = NewInputEntry()
	ui.flashSound0Entry.OnChanged = ui.onDetailChanged
	ui.altFlashSound0Entry = NewInputEntry()
	ui.altFlashSound0Entry.OnChanged = ui.onDetailChanged
	ui.chargeSoundEntry = NewInputEntry()
	ui.chargeSoundEntry.OnChanged = ui.onDetailChanged

	ui.customAmmoEntry = NewInputEntry()
	ui.customAmmoEntry.OnChanged = ui.onDetailChanged
	ui.clipSizeEntry = NewInputEntry()
	ui.clipSizeEntry.OnChanged = ui.onDetailChanged
	ui.reloadTimeModifierEntry = NewInputEntry()
	ui.reloadTimeModifierEntry.OnChanged = ui.onDetailChanged

	// Grouped sections. Each group gets its own accordion item so
	// users can collapse areas they aren't touching — 20-field
	// flat forms were a scroll-hunt every edit.
	identityForm := widget.NewForm(
		widget.NewFormItem("Weapon To Replace", ui.weaponToReplaceSelect),
		widget.NewFormItem("Weapon Based Off", ui.weaponBasedOffSelect),
		widget.NewFormItem("Weapon Name", ui.weaponNameEntry),
		widget.NewFormItem("Icon",
			container.NewBorder(nil, nil,
				container.NewGridWrap(fyne.NewSize(48, 48), ui.iconPreview),
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.iconEntry, AssetTypeIcon) },
					"Browse for Icon"),
				ui.iconEntry,
			),
		),
	)

	visualsForm := widget.NewForm(
		widget.NewFormItem("New World Model",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.newWorldModelEntry, AssetTypeModel) },
					"Browse for World Model"),
				ui.newWorldModelEntry)),
		widget.NewFormItem("New View Model",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.newViewModelEntry, AssetTypeModel) },
					"Browse for View Model"),
				ui.newViewModelEntry)),
		widget.NewFormItem("Muzzle Effect",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.muzzleEffectEntry, AssetTypeEffect) },
					"Browse for Muzzle Effect"),
				ui.muzzleEffectEntry)),
		widget.NewFormItem("Alt Muzzle Effect",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.altMuzzleEffectEntry, AssetTypeEffect) },
					"Browse for Alt Muzzle Effect"),
				ui.altMuzzleEffectEntry)),
		widget.NewFormItem("Missile Effect",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.missileEffectEntry, AssetTypeEffect) },
					"Browse for Missile Effect"),
				ui.missileEffectEntry)),
		widget.NewFormItem("Alt Missile Effect",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.altMissileEffectEntry, AssetTypeEffect) },
					"Browse for Alt Missile Effect"),
				ui.altMissileEffectEntry)),
	)

	soundsForm := widget.NewForm(
		widget.NewFormItem("Flash Sound 0",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.flashSound0Entry, AssetTypeSound) },
					"Browse for Flash Sound"),
				ui.flashSound0Entry)),
		widget.NewFormItem("Alt Flash Sound 0",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.altFlashSound0Entry, AssetTypeSound) },
					"Browse for Alt Flash Sound"),
				ui.altFlashSound0Entry)),
		widget.NewFormItem("Charge Sound",
			container.NewBorder(nil, nil, nil,
				NewTooltipButton("", theme.FolderOpenIcon(),
					func() { ui.browseAsset(ui.chargeSoundEntry, AssetTypeSound) },
					"Browse for Charge Sound"),
				ui.chargeSoundEntry)),
	)

	ammoForm := widget.NewForm(
		widget.NewFormItem("Custom Ammo", ui.customAmmoEntry),
		widget.NewFormItem("Clip Size", ui.clipSizeEntry),
		widget.NewFormItem("Reload Time Modifier", ui.reloadTimeModifierEntry),
	)

	accordion := widget.NewAccordion(
		widget.NewAccordionItem("Identity", identityForm),
		widget.NewAccordionItem("Visuals & Effects", visualsForm),
		widget.NewAccordionItem("Sounds", soundsForm),
		widget.NewAccordionItem("Ammo & Reload", ammoForm),
	)
	accordion.MultiOpen = true
	accordion.Open(0) // Identity expanded by default

	// Empty state — shown when no override is selected. Encourages
	// the user to either add one or pick an existing row. Better
	// than showing an empty form with placeholder text everywhere.
	emptyMsg := widget.NewLabelWithStyle(
		"Select a weapon override from the list,\nor click Add to create one.",
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	ui.emptyState = container.NewCenter(emptyMsg)

	// Stack holds both empty-state + detail form; we toggle Show/Hide
	// on children to swap between them rather than rebuilding.
	detailScroll := container.NewVScroll(accordion)
	detailScroll.Hide()
	ui.detailPane = container.NewStack(ui.emptyState, detailScroll)

	ui.container = container.NewHSplit(listPane, ui.detailPane)
	ui.container.SetOffset(0.3)
}

// resolveRowIcon pulls an icon for the list row. Uses the override's
// own Icon field first (which is an explicit game-asset path the
// author chose), falls back to the WeaponBasedOff's canonical icon,
// then finally to the theme glyph.
func (ui *WeaponInfoUI) resolveRowIcon(wi parsers.WeaponInfo) fyne.Resource {
	if ui.editor.assetBrowser != nil && wi.Icon != "" {
		if res := ui.editor.assetBrowser.LoadIconResource(wi.Icon); res != nil {
			return res
		}
	}
	if ui.editor.iconResolver != nil && wi.WeaponBasedOff != "" {
		path := ui.editor.iconResolver.ResolveWeaponIcon(wi.WeaponBasedOff)
		if ui.editor.assetBrowser != nil {
			if res := ui.editor.assetBrowser.LoadIconResource(path); res != nil {
				return res
			}
		}
	}
	return theme.FileImageIcon()
}

// showDetailPane swaps the right pane from empty-state to the form.
// Called on row selection. When the list goes empty (remove last
// override), hideDetailPane brings the empty state back.
func (ui *WeaponInfoUI) showDetailPane() {
	if ui.detailPane == nil || len(ui.detailPane.Objects) != 2 {
		return
	}
	ui.detailPane.Objects[0].Hide()
	ui.detailPane.Objects[1].Show()
	ui.detailPane.Refresh()
}

func (ui *WeaponInfoUI) hideDetailPane() {
	if ui.detailPane == nil || len(ui.detailPane.Objects) != 2 {
		return
	}
	ui.detailPane.Objects[0].Show()
	ui.detailPane.Objects[1].Hide()
	ui.detailPane.Refresh()
}

// refreshIconPreview updates the preview image next to the Icon
// entry. Swapped out live as the user types, so they see the moment
// the path resolves against the VFS.
func (ui *WeaponInfoUI) refreshIconPreview(path string) {
	if ui.editor.assetBrowser != nil && path != "" {
		if res := ui.editor.assetBrowser.LoadIconResource(path); res != nil {
			ui.iconPreview.Resource = res
			ui.iconPreview.Refresh()
			return
		}
	}
	ui.iconPreview.Resource = theme.FileImageIcon()
	ui.iconPreview.Refresh()
}

func (ui *WeaponInfoUI) GetContent() fyne.CanvasObject { return ui.container }

// UpdateUI refreshes the view from the character. Called after load.
func (ui *WeaponInfoUI) UpdateUI() {
	ui.weaponList.Refresh()
	if ui.currentWeaponIndex != -1 && ui.currentWeaponIndex < len(ui.editor.character.WeaponOverrides) {
		ui.loadWeaponDetails(ui.currentWeaponIndex)
		ui.showDetailPane()
	} else {
		ui.clearDetails()
		ui.hideDetailPane()
	}
}

func (ui *WeaponInfoUI) addWeapon() {
	ui.editor.character.WeaponOverrides = append(ui.editor.character.WeaponOverrides,
		parsers.WeaponInfo{ExtraFields: make(map[string]string)})
	ui.editor.markDirty()
	ui.weaponList.Refresh()
	ui.weaponList.Select(len(ui.editor.character.WeaponOverrides) - 1)
}

// duplicateWeapon clones the currently-selected override. Common
// request for classes that swap multiple variants of the same
// weapon family with near-identical settings.
func (ui *WeaponInfoUI) duplicateWeapon() {
	if ui.currentWeaponIndex < 0 ||
		ui.currentWeaponIndex >= len(ui.editor.character.WeaponOverrides) {
		return
	}
	src := ui.editor.character.WeaponOverrides[ui.currentWeaponIndex]
	// Deep-copy the ExtraFields map so edits on the clone don't
	// stomp the original.
	clone := src
	clone.ExtraFields = map[string]string{}
	for k, v := range src.ExtraFields {
		clone.ExtraFields[k] = v
	}
	ui.editor.character.WeaponOverrides = append(ui.editor.character.WeaponOverrides, clone)
	ui.editor.markDirty()
	ui.weaponList.Refresh()
	ui.weaponList.Select(len(ui.editor.character.WeaponOverrides) - 1)
}

func (ui *WeaponInfoUI) removeWeapon() {
	if ui.currentWeaponIndex == -1 {
		return
	}
	ui.editor.character.WeaponOverrides = append(
		ui.editor.character.WeaponOverrides[:ui.currentWeaponIndex],
		ui.editor.character.WeaponOverrides[ui.currentWeaponIndex+1:]...,
	)
	ui.currentWeaponIndex = -1
	ui.editor.markDirty()
	ui.weaponList.Refresh()
	ui.clearDetails()
	ui.hideDetailPane()
}

func (ui *WeaponInfoUI) loadWeaponDetails(index int) {
	if index < 0 || index >= len(ui.editor.character.WeaponOverrides) {
		ui.clearDetails()
		return
	}

	wi := ui.editor.character.WeaponOverrides[index]
	ui.weaponToReplaceSelect.SetSelected(wi.WeaponToReplace)
	ui.weaponBasedOffSelect.SetSelected(wi.WeaponBasedOff)
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
	if wi.CustomAmmo != 0 {
		ui.customAmmoEntry.SetText(strconv.Itoa(wi.CustomAmmo))
	} else {
		ui.customAmmoEntry.SetText("")
	}
	if wi.ClipSize != 0 {
		ui.clipSizeEntry.SetText(strconv.Itoa(wi.ClipSize))
	} else {
		ui.clipSizeEntry.SetText("")
	}
	if wi.ReloadTimeModifier != 0 {
		ui.reloadTimeModifierEntry.SetText(fmt.Sprintf("%g", wi.ReloadTimeModifier))
	} else {
		ui.reloadTimeModifierEntry.SetText("")
	}
	ui.refreshIconPreview(wi.Icon)
}

func (ui *WeaponInfoUI) clearDetails() {
	ui.weaponToReplaceSelect.ClearSelected()
	ui.weaponBasedOffSelect.ClearSelected()
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
	ui.iconPreview.Resource = theme.FileImageIcon()
	ui.iconPreview.Refresh()
}

// onDetailChanged fires on any form widget mutation and writes the
// current values back to the selected WeaponInfo entry. The string
// arg is ignored — we re-read every field — because the form uses
// multiple widget types (Entry + Select) with different callback
// signatures. Rereading is cheap and keeps a single code path.
func (ui *WeaponInfoUI) onDetailChanged(s string) {
	_ = s
	if ui.currentWeaponIndex == -1 ||
		ui.currentWeaponIndex >= len(ui.editor.character.WeaponOverrides) {
		return
	}

	wi := &ui.editor.character.WeaponOverrides[ui.currentWeaponIndex]
	wi.WeaponToReplace = ui.weaponToReplaceSelect.Selected
	wi.WeaponBasedOff = ui.weaponBasedOffSelect.Selected
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

	ui.editor.markDirty()
	ui.weaponList.RefreshItem(ui.currentWeaponIndex)
}
