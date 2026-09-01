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
				Name:    modelName,
				Faction: categorizeModelFaction(modelName),
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

// ShowModelGalleryModal displays an interactive visual picker dialog for player models and skins.
func ShowModelGalleryModal(parent fyne.Window, vfs *VirtualFileSystem, ab *AssetBrowser, onSelected func(model, skin string)) {
	if vfs == nil || ab == nil {
		return
	}

	modelsMap := gatherModelsMap(vfs)

	var modelsList []*ModelInfo
	for _, m := range modelsMap {
		modelsList = append(modelsList, m)
	}
	sort.Slice(modelsList, func(i, j int) bool {
		return modelsList[i].Name < modelsList[j].Name
	})

	var selectedModel *ModelInfo
	var selectedSkin string = "default"

	var dialogModal *widget.PopUp
	previewBox := container.NewMax()
	skinSelect := widget.NewSelect([]string{"default"}, func(s string) {
		selectedSkin = s
		updateGalleryPreview(selectedModel, selectedSkin, previewBox, ab)
	})

	gridContainer := container.NewGridWrap(fyne.NewSize(110, 130))

	renderCards := func(filterText, factionFilter string) {
		gridContainer.Objects = nil
		fText := strings.ToLower(strings.TrimSpace(filterText))

		for _, m := range modelsList {
			if fText != "" && !strings.Contains(strings.ToLower(m.Name), fText) {
				continue
			}
			if factionFilter != "All" && m.Faction != factionFilter {
				continue
			}

			modelRef := m
			var iconObj fyne.CanvasObject
			portraitPath := modelRef.PortraitKey
			if portraitPath == "" {
				portraitPath = "models/players/" + modelRef.Name + "/mb2_icon_default"
			}

			res := ab.LoadIconResource(portraitPath)
			if res != nil {
				iconObj = NewRasterIconFromResource(res, 64, 64)
			} else {
				iconObj = container.NewGridWrap(fyne.NewSize(64, 64), widget.NewIcon(theme.AccountIcon()))
			}

			nameLbl := widget.NewLabelWithStyle(modelRef.Name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
			nameLbl.Truncation = fyne.TextTruncateEllipsis

			cardContent := container.NewVBox(
				container.NewCenter(iconObj),
				nameLbl,
			)

			clickable := newClickableCell(cardContent, func() {
				selectedModel = modelRef
				skinSelect.Options = modelRef.Skins
				if len(modelRef.Skins) > 0 {
					selectedSkin = modelRef.Skins[0]
					skinSelect.SetSelected(selectedSkin)
				}
				updateGalleryPreview(selectedModel, selectedSkin, previewBox, ab)
			})

			gridContainer.Add(clickable)
		}
		gridContainer.Refresh()
	}

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search models...")

	factionSelect := widget.NewSelect([]string{"All", "Republic", "Imperial", "Rebel", "CIS", "Jedi/Sith", "Mandalorian", "Mercenary"}, func(s string) {
		renderCards(searchEntry.Text, s)
	})
	factionSelect.SetSelected("All")

	searchEntry.OnChanged = func(s string) {
		renderCards(s, factionSelect.Selected)
	}

	renderCards("", "All")

	scrollGrid := container.NewScroll(gridContainer)
	scrollGrid.SetMinSize(fyne.NewSize(520, 380))

	// Sidebar detail panel
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

	split := container.NewHSplit(scrollGrid, container.NewPadded(rightPanel))
	split.SetOffset(0.68)

	modalContent := container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("Player Model & Skin Gallery", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(widget.NewIcon(theme.SearchIcon()), searchEntry, widget.NewLabel("Faction:"), factionSelect),
		),
		nil, nil, nil,
		split,
	)

	dialogModal = widget.NewModalPopUp(container.NewPadded(modalContent), parent.Canvas())
	dialogModal.Resize(fyne.NewSize(820, 520))
	dialogModal.Show()
}

func updateGalleryPreview(m *ModelInfo, skin string, containerObj *fyne.Container, ab *AssetBrowser) {
	containerObj.Objects = nil
	if m == nil {
		containerObj.Add(widget.NewLabelWithStyle("No model selected", fyne.TextAlignCenter, fyne.TextStyle{Italic: true}))
		containerObj.Refresh()
		return
	}

	// Try candidate skin paths
	candidates := []string{
		"models/players/" + m.Name + "/mb2_icon_" + skin,
		"models/players/" + m.Name + "/icon_" + skin,
		"models/players/" + m.Name + "/" + skin,
		m.PortraitKey,
	}

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
