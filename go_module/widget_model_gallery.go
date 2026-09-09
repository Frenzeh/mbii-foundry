package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ModelInfo represents a player model discovered in the VFS
type ModelInfo struct {
	Name        string
	SearchName  string // Pre-lowercased for zero-allocation filtering
	Skins       []string
	PortraitKey string
	Faction     string
}

// gatherModelsMap scans the VFS for player models, skin variants, and portraits.
func gatherModelsMap(vfs *VirtualFileSystem) map[string]*ModelInfo {
	modelsMap := make(map[string]*ModelInfo)
	if vfs == nil {
		return modelsMap
	}

	vfs.mu.RLock()
	for key := range vfs.Index {
		// Look for models/players/<model>/...
		if !strings.HasPrefix(key, "models/players/") {
			continue
		}
		rel := strings.TrimPrefix(key, "models/players/")
		parts := strings.Split(rel, "/")
		if len(parts) < 2 {
			continue
		}
		modelName := parts[0]
		if modelName == "" || strings.HasPrefix(modelName, ".") {
			continue
		}

		info, ok := modelsMap[modelName]
		if !ok {
			info = &ModelInfo{
				Name:       modelName,
				SearchName: strings.ToLower(modelName),
				Faction:    categorizeModelFaction(modelName),
			}
			modelsMap[modelName] = info
		}

		fileName := parts[len(parts)-1]
		// Check for .skin files
		if strings.HasSuffix(fileName, ".skin") && !strings.HasSuffix(fileName, "_low.skin") {
			skinName := strings.TrimSuffix(fileName, ".skin")
			skinName = strings.TrimPrefix(skinName, "model_")
			// Skip body part sub-mesh files from stock JKA (torso_*, lower_*, etc.)
			if strings.HasPrefix(skinName, "lower_") || strings.HasPrefix(skinName, "torso_") ||
				strings.HasPrefix(skinName, "hips_") || strings.HasPrefix(skinName, "legs_") ||
				strings.HasPrefix(skinName, "arms_") || strings.HasPrefix(skinName, "hands_") ||
				strings.HasPrefix(skinName, "cap_") || strings.HasPrefix(skinName, "head_cap_") {
				continue
			}
			if skinName != "" && !containsStr(info.Skins, skinName) {
				info.Skins = append(info.Skins, skinName)
			}
		}

		// Check for default and skin portraits
		if strings.HasPrefix(fileName, "mb2_icon_") || strings.HasPrefix(fileName, "icon_") {
			ext := filepath.Ext(fileName)
			if ext == ".jpg" || ext == ".png" || ext == ".tga" {
				if info.PortraitKey == "" {
					info.PortraitKey = key
				}
				// Also derive skin variant from the icon filename
				baseIcon := strings.TrimSuffix(fileName, ext)
				skinDerived := strings.TrimPrefix(baseIcon, "mb2_icon_")
				skinDerived = strings.TrimPrefix(skinDerived, "icon_")
				// Skip body part icons (torso_*, lower_*, etc.)
				if strings.HasPrefix(skinDerived, "lower_") || strings.HasPrefix(skinDerived, "torso_") ||
					strings.HasPrefix(skinDerived, "hips_") || strings.HasPrefix(skinDerived, "legs_") ||
					strings.HasPrefix(skinDerived, "arms_") || strings.HasPrefix(skinDerived, "hands_") ||
					strings.HasPrefix(skinDerived, "cap_") {
					continue
				}
				if skinDerived != "" && !containsStr(info.Skins, skinDerived) {
					info.Skins = append(info.Skins, skinDerived)
				}
			}
		}
	}
	vfs.mu.RUnlock()

	for _, m := range modelsMap {
		if len(m.Skins) == 0 {
			m.Skins = []string{"default"}
		}
		sort.Strings(m.Skins)
	}
	return modelsMap
}

// ShowSkinPickerModal displays a focused modal showing all available skin portraits for the current model.
func ShowSkinPickerModal(parent fyne.Window, modelName, currentSkin string, vfs *VirtualFileSystem, ab *AssetBrowser, ir *IconResolver, onSelected func(model, skin string)) {
	if parent == nil || vfs == nil || ab == nil {
		return
	}

	modelLower := strings.ToLower(strings.TrimSpace(modelName))
	if modelLower == "" {
		ShowModelGalleryModal(parent, vfs, ab, onSelected)
		return
	}

	modelsMap := gatherModelsMap(vfs)
	modelInfo, ok := modelsMap[modelLower]
	if !ok || len(modelInfo.Skins) == 0 {
		ShowModelGalleryModal(parent, vfs, ab, onSelected)
		return
	}

	var dialogModal *widget.PopUp

	grid := container.NewGridWrap(fyne.NewSize(120, 136))
	for _, skin := range modelInfo.Skins {
		skinName := skin
		var res fyne.Resource

		// Resolve candidate portrait strictly for this skin
		candidates := []string{
			"models/players/" + modelInfo.Name + "/mb2_icon_" + skinName,
			"models/players/" + modelInfo.Name + "/icon_" + skinName,
			"models/players/" + modelInfo.Name + "/" + skinName,
		}
		for _, c := range candidates {
			if r := ab.LoadIconResource(c); r != nil {
				res = r
				break
			}
		}

		var iconObj fyne.CanvasObject
		if res != nil {
			iconObj = NewRasterIconFromResource(res, 64, 64)
		} else {
			iconObj = container.NewGridWrap(fyne.NewSize(64, 64), widget.NewIcon(theme.AccountIcon()))
		}

		lbl := widget.NewLabelWithStyle(skinName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		lbl.Truncation = fyne.TextTruncateEllipsis

		cardContent := container.NewVBox(
			container.NewCenter(iconObj),
			lbl,
		)

		isSelected := strings.EqualFold(skinName, currentSkin)
		accent := CurrentThemeColor
		fillAlpha := uint8(8)
		strokeAlpha := uint8(30)
		if isSelected {
			accent = color.NRGBA{R: 56, G: 189, B: 248, A: 255} // Active Sky Cyan
			fillAlpha = 28
			strokeAlpha = 110
		}

		cardTile := NewTilePanel(cardContent, TileOpts{
			AccentColor: accent,
			FillAlpha:   fillAlpha,
			StrokeAlpha: strokeAlpha,
			Padded:      true,
		})

		clickable := newClickableCell(cardTile, func() {
			if onSelected != nil {
				onSelected(modelInfo.Name, skinName)
			}
			if dialogModal != nil {
				dialogModal.Hide()
			}
		})

		grid.Add(clickable)
	}

	browseAllBtn := widget.NewButtonWithIcon("Browse All Models", theme.SearchIcon(), func() {
		if dialogModal != nil {
			dialogModal.Hide()
		}
		ShowModelGalleryModal(parent, vfs, ab, onSelected)
	})

	closeBtn := widget.NewButton("Cancel", func() {
		if dialogModal != nil {
			dialogModal.Hide()
		}
	})

	header := container.NewBorder(
		nil, nil,
		widget.NewLabelWithStyle(fmt.Sprintf("Skins for %s (%d available)", modelInfo.Name, len(modelInfo.Skins)), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		browseAllBtn,
	)

	scroll := container.NewVScroll(grid)
	scroll.SetMinSize(fyne.NewSize(540, 320))

	modalContent := container.NewBorder(
		container.NewPadded(header),
		container.NewHBox(layout.NewSpacer(), closeBtn),
		nil, nil,
		container.NewPadded(scroll),
	)

	dialogModal = widget.NewModalPopUp(container.NewPadded(modalContent), parent.Canvas())
	dialogModal.Resize(fyne.NewSize(620, 440))
	dialogModal.Show()
}

type modelGalleryCard struct {
	widget.BaseWidget
	portrait *fyne.Container
	name     *widget.Label
}

func newModelGalleryCard() *modelGalleryCard {
	card := &modelGalleryCard{
		portrait: container.NewStack(),
		name:     widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	}
	card.name.Truncation = fyne.TextTruncateEllipsis
	card.ExtendBaseWidget(card)
	return card
}

func (card *modelGalleryCard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewVBox(
		container.NewCenter(card.portrait),
		card.name,
	))
}

func (card *modelGalleryCard) MinSize() fyne.Size {
	return fyne.NewSize(110, 130)
}

func (card *modelGalleryCard) update(model *ModelInfo, ab *AssetBrowser) {
	portraitPath := model.PortraitKey
	if portraitPath == "" {
		portraitPath = "models/players/" + model.Name + "/mb2_icon_default"
	}
	var portrait fyne.CanvasObject
	if res := ab.LoadIconResource(portraitPath); res != nil {
		portrait = NewRasterIconFromResource(res, 64, 64)
	} else {
		portrait = container.NewGridWrap(fyne.NewSize(64, 64), widget.NewIcon(theme.AccountIcon()))
	}
	card.portrait.Objects = []fyne.CanvasObject{portrait}
	card.portrait.Refresh()
	card.name.SetText(model.Name)
}

// ShowModelGalleryModal displays an interactive visual picker dialog for player models and skins.
func ShowModelGalleryModal(parent fyne.Window, vfs *VirtualFileSystem, ab *AssetBrowser, onSelected func(model, skin string)) {
	if parent == nil || vfs == nil || ab == nil {
		return
	}

	modelsMap := gatherModelsMap(vfs)
	modelsList := make([]*ModelInfo, 0, len(modelsMap))
	for _, model := range modelsMap {
		modelsList = append(modelsList, model)
	}
	sort.Slice(modelsList, func(i, j int) bool {
		return modelsList[i].Name < modelsList[j].Name
	})

	var selectedModel *ModelInfo
	selectedSkin := "default"
	var dialogModal *widget.PopUp

	previewBox := container.NewMax()
	skinSelect := widget.NewSelect([]string{"default"}, func(s string) {
		selectedSkin = s
		updateGalleryPreview(selectedModel, selectedSkin, previewBox, ab)
	})

	// widget.GridWrap virtualizes cells and reuses their widgets. The previous
	// container.NewGridWrap rebuilt and text-shaped every model card on each
	// keystroke, which dominated the measured gallery open/filter cost.
	filteredModels := append([]*ModelInfo(nil), modelsList...)
	modelGrid := widget.NewGridWrap(
		func() int { return len(filteredModels) },
		func() fyne.CanvasObject { return newModelGalleryCard() },
		func(id widget.GridWrapItemID, item fyne.CanvasObject) {
			if id < 0 || id >= len(filteredModels) {
				return
			}
			item.(*modelGalleryCard).update(filteredModels[id], ab)
		},
	)
	modelGridBox := container.NewGridWrap(fyne.NewSize(520, 380), modelGrid)
	modelGrid.OnSelected = func(id widget.GridWrapItemID) {
		if id < 0 || id >= len(filteredModels) {
			return
		}
		selectedModel = filteredModels[id]
		skinSelect.Options = selectedModel.Skins
		if len(selectedModel.Skins) > 0 {
			selectedSkin = selectedModel.Skins[0]
			skinSelect.SetSelected(selectedSkin)
		}
		updateGalleryPreview(selectedModel, selectedSkin, previewBox, ab)
	}

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search models...")

	factionSelect := widget.NewSelect([]string{"All", "Republic", "Imperial", "Rebel", "CIS", "Jedi/Sith", "Mandalorian", "Mercenary"}, nil)
	renderCards := func(filterText, factionFilter string) {
		modelGrid.UnselectAll()
		filteredModels = filteredModels[:0]
		filterText = strings.ToLower(strings.TrimSpace(filterText))
		for _, model := range modelsList {
			if filterText != "" && !strings.Contains(model.SearchName, filterText) {
				continue
			}
			if factionFilter != "All" && model.Faction != factionFilter {
				continue
			}
			filteredModels = append(filteredModels, model)
		}
		modelGrid.Refresh()
	}
	factionSelect.OnChanged = func(faction string) {
		renderCards(searchEntry.Text, faction)
	}
	factionSelect.SetSelected("All")
	searchEntry.OnChanged = func(text string) {
		renderCards(text, factionSelect.Selected)
	}

	applyBtn := widget.NewButtonWithIcon("Select Model & Skin", theme.ConfirmIcon(), func() {
		if selectedModel != nil && onSelected != nil {
			onSelected(selectedModel.Name, selectedSkin)
		}
		if dialogModal != nil {
			dialogModal.Hide()
		}
	})
	applyBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButton("Cancel", func() {
		if dialogModal != nil {
			dialogModal.Hide()
		}
	})

	rightPanel := container.NewVBox(
		widget.NewLabelWithStyle("Selected Character", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		previewBox,
		widget.NewLabel("Skin Variant:"),
		skinSelect,
		container.NewHBox(applyBtn, closeBtn),
	)
	split := container.NewHSplit(modelGridBox, container.NewPadded(rightPanel))
	split.SetOffset(0.68)
	modalContent := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Player Model & Skin Gallery", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewBorder(nil, nil, widget.NewIcon(theme.SearchIcon()), container.NewHBox(widget.NewLabel("Faction:"), factionSelect), searchEntry),
		),
		nil, nil, nil,
		split,
	)
	dialogModal = widget.NewModalPopUp(container.NewPadded(modalContent), parent.Canvas())
	dialogModal.Resize(fyne.NewSize(820, 520))
	dialogModal.Show()
	parent.Canvas().Focus(searchEntry)
}

func updateGalleryPreview(m *ModelInfo, skin string, containerObj *fyne.Container, ab *AssetBrowser) {
	containerObj.Objects = nil
	if m == nil {
		containerObj.Add(widget.NewLabelWithStyle("No model selected", fyne.TextAlignCenter, fyne.TextStyle{Italic: true}))
		containerObj.Refresh()
		return
	}

	// Use the same documented default fallback as the character editor,
	// never an arbitrary portrait for another skin from map iteration.
	candidates := NewIconResolver(ab.vfs).ResolveClassIconCandidates(m.Name, skin, "")
	var res fyne.Resource
	for _, c := range candidates {
		if c != "" {
			if r := ab.LoadIconResource(c); r != nil {
				res = r
				break
			}
		}
	}

	var imgObj fyne.CanvasObject
	if res != nil {
		imgObj = NewRasterIconFromResource(res, 96, 96)
	} else {
		imgObj = container.NewGridWrap(fyne.NewSize(96, 96), widget.NewIcon(theme.AccountIcon()))
	}

	title := widget.NewLabelWithStyle(m.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	sub := widget.NewLabelWithStyle("Skin: "+skin, fyne.TextAlignCenter, fyne.TextStyle{Monospace: true})

	containerObj.Add(container.NewVBox(
		container.NewCenter(imgObj),
		title,
		sub,
	))
	containerObj.Refresh()
}

func categorizeModelFaction(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "clone") || strings.Contains(m, "clonetrooper") || strings.Contains(m, "arc") || strings.Contains(m, "rc") || strings.Contains(m, "cody") || strings.Contains(m, "rex"):
		return "Republic"
	case strings.Contains(m, "storm") || strings.Contains(m, "scout") || strings.Contains(m, "shadow") || strings.Contains(m, "officer") || strings.Contains(m, "trooper") || strings.Contains(m, "vader"):
		return "Imperial"
	case strings.Contains(m, "rebel") || strings.Contains(m, "hero") || strings.Contains(m, "luke") || strings.Contains(m, "leia") || strings.Contains(m, "han") || strings.Contains(m, "chewie"):
		return "Rebel"
	case strings.Contains(m, "sbd") || strings.Contains(m, "super_battle_droid") || strings.Contains(m, "battledroid") || strings.Contains(m, "droideka") || strings.Contains(m, "grievous") || strings.Contains(m, "dooku"):
		return "CIS"
	case strings.Contains(m, "jedi") || strings.Contains(m, "sith") || strings.Contains(m, "revan") || strings.Contains(m, "malak") || strings.Contains(m, "nihilus") || strings.Contains(m, "kyle"):
		return "Jedi/Sith"
	case strings.Contains(m, "mando") || strings.Contains(m, "boba") || strings.Contains(m, "jango") || strings.Contains(m, "deathwatch"):
		return "Mandalorian"
	default:
		return "Mercenary"
	}
}

func containsStr(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
