package main

// ClassIconPicker — horizontal row of compact icon cards, one per
// live MB_CLASS_* enum. Click a card to select. Replaces the flat
// widget.Select dropdown that used to live on the MBCH editor's
// Profile tab, because "pick a class" is a fundamentally visual
// action — the class icon carries more identity than the enum name.
//
// Design notes:
//   - Cards are small (~44px icon + caption) so the row fits a single
//     strip in the form without dominating the layout.
//   - Active card has an accent-colored border + subtle tint fill;
//     inactive cards render as a quiet icon + name pair.
//   - Icons come from assets/icons/classes/ which tools/extract-icons
//     populated from MBAssets2.pk3. Filename-to-enum mapping is via
//     classIconAliases below, since MBII shortens names in the
//     filesystem (MB_CLASS_BOUNTY_HUNTER → bh.png, SBD → sbd.png).
//   - Classes that don't have an icon (NOCLASS, etc.) are not
//     rendered at all — the hidden-content filter via GetClasses()
//     already drops them.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// classIconAliases maps MB_CLASS_* enum values to the basename of
// their embedded class icon. Sourced by inspecting
// assets/icons/classes/ after extraction from MBAssets2.pk3's
// gfx/menus/classes/ directory.
var classIconAliases = map[string]string{
	"MB_CLASS_SOLDIER":       "reb", // Rebel soldier
	"MB_CLASS_TROOPER":       "imp", // Imperial trooper
	"MB_CLASS_COMMANDER":     "com",
	"MB_CLASS_ELITETROOPER":  "imp", // reuse imp; no dedicated ET icon shipped
	"MB_CLASS_SITH":          "jedi", // no dedicated sith icon; jedi/sith share the "user" art
	"MB_CLASS_JEDI":          "jedi",
	"MB_CLASS_BOUNTY_HUNTER": "bh",
	"MB_CLASS_HERO":          "hero",
	"MB_CLASS_SBD":           "sbd",
	"MB_CLASS_WOOKIE":        "wook",
	"MB_CLASS_DROIDEKA":      "deka",
	"MB_CLASS_CLONETROOPER":  "clone",
	"MB_CLASS_MANDALORIAN":   "manda",
	"MB_CLASS_ARCTROOPER":    "arc",
}

// ClassIconPicker is the composite widget that mbch_editor embeds in
// place of classSelect. Selected holds the current MB_CLASS_* ID;
// onChange fires on selection, onHover/onUnhover on mouse enter/leave
// of a card — the latter pair drives the info panel's transient
// preview state so pointing at a card gives a peek at its docs
// without committing it as sticky.
type ClassIconPicker struct {
	widget.BaseWidget

	selected  string
	onChange  func(string)
	onHover   func(id, context string)
	onUnhover func()

	cards     []*classCard
	container *fyne.Container
}

func NewClassIconPicker(onChange func(string)) *ClassIconPicker {
	p := &ClassIconPicker{onChange: onChange}
	p.ExtendBaseWidget(p)
	p.buildCards()
	return p
}

// SetHoverHandlers wires transient-preview callbacks. onHover fires
// with the hovered card's class ID, onUnhover on mouse-out. Both
// optional — nil disables the preview behavior.
func (p *ClassIconPicker) SetHoverHandlers(onHover func(id, context string), onUnhover func()) {
	p.onHover = onHover
	p.onUnhover = onUnhover
}

func (p *ClassIconPicker) buildCards() {
	// 14 live classes laid out 5-per-row → 3 rows (5+5+4). Better
	// use of vertical space in the Profile tab than a single long
	// strip, and keeps each card larger/readable without shrinking
	// the icons. GridWithColumns distributes width evenly, so the
	// row width flexes with the form pane.
	grid := container.NewGridWithColumns(5)
	for _, c := range GetClasses() {
		card := newClassCard(c, p)
		p.cards = append(p.cards, card)
		grid.Add(card)
	}
	p.container = container.NewStack(grid)
}

// CreateRenderer wires the composite widget into Fyne's render tree.
// All the real layout lives in the container built by buildCards.
func (p *ClassIconPicker) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(p.container)
}

// SetSelected marks the given class ID as active without firing
// onChange — used when the editor loads a file and needs to reflect
// the file's class in the picker. Passing "" clears selection.
func (p *ClassIconPicker) SetSelected(id string) {
	p.selected = id
	for _, c := range p.cards {
		c.setActive(c.def.ID == id)
	}
}

// Selected returns the currently selected class ID.
func (p *ClassIconPicker) Selected() string { return p.selected }

// selectFromCard is called by a card on tap. Fires onChange if the
// selection actually changed.
func (p *ClassIconPicker) selectFromCard(def ClassDef) {
	if p.selected == def.ID {
		return
	}
	p.selected = def.ID
	for _, c := range p.cards {
		c.setActive(c.def.ID == def.ID)
	}
	if p.onChange != nil {
		p.onChange(def.ID)
	}
}

// classCard is a single picker cell — icon + short caption, hoverable,
// clickable, with an active/inactive visual state.
type classCard struct {
	widget.BaseWidget

	def     ClassDef
	owner   *ClassIconPicker
	active  bool
	hovered bool

	bg      *canvas.Rectangle
	border  *canvas.Rectangle
	iconObj fyne.CanvasObject // raster-friendly wrapper (see NewRasterIconFromResource)
	caption *canvas.Text
}

func newClassCard(def ClassDef, owner *ClassIconPicker) *classCard {
	c := &classCard{def: def, owner: owner}
	c.ExtendBaseWidget(c)
	return c
}

func (c *classCard) CreateRenderer() fyne.WidgetRenderer {
	c.bg = canvas.NewRectangle(color.Transparent)
	c.border = canvas.NewRectangle(color.Transparent)
	c.border.StrokeWidth = 2
	c.border.FillColor = color.Transparent

	// Class icon — loaded from embedded assets via LoadGameIcon.
	// Uses the alias table to turn MB_CLASS_* → filename basename;
	// LoadGameIcon then looks up the PNG in embedIcons. Renders via
	// canvas.Image inside a GridWrap — widget.Icon tries to respect
	// theme icon size (~20px) for SVG tinting and leaves raster PNGs
	// looking like a tiny dot in the middle of a 44×44 cell. Using
	// canvas.Image with ImageFillContain fills the whole box.
	alias := classIconAliases[c.def.ID]
	if alias != "" {
		if img, ok := LoadGameIcon(nil, "classes/"+alias); ok {
			ci := canvas.NewImageFromImage(img)
			ci.FillMode = canvas.ImageFillContain
			ci.ScaleMode = canvas.ImageScaleSmooth
			ci.SetMinSize(fyne.NewSize(44, 44))
			c.iconObj = container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 44)), ci)
		}
	}
	if c.iconObj == nil {
		// Fallback — theme person icon, which is actually an SVG so
		// widget.Icon renders it fine at theme size inside the grid.
		fb := widget.NewIcon(theme.AccountIcon())
		c.iconObj = container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 44)), fb)
	}

	c.caption = canvas.NewText(prettyClassName(c.def), theme.ForegroundColor())
	c.caption.TextSize = SizeSmall
	c.caption.TextStyle = fyne.TextStyle{Bold: true}
	c.caption.Alignment = fyne.TextAlignCenter

	// Center both icon + caption horizontally inside the card. VBox
	// alone left-aligns its children; wrapping each row in a Center
	// container means the 44x44 icon and the caption text both
	// track the card's horizontal midline regardless of how wide
	// the card ends up (GridWithColumns flexes the card width).
	iconHost := container.NewCenter(c.iconObj)
	captionHost := container.NewCenter(c.caption)
	body := container.NewVBox(iconHost, captionHost)
	inner := container.NewPadded(body)
	cell := container.NewStack(c.bg, c.border, inner)

	c.applyStyle()
	return widget.NewSimpleRenderer(cell)
}

// setActive is called by the owning picker when selection changes;
// kept tight because it can fire 14 times per selection.
func (c *classCard) setActive(on bool) {
	if c.active == on {
		return
	}
	c.active = on
	c.applyStyle()
}

func (c *classCard) Tapped(*fyne.PointEvent) {
	c.owner.selectFromCard(c.def)
}

func (c *classCard) MouseIn(*desktop.MouseEvent) {
	c.hovered = true
	c.applyStyle()
	if c.owner.onHover != nil {
		c.owner.onHover(c.def.ID, "Class Definition")
	}
}

func (c *classCard) MouseOut() {
	c.hovered = false
	c.applyStyle()
	if c.owner.onUnhover != nil {
		c.owner.onUnhover()
	}
}

func (c *classCard) MouseMoved(*desktop.MouseEvent) {}

func (c *classCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

// applyStyle paints the card's bg/border based on active + hover
// state. Accent color drives the active + hover tints — same palette
// used by sidebar pills and tile cards so the picker visually
// coheres with the rest of the chrome.
func (c *classCard) applyStyle() {
	if c.bg == nil || c.border == nil {
		return
	}
	switch {
	case c.active:
		c.bg.FillColor = tintWithAlpha(CurrentThemeColor, 80)
		c.border.StrokeColor = CurrentThemeColor
	case c.hovered:
		c.bg.FillColor = tintWithAlpha(CurrentThemeColor, 40)
		c.border.StrokeColor = color.Transparent
	default:
		c.bg.FillColor = color.Transparent
		c.border.StrokeColor = color.Transparent
	}
	c.bg.Refresh()
	c.border.Refresh()
}

// prettyClassName turns MB_CLASS_ELITETROOPER into "Elite Trooper".
// Class JSON sometimes has a polished Name but older loads leave it
// as the raw ID — normalize either way so the caption reads nicely.
func prettyClassName(c ClassDef) string {
	if c.Name != "" && !strings.HasPrefix(c.Name, "MB_CLASS_") {
		return c.Name
	}
	// Derive from ID.
	s := strings.TrimPrefix(c.ID, "MB_CLASS_")
	// Special cases the naive splitter mangles.
	switch s {
	case "SBD":
		return "SBD"
	case "ARCTROOPER":
		return "ARC Trooper"
	case "ELITETROOPER":
		return "Elite Trooper"
	case "CLONETROOPER":
		return "Clone"
	case "BOUNTY_HUNTER":
		return "Bounty Hunter"
	case "WOOKIE":
		return "Wookiee"
	case "DROIDEKA":
		return "Droideka"
	case "MANDALORIAN":
		return "Mandalorian"
	case "COMMANDER":
		return "Commander"
	case "SOLDIER":
		return "Soldier"
	case "TROOPER":
		return "Trooper"
	case "JEDI":
		return "Jedi"
	case "SITH":
		return "Sith"
	case "HERO":
		return "Hero"
	}
	// Fallback: snake_case → Title Case.
	return strings.Title(strings.ToLower(strings.ReplaceAll(s, "_", " ")))
}
