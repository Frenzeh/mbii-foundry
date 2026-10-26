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

// CharacterSummaryWidget provides a compact, persistent identity header for
// character documents. It anchors the current name, class, portrait, and core
// resources without taking over the editor viewport.
type CharacterSummaryWidget struct {
	widget.BaseWidget
	character *parsers.MBCHCharacter
	vfs       *VirtualFileSystem
	ab        *AssetBrowser
	ir        *IconResolver

	container *fyne.Container
	nameLbl   *canvas.Text
	classLbl  *canvas.Text
	statsLbl  *canvas.Text
	preview   *canvas.Image
}

func NewCharacterSummaryWidget(ch *parsers.MBCHCharacter, vfs *VirtualFileSystem, ab *AssetBrowser, ir *IconResolver) *CharacterSummaryWidget {
	w := &CharacterSummaryWidget{
		character: ch,
		vfs:       vfs,
		ab:        ab,
		ir:        ir,
	}
	w.createUI()
	w.Refresh()
	w.ExtendBaseWidget(w)
	return w
}

func (w *CharacterSummaryWidget) createUI() {
	eyebrow := canvas.NewText("CHARACTER", theme.PlaceHolderColor())
	eyebrow.TextSize = SizeSmall
	eyebrow.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	w.nameLbl = canvas.NewText("", theme.ForegroundColor())
	w.nameLbl.TextSize = SizeSubtitle
	w.nameLbl.TextStyle = fyne.TextStyle{Bold: true}
	w.classLbl = canvas.NewText("", theme.PlaceHolderColor())
	w.classLbl.TextSize = SizeSmall
	w.classLbl.TextStyle = fyne.TextStyle{Bold: true}
	w.statsLbl = canvas.NewText("", theme.PlaceHolderColor())
	w.statsLbl.TextSize = SizeSmall
	w.statsLbl.TextStyle = fyne.TextStyle{Monospace: true}

	// GridWrap owns the portrait geometry. Setting a conflicting MinSize on
	// canvas.Image triggers Fyne's image renderer "param mismatch" path.
	w.preview = canvas.NewImageFromResource(theme.AccountIcon())
	w.preview.FillMode = canvas.ImageFillContain
	w.preview.ScaleMode = canvas.ImageScaleSmooth

	identity := container.NewVBox(eyebrow, w.nameLbl, w.classLbl, w.statsLbl)
	row := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(56, 56), w.preview),
		nil,
		identity,
	)

	w.container = NewTilePanel(row, TileOpts{
		FillAlpha:   10,
		StrokeAlpha: 42,
		Padded:      true,
	})
}

func (w *CharacterSummaryWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.container)
}

func (w *CharacterSummaryWidget) Refresh() {
	if w.character == nil {
		return
	}

	name := w.character.Name
	if name == "" {
		name = "Unnamed Character"
	}
	w.nameLbl.Text = stripQ3Colors(name)

	classInfo := w.character.MBClass
	if classInfo == "" {
		classInfo = "MB_CLASS_UNKNOWN"
	}
	w.classLbl.Text = formatClassDisplayName(classInfo)

	// Keep the summary scannable: the labels are compact and the values
	// line up in monospace without punctuation-heavy pipes.
	hp := fmt.Sprintf("%d", w.character.MaxHealth)
	if w.character.MaxHealth == 0 {
		hp = "100"
	}
	stats := []string{
		"HP " + hp,
		fmt.Sprintf("ARMOR %d", w.character.MaxArmor),
	}
	if w.character.ForcePool > 0 {
		stats = append(stats, fmt.Sprintf("FP %d", w.character.ForcePool))
	}
	if w.character.Saber1 != "" {
		if w.character.APMultiplier > 0 {
			stats = append(stats, fmt.Sprintf("AP %.2f", w.character.APMultiplier))
		}
		if w.character.BPMultiplier > 0 {
			stats = append(stats, fmt.Sprintf("BP %.2f", w.character.BPMultiplier))
		}
	}
	w.statsLbl.Text = strings.Join(stats, "   ")

	// Update preview image
	if w.ir != nil && w.ab != nil {
		candidates := w.ir.ResolveClassIconCandidates(w.character.Model, w.character.Skin, w.character.UIShader)
		resolved := false
		for _, candidate := range candidates {
			if res := w.ab.LoadIconResource(candidate); res != nil {
				setRasterPreview(w.preview, res)
				resolved = true
				break
			}
		}
		if !resolved {
			setRasterPreview(w.preview, nil)
		}
	} else {
		setRasterPreview(w.preview, nil)
	}

	w.nameLbl.Refresh()
	w.classLbl.Refresh()
	w.statsLbl.Refresh()
}

func formatClassDisplayName(classEnum string) string {
	clean := strings.TrimPrefix(classEnum, "MB_CLASS_")
	clean = strings.ReplaceAll(clean, "_", " ")
	return strings.ToUpper(clean)
}
