package main

import (
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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
	label     *widget.Label
	buttons   []*HoverButton
	infoBtn   *TooltipButton
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
	// Primary label: display name (with auto-derived fallback when the
	// data doesn't carry one). Secondary: monospace enum ID caption
	// underneath so authors who think in source can still recognize
	// the row. Older builds inconsistently showed the raw MB_ATT_*
	// when no display name was set, which produced a mixed grid.
	displayName := w.Name
	if displayName == "" || displayName == w.ID || strings.HasPrefix(displayName, "MB_ATT_") {
		displayName = prettyAttributeName(w.ID)
	}
	w.label = widget.NewLabel(displayName)
	w.label.TextStyle = fyne.TextStyle{Bold: true}

	// Info-affordance: when an icon resolves, the icon IS the click
	// target (clicking it pins the sidebar). When no icon resolves,
	// fall back to the (i) glyph. The previous layout rendered both
	// — wasted horizontal space and made the (i) feel redundant.
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
		// Real icon → wrap in a clickableCell so the whole 28×28 icon
		// is the affordance. Hover on it also fires onInfo (the
		// transient hover dispatcher) so the info panel can preview
		// while the user scans rows.
		raster := NewRasterIconFromResource(iconRes, 28, 28)
		clickable := newClickableCell(raster, infoClick)
		if onInfo != nil {
			clickable.onHover = func() { onInfo(w.ID, "") }
		}
		// onLeave wired below via SetOnInfoLeave indirection.
		iconObj = clickable
		w.infoBtn = nil // signal: no separate (i) glyph needed
	} else {
		// No icon → fall back to the (i) tooltip button. This
		// preserves the "I want the docs for this row" affordance
		// when we don't have a graphic to click on.
		w.infoBtn = NewTooltipButton("", theme.InfoIcon(), infoClick,
			"View documentation for this attribute")
		w.infoBtn.Importance = widget.LowImportance
		// Fixed 24×24 spacer so the no-icon branch lines up
		// vertically with the icon branch — Spacer would expand and
		// re-introduce the empty-rectangle visual bug.
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

	// Category color — drives the left strip and the tile's accent
	// border. Re-uses the same palette as the info-panel's category
	// chip so the same row identity reads consistently across surfaces.
	var catColor color.Color
	switch w.Category {
	case "Force":
		catColor = color.RGBA{0, 191, 255, 255} // Deep Sky Blue
	case "Saber":
		catColor = color.RGBA{255, 69, 0, 255} // Orange Red
	case "Weapons":
		catColor = color.RGBA{255, 215, 0, 255} // Gold
	case "Class Specific":
		catColor = color.RGBA{50, 205, 50, 255} // Lime Green
	case "Supply":
		catColor = color.RGBA{195, 130, 80, 255} // Bronze
	case "Regen":
		catColor = color.RGBA{120, 200, 140, 255} // Mint
	case "Multipliers":
		catColor = color.RGBA{180, 140, 220, 255} // Lavender
	case "Advanced":
		catColor = color.RGBA{100, 100, 110, 255} // Slate
	default:
		catColor = color.RGBA{128, 128, 128, 255} // Grey
	}

	// Two-row label block: bold display name on top, monospace enum
	// ID caption underneath in muted text. Lets authors who think in
	// source still recognize the row without sacrificing the warmer
	// display name as the primary affordance.
	idCaption := canvas.NewText(w.ID, theme.PlaceHolderColor())
	idCaption.TextSize = SizeSmall
	idCaption.TextStyle = fyne.TextStyle{Monospace: true}
	labelBlock := container.NewVBox(w.label, idCaption)

	// Slim left-edge accent strip — 2px wide, rounded. Re-introduced
	// after the section TilePanel proved insufficient on its own as a
	// per-row family cue: when 30+ attributes share the same section
	// background, the per-category color (Force = blue, Saber = orange,
	// Weapons = gold) needs a per-row marker too. The strip is much
	// thinner than the original 3px chunky bar so it reads as a hairline
	// hint rather than a card border.
	stripRect := canvas.NewRectangle(catColor)
	stripRect.CornerRadius = 1
	stripRect.SetMinSize(fyne.NewSize(2, 0))
	strip := container.New(layout.NewGridWrapLayout(fyne.NewSize(2, 36)), stripRect)

	// Layout: [Strip] [Icon-or-(i)] [Label+ID] -- Spacer -- Buttons
	// iconObj already carries the click affordance — when an icon
	// resolves it IS the info button; when none resolves the (i)
	// tooltip button stands in. Each child is wrapped in NewCenter
	// so the heights line up against the row's tallest element
	// (label block is 2 lines ~30px, icon is 28px, buttons box is
	// ~28px) — without center-wrap the icon was pinning to the top
	// of the row instead of sitting on the label baseline.
	leftContainer := container.NewHBox(
		container.NewCenter(strip),
		container.NewCenter(iconObj),
		container.NewCenter(labelBlock),
	)

	row := container.NewBorder(nil, nil,
		leftContainer,
		container.NewCenter(btnBox),
		layout.NewSpacer(),
	)

	// Per-attribute fill: moderate alpha (12/55) gives each row visible
	// family identity (Force is blueish, Saber is orange-red, etc.)
	// without competing with the section TilePanel (~20/70) above. The
	// previous near-flat 4/22 felt washed out — rows blurred together
	// against the section bg, losing the per-row category cue. Tuned
	// so that the section reads as the *area* and the row reads as a
	// *colored chip inside the area*.
	tile := NewTilePanel(row, TileOpts{
		AccentColor: catColor,
		FillAlpha:   12,
		StrokeAlpha: 55,
		Padded:      true,
	})

	// Wrap the whole tile in a HoverContainer so mousing over the
	// label/icon/strip area fires the info-panel hover the same way
	// hovering a level button does. Previously only level buttons
	// reported hover, so the info panel rarely repainted unless the
	// user's mouse landed precisely on a numeric pill.
	hover := NewHoverContainer(tile, func() {
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
		// Clicking Off when the row is already off cycles UP to
		// level 1 — interpretation of "click Off to turn it on
		// since it's currently off." Tester reported assuming Off
		// toggled the attribute on, which it didn't (set 0 → 0
		// was a no-op). Now Off acts as a unified on/off toggle:
		//   - off + click Off  → level 1 (turn on)
		//   - on  + click Off  → 0       (turn off)
		// Clicking 1/2/3 directly always sets that exact level.
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
		// MouseOut — tell the info panel to revert to sticky.
		// Closure captures the widget; OnInfoLeave may be set later
		// via SetOnInfoLeave so read it lazily each time.
		if w.OnInfoLeave != nil {
			w.OnInfoLeave()
		}
	})
	return btn
}

func (w *AttributeToggleWidget) refreshButtons() {
	// All pills always visible — hiding the 1/2/3 buttons when the row
	// is OFF made it impossible to *turn on* an attribute (only the Off
	// pill rendered, so there was nothing to click). The active level's
	// pill gets HighImportance for visual emphasis; the rest stay
	// MediumImportance. External tester reported "nothing in the
	// Attributes tab did anything except those already written level
	// could be changed" — that was this bug.
	for i, btn := range w.buttons {
		if i == w.CurrentVal {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Show()
		btn.Refresh()
	}
}

func (w *AttributeToggleWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.container)
}

