package main

import (
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// prettyAttributeName turns an MB_ATT_* enum into a Title Case display
// name for grids that don't have curated display names. Honors a few
// well-known acronyms (UGL/MGL/SS/FP/etc.) that should stay all-caps
// rather than being mangled to "Ugl"/"Mgl"/etc.
func prettyAttributeName(id string) string {
	s := strings.TrimPrefix(id, "MB_ATT_")
	parts := strings.Split(s, "_")
	keepCaps := map[string]bool{
		"UGL": true, "MGL": true, "SS": true, "FP": true,
		"AP": true, "BP": true, "CS": true, "AS": true,
		"ROF": true, "STM": true, "KB": true, "DMG": true,
		"TD": true, "SBD": true, "MT": true, "ARC": true,
		"MD": true, "ET": true, "DC": true, "EE3": true,
		"EE4": true, "DLT19": true, "DLT20A": true, "T21": true,
		"PLX1": true, "DEMP2": true, "CR2": true, "A280": true,
		"E_22": true, "ID": true, "AOE": true, "HP": true,
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		up := strings.ToUpper(p)
		if keepCaps[up] {
			out = append(out, up)
			continue
		}
		// Standard Title Case for the rest.
		if len(p) > 0 {
			out = append(out, strings.ToUpper(p[:1])+strings.ToLower(p[1:]))
		}
	}
	return strings.Join(out, " ")
}

// AttributeToggleWidget is a custom widget for selecting attribute levels
type AttributeToggleWidget struct {
	widget.BaseWidget

	ID          string
	Name        string
	Category    string
	MaxLevel    int
	CurrentVal  int
	Description string

	OnChange func(int)
	OnInfo   func(string, string) // Key, Context — called on row hover
	// OnInfoLeave fires when the mouse leaves the row's hoverable
	// controls (info button, level buttons). Paired with OnInfo so
	// the info-panel's sticky-context behavior can revert to the
	// last-interacted entry rather than freezing on the attribute
	// you happen to have passed over last.
	OnInfoLeave func()

	// OnInfoClick is called on (i) button click. Wired to the App's
	// showStickyContext path (NOT showHoverContext) so a click pins
	// the sidebar regardless of the hover-toggle state. Without this
	// the (i) button silently no-ops whenever the hover toggle is
	// OFF — which is the default — and users have no way to pull up
	// docs for an attribute.
	OnInfoClick func(string, string)

	// UI Components
	titleText *canvas.Text
	idText    *canvas.Text
	buttons   []*HoverButton
	infoBtn   *TooltipButton
	tile      *TilePanelWidget
	catColor  color.Color
	container fyne.CanvasObject
}

func NewAttributeToggleWidget(attr AttributeDef, currentVal int, onChange func(int), onInfo func(string, string), icon fyne.Resource) *AttributeToggleWidget {
	w := &AttributeToggleWidget{
		ID:          attr.ID,
		Name:        attr.Name,
		Category:    attr.Category,
		MaxLevel:    attr.MaxLevel,
		CurrentVal:  currentVal,
		Description: attr.Description,
		OnChange:    onChange,
	}

	w.ExtendBaseWidget(w)
	w.createUI(onInfo, icon)
	return w
}

// SetOnInfoLeave wires the MouseOut callback post-construction so
// callers that didn't have the clear-hover func at build time can
// still opt in. Keeps the widget's primary constructor small.
func (w *AttributeToggleWidget) SetOnInfoLeave(f func()) {
	w.OnInfoLeave = f
	// Re-apply to already-built level buttons — refreshButtons
	// doesn't rewire, so we patch each button's onHoverOut directly.
	for _, btn := range w.buttons {
		if btn != nil {
			btn.onHoverOut = f
		}
	}
}

func (w *AttributeToggleWidget) createUI(onInfo func(string, string), iconRes fyne.Resource) {
	if w.MaxLevel <= 0 {
		w.MaxLevel = 3
	}

	// Primary label: display name (with auto-derived fallback when the
	// data doesn't carry one). Secondary: monospace enum ID caption
	// underneath so authors who think in source can still recognize
	// the row.
	displayName := w.Name
	if displayName == "" || displayName == w.ID || strings.HasPrefix(displayName, "MB_ATT_") {
		displayName = prettyAttributeName(w.ID)
	}

	w.titleText = canvas.NewText(displayName, color.NRGBA{R: 242, G: 245, B: 250, A: 255})
	w.titleText.TextSize = 10.5
	w.titleText.TextStyle = fyne.TextStyle{Bold: true}

	w.idText = canvas.NewText(w.ID, color.NRGBA{R: 130, G: 142, B: 158, A: 215})
	w.idText.TextSize = 8.0
	w.idText.TextStyle = fyne.TextStyle{Monospace: true}

	infoClick := func() {
		if w.OnInfoClick != nil {
			w.OnInfoClick(w.ID, "")
			return
		}
		if onInfo != nil {
			onInfo(w.ID, "")
		}
	}

	var iconObj fyne.CanvasObject
	if iconRes != nil {
		raster := NewRasterIconFromResource(iconRes, 28, 28)
		clickable := newClickableCell(raster, infoClick)
		if onInfo != nil {
			clickable.onHover = func() { onInfo(w.ID, "") }
		}
		iconObj = clickable
		w.infoBtn = nil
	} else {
		w.infoBtn = NewTooltipButton("", theme.InfoIcon(), infoClick,
			"View documentation for this attribute")
		w.infoBtn.Importance = widget.LowImportance
		iconObj = container.NewGridWrap(fyne.NewSize(24, 24), w.infoBtn)
	}

	// Create toggle buttons
	w.buttons = make([]*HoverButton, w.MaxLevel+1)

	// Level 0 (Off)
	w.buttons[0] = w.createLevelButton(0, "Off", onInfo)

	// Levels 1..Max
	for i := 1; i <= w.MaxLevel; i++ {
		w.buttons[i] = w.createLevelButton(i, strconv.Itoa(i), onInfo)
	}

	btnBox := container.NewHBox()
	for _, btn := range w.buttons {
		btnBox.Add(btn)
	}

	w.catColor = attributeColor(w.ID, w.Category)

	labelBlock := container.NewVBox(w.titleText, w.idText)

	row := container.NewBorder(nil, nil,
		container.NewCenter(iconObj),
		container.NewCenter(btnBox),
		container.NewCenter(labelBlock),
	)

	fillAlpha := uint8(5)
	strokeAlpha := uint8(24)
	if w.CurrentVal > 0 {
		fillAlpha = 26
		strokeAlpha = 95
	}

	w.tile = NewDynamicTilePanel(row, TileOpts{
		AccentColor: w.catColor,
		FillAlpha:   fillAlpha,
		StrokeAlpha: strokeAlpha,
		Padded:      false,
	})

	hover := NewHoverContainer(w.tile, func() {
		if onInfo != nil {
			onInfo(w.ID, "")
		}
	})
	if w.OnInfoLeave != nil {
		hover.SetOnLeave(w.OnInfoLeave)
	}
	w.container = hover

	w.refreshButtons()
}

func (w *AttributeToggleWidget) createLevelButton(level int, text string, onInfo func(string, string)) *HoverButton {
	hover := func() {
		if onInfo != nil {
			context := ""
			if level > 0 {
				context = "Level " + strconv.Itoa(level)
			}
			onInfo(w.ID, context)
		}
	}

	btn := NewHoverButton(text, func() {
		target := level
		if level == 0 && w.CurrentVal == 0 {
			target = 1
		}
		w.CurrentVal = target
		w.refreshButtons()
		if w.OnChange != nil {
			w.OnChange(target)
		}
	}, hover, func() {
		if w.OnInfoLeave != nil {
			w.OnInfoLeave()
		}
	})
	return btn
}

func (w *AttributeToggleWidget) refreshButtons() {
	for i, btn := range w.buttons {
		if i == w.CurrentVal {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Show()
		btn.Refresh()
	}

	if w.tile != nil {
		if w.CurrentVal > 0 {
			w.tile.SetAccent(w.catColor, 26, 95)
		} else {
			w.tile.SetAccent(w.catColor, 5, 24)
		}
	}
}

// attributeColor assigns a harmonious, low-cognitive-load accent color per archetype
func attributeColor(id, category string) color.Color {
	idUpper := strings.ToUpper(id)
	switch {
	// Weapons - Pistols & Blasters
	case strings.Contains(idUpper, "PISTOL"), strings.Contains(idUpper, "BLASTER"),
		idUpper == "MB_ATT_A280", idUpper == "MB_ATT_DLT20A", idUpper == "MB_ATT_DLT19",
		idUpper == "MB_ATT_EE3", idUpper == "MB_ATT_EE4", idUpper == "MB_ATT_T21",
		idUpper == "MB_ATT_QUICKDRAW", idUpper == "MB_ATT_PROJECTILE_RIFLE":
		return color.NRGBA{R: 56, G: 189, B: 248, A: 255} // Sky Cyan

	// Weapons - Heavy & Special
	case strings.Contains(idUpper, "CLONERIFLE"), strings.Contains(idUpper, "WESTARM5"),
		strings.Contains(idUpper, "AMBAN"), strings.Contains(idUpper, "DISRUPTOR"),
		strings.Contains(idUpper, "BOWCASTER"), strings.Contains(idUpper, "REPEATER"),
		strings.Contains(idUpper, "FLECHETTE"), strings.Contains(idUpper, "DEMP2"),
		strings.Contains(idUpper, "MINIGUN"), strings.Contains(idUpper, "SHOTGUN"),
		strings.Contains(idUpper, "CONCUSSION"), strings.Contains(idUpper, "THROWER"),
		strings.Contains(idUpper, "FLAMETHROWER"):
		return color.NRGBA{R: 52, G: 211, B: 153, A: 255} // Emerald Green

	// Weapons - Launchers & Ordnance
	case strings.Contains(idUpper, "UGL"), strings.Contains(idUpper, "MGL"),
		strings.Contains(idUpper, "ROCKET"), strings.Contains(idUpper, "PLX1"),
		strings.Contains(idUpper, "BLOB"), strings.Contains(idUpper, "NADES"):
		return color.NRGBA{R: 251, G: 146, B: 60, A: 255} // Warm Orange

	// Weapons - Grenades & Mines
	case strings.Contains(idUpper, "FRAG"), strings.Contains(idUpper, "THERMAL"),
		strings.Contains(idUpper, "GRENADE"), strings.Contains(idUpper, "DET_PACK"),
		strings.Contains(idUpper, "TRIP_MINE"), strings.Contains(idUpper, "SONIC"),
		strings.Contains(idUpper, "CRYOBAN"), strings.Contains(idUpper, "STICKY"):
		return color.NRGBA{R: 248, G: 113, B: 113, A: 255} // Coral Red

	// Lightsaber
	case strings.Contains(idUpper, "SABER"), strings.Contains(idUpper, "STYLE"),
		strings.Contains(idUpper, "BP_"), strings.Contains(idUpper, "AP_"),
		strings.Contains(idUpper, "DEFLECT"):
		return color.NRGBA{R: 244, G: 63, B: 94, A: 255} // Rose / Ruby

	// Force Powers
	case strings.Contains(idUpper, "FORCE"), strings.Contains(idUpper, "FP_"), category == "Force":
		if strings.Contains(idUpper, "LIGHTNING") || strings.Contains(idUpper, "GRIP") ||
			strings.Contains(idUpper, "DRAIN") || strings.Contains(idUpper, "DESTRUCTION") {
			return color.NRGBA{R: 239, G: 68, B: 68, A: 255} // Dark Side Crimson
		}
		if strings.Contains(idUpper, "HEAL") || strings.Contains(idUpper, "PROTECT") ||
			strings.Contains(idUpper, "ABSORB") {
			return color.NRGBA{R: 74, G: 222, B: 128, A: 255} // Light Side Jade
		}
		return color.NRGBA{R: 168, G: 85, B: 247, A: 255} // Force Purple

	// Physicals & Mobility
	case strings.Contains(idUpper, "STAMINA"), strings.Contains(idUpper, "DEXTERITY"),
		strings.Contains(idUpper, "SPEED"), strings.Contains(idUpper, "DODGE"),
		strings.Contains(idUpper, "DASH"), strings.Contains(idUpper, "BUNNY_HOP"),
		strings.Contains(idUpper, "JETPACK"), strings.Contains(idUpper, "FUEL"):
		return color.NRGBA{R: 45, G: 212, B: 191, A: 255} // Teal / Mint

	// Defenses & Armor
	case strings.Contains(idUpper, "ARMOUR"), strings.Contains(idUpper, "ARMOR"),
		strings.Contains(idUpper, "SHIELD"), strings.Contains(idUpper, "CORTOSIS"),
		strings.Contains(idUpper, "BLAST"), strings.Contains(idUpper, "MAGNETIC"),
		strings.Contains(idUpper, "HEALING"), strings.Contains(idUpper, "RECHARGE"),
		strings.Contains(idUpper, "HEALTH"):
		return color.NRGBA{R: 96, G: 165, B: 250, A: 255} // Steel Blue

	// Class Tech
	case strings.Contains(idUpper, "SBD"):
		return color.NRGBA{R: 148, G: 163, B: 184, A: 255} // SBD Gunmetal
	case strings.Contains(idUpper, "DEKA"):
		return color.NRGBA{R: 217, G: 119, B: 6, A: 255} // Droideka Bronze
	case strings.Contains(idUpper, "CLONE"), strings.Contains(idUpper, "ARC_"):
		return color.NRGBA{R: 37, G: 99, B: 235, A: 255} // Clone Cobalt
	case strings.Contains(idUpper, "MANDO"), strings.Contains(idUpper, "BESKAR"),
		strings.Contains(idUpper, "WRIST"):
		return color.NRGBA{R: 234, G: 179, B: 8, A: 255} // Mando Gold
	case strings.Contains(idUpper, "WOOKIE"):
		return color.NRGBA{R: 161, G: 98, B: 7, A: 255} // Wookiee Warm Wood
	case strings.Contains(idUpper, "RALLY"), strings.Contains(idUpper, "ASSEMBLE"),
		strings.Contains(idUpper, "HERO"):
		return color.NRGBA{R: 132, G: 204, B: 22, A: 255} // Hero Lime

	// Supplies & Medical
	case strings.Contains(idUpper, "BACTA"), strings.Contains(idUpper, "DISP_"),
		strings.Contains(idUpper, "MEDI_"), strings.Contains(idUpper, "AMMO_PACK"),
		strings.Contains(idUpper, "STIMPACK"), strings.Contains(idUpper, "SUPPLY"):
		return color.NRGBA{R: 20, G: 184, B: 166, A: 255} // Medical Aqua
	}

	return color.NRGBA{R: 120, G: 130, B: 145, A: 255} // Calm Slate Default
}

func (w *AttributeToggleWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.container)
}
