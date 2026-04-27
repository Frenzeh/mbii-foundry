package main

// HoldableGrid — circular-badge picker for HI_* inventory items.
// Lives on the Inventory tab below the weapon list.
//
// Each HI_* renders as a small card with a colored circle showing
// the level (0 = empty/dim, 1+ = filled with level number). Click
// the circle to cycle 0 → 1 → 2 → … → max → 0. The level is
// persisted into the same `attributes` pipe-string the MB_ATT_*
// grid writes to, since MBII's MBCH treats HI_* as another kind of
// attribute entry: `HI_MEDPAC,1|HI_BINOCULARS,1|MB_ATT_HEAL,3`.
//
// Visually distinct from the weapon cards: smaller (a wide row of
// small badges), no per-attribute fill — just the circle itself
// changes color/saturation by ownership state. Tester ask:
// "solid circles with numbers in them and the surface changing
// whether or not it is bought or selected."

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// HoldableDef is the static catalog row for a HI_* inventory item.
// MaxLevel of 1 covers the typical "have / don't have" model;
// items with stacking (e.g. some grenades) bump it up.
type HoldableDef struct {
	ID       string
	Name     string
	MaxLevel int
	Family   string // "Healing", "Vision", "Deployable", "Mobility", "Other"
	Icon     string // basename in assets/icons/attributes/ — optional
}

// KnownHoldables is the curated catalog of HI_* items the editor
// surfaces. Sourced from data/attributes.json's HI_* rows + the
// definitions/holdables/ markdown set; family + icon mappings
// chosen so the grid reads as a logical Inventory loadout.
var KnownHoldables = []HoldableDef{
	{"HI_MEDPAC", "Medpac", 1, "Healing", "i_icon_medkit"},
	{"HI_MEDPAC_BIG", "Big Medpac", 1, "Healing", "i_icon_bacta"},
	{"HI_STIMPACK", "Stimpack", 1, "Healing", "i_icon_medkit"},
	{"HI_BINOCULARS", "Binoculars", 1, "Vision", "i_icon_goggles"},
	{"HI_CLOAK", "Cloak", 1, "Vision", "i_icon_cloak"},
	{"HI_SEEKER", "Seeker Drone", 1, "Deployable", "i_icon_seeker"},
	{"HI_SENTRY_GUN", "Sentry Gun", 1, "Deployable", "i_icon_sentrygun"},
	{"HI_EWEB", "E-Web", 1, "Deployable", "i_icon_eweb"},
	{"HI_SHIELD", "Shield Wall", 1, "Deployable", "i_icon_shieldwall"},
}

type HoldableGrid struct {
	container *fyne.Container
	values    map[string]int // HI_ID → level
	onChange  func(string)   // called with serialized "HI_X,1|HI_Y,1"
	onHover   func(string, string)
	onUnhover func()
}

// NewHoldableGrid builds the grid and parses an initial pipe-string
// (the same one the AttributeGrid uses — they share the field).
func NewHoldableGrid(initialStr string, onChange func(string), onHover func(string, string)) *HoldableGrid {
	hg := &HoldableGrid{
		values:   parseHoldablesString(initialStr),
		onChange: onChange,
		onHover:  onHover,
	}
	hg.createUI()
	return hg
}

// SetOnUnhover wires the mouse-leave hook so info-panel hover state
// reverts to the user's last sticky selection.
func (hg *HoldableGrid) SetOnUnhover(f func()) { hg.onUnhover = f }

// parseHoldablesString pulls the HI_* entries out of a pipe-separated
// attributes string. Non-HI tokens are ignored — the caller (the
// MBCH editor) keeps the full attributes string elsewhere.
func parseHoldablesString(s string) map[string]int {
	res := map[string]int{}
	if s == "" {
		return res
	}
	for _, tok := range strings.Split(s, "|") {
		tok = strings.TrimSpace(tok)
		if !strings.HasPrefix(tok, "HI_") {
			continue
		}
		parts := strings.SplitN(tok, ",", 2)
		id := strings.TrimSpace(parts[0])
		val := 1
		if len(parts) > 1 {
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &val)
		}
		if val > 0 {
			res[id] = val
		}
	}
	return res
}

// createUI builds the row of family-grouped circle badges.
func (hg *HoldableGrid) createUI() {
	families := []string{"Healing", "Vision", "Deployable", "Mobility", "Other"}
	groups := map[string][]HoldableDef{}
	for _, h := range KnownHoldables {
		fam := h.Family
		if fam == "" {
			fam = "Other"
		}
		groups[fam] = append(groups[fam], h)
	}
	for fam := range groups {
		sort.SliceStable(groups[fam], func(i, j int) bool {
			return groups[fam][i].Name < groups[fam][j].Name
		})
	}

	body := container.NewVBox()
	for _, fam := range families {
		members, ok := groups[fam]
		if !ok || len(members) == 0 {
			continue
		}
		body.Add(hg.buildFamilyTile(fam, members))
	}
	hg.container = body
}

// buildFamilyTile renders one family group as a section TilePanel
// with a row of circle badges inside.
func (hg *HoldableGrid) buildFamilyTile(family string, members []HoldableDef) fyne.CanvasObject {
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("%s  ·  %d", family, len(members)),
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	row := container.NewGridWrap(fyne.NewSize(96, 72))
	for _, m := range members {
		row.Add(hg.buildBadge(m))
	}
	return NewTilePanel(
		container.NewVBox(header, row),
		TileOpts{
			AccentColor: holdableFamilyAccent(family),
			FillAlpha:   18,
			StrokeAlpha: 60,
			Padded:      true,
		},
	)
}

// buildBadge is one HI_* card — circle (no number) + label.
// Surface state communicates ownership: off = dim grey unfilled,
// owned = saturated family-color fill. The user said the level
// number inside the circle wasn't useful for HI_* (most are 1-of
// anyway) so the circle is now a pure on/off pip.
func (hg *HoldableGrid) buildBadge(h HoldableDef) fyne.CanvasObject {
	level := hg.values[h.ID]
	owned := level > 0

	// Safe NRGBA extraction — the family-accent helper currently
	// always returns NRGBA, but a hard `.(color.NRGBA)` assertion
	// would panic if any future caller / refactor handed us an RGBA
	// or interface value. Type-switch + RGBA() conversion fallback
	// keeps this path defensive.
	famAccent := holdableFamilyAccent(h.Family)
	var na color.NRGBA
	switch v := famAccent.(type) {
	case color.NRGBA:
		na = v
	default:
		r, g, b, a := famAccent.RGBA()
		na = color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
	var fill color.Color = color.NRGBA{R: 50, G: 50, B: 60, A: 220}
	var border = color.NRGBA{R: 90, G: 90, B: 100, A: 255}
	var textCol color.Color = color.NRGBA{R: 180, G: 180, B: 190, A: 255}
	if owned {
		fill = color.NRGBA{R: na.R, G: na.G, B: na.B, A: 230}
		border = color.NRGBA{R: na.R, G: na.G, B: na.B, A: 255}
		// Slight contrast bump on the label when selected.
		textCol = color.NRGBA{R: 230, G: 230, B: 235, A: 255}
	}

	circle := canvas.NewCircle(fill)
	circle.StrokeColor = border
	circle.StrokeWidth = 2
	badge := container.NewGridWrap(fyne.NewSize(22, 22), circle)

	label := canvas.NewText(h.Name, textCol)
	label.Alignment = fyne.TextAlignCenter
	label.TextSize = 11
	if owned {
		label.TextStyle = fyne.TextStyle{Bold: true}
	}

	cell := container.NewVBox(
		container.NewCenter(badge),
		label,
	)

	clickable := newClickableCell(cell, func() {
		next := level + 1
		if next > h.MaxLevel {
			next = 0
		}
		hg.setLevel(h.ID, next)
	})
	if hg.onHover != nil {
		clickable.onHover = func() {
			hg.onHover(h.ID, "")
		}
	}
	if hg.onUnhover != nil {
		clickable.onLeave = hg.onUnhover
	}
	return clickable
}

// setLevel mutates the in-memory values map and notifies callers
// via onChange with the serialized HI_* slice. The MBCH editor
// merges this back into the full attributes pipe-string.
func (hg *HoldableGrid) setLevel(id string, level int) {
	// Defensive: writing to a nil map panics. parseHoldablesString
	// always returns a non-nil map, but a future caller might
	// assign hg.values directly without going through the parser.
	if hg.values == nil {
		hg.values = map[string]int{}
	}
	if level <= 0 {
		delete(hg.values, id)
	} else {
		hg.values[id] = level
	}
	hg.TriggerChange()
	// Inline refresh — fyne.Do from main thread deadlocked Fyne
	// v2.7.1's dispatch queue (sample dump confirmed every thread
	// in __psynch_cvwait). Inline tree rebuild during a click
	// handler works in practice.
	hg.Refresh()
}

// TriggerChange fires the onChange callback with a serialized
// HI_X,1|HI_Y,1 string. Caller is responsible for merging this
// with the rest of the attribute set.
func (hg *HoldableGrid) TriggerChange() {
	if hg.onChange == nil {
		return
	}
	keys := make([]string, 0, len(hg.values))
	for k := range hg.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s,%d", k, hg.values[k]))
	}
	hg.onChange(strings.Join(parts, "|"))
}

// Refresh rebuilds the body in place when the values map changes.
func (hg *HoldableGrid) Refresh() {
	if hg.container == nil {
		hg.createUI()
		return
	}
	hg.container.Objects = nil
	families := []string{"Healing", "Vision", "Deployable", "Mobility", "Other"}
	groups := map[string][]HoldableDef{}
	for _, h := range KnownHoldables {
		fam := h.Family
		if fam == "" {
			fam = "Other"
		}
		groups[fam] = append(groups[fam], h)
	}
	for _, fam := range families {
		members, ok := groups[fam]
		if !ok || len(members) == 0 {
			continue
		}
		hg.container.Add(hg.buildFamilyTile(fam, members))
	}
	hg.container.Refresh()
}

func (hg *HoldableGrid) GetContent() fyne.CanvasObject { return hg.container }

// holdableFamilyAccent — same palette idea as weapon families but
// tuned per holdable family.
func holdableFamilyAccent(family string) color.Color {
	switch family {
	case "Healing":
		return color.NRGBA{R: 220, G: 110, B: 130, A: 255}
	case "Vision":
		return color.NRGBA{R: 130, G: 170, B: 220, A: 255}
	case "Deployable":
		return color.NRGBA{R: 220, G: 170, B: 100, A: 255}
	case "Mobility":
		return color.NRGBA{R: 140, G: 200, B: 140, A: 255}
	}
	return color.NRGBA{R: 160, G: 160, B: 170, A: 255}
}

// clickableCell wraps an arbitrary CanvasObject with a tap handler
// + hover callbacks. Used by holdable badges so the entire card area
// is the click target instead of just the inner circle.
type clickableCell struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
	onHover func()
	onLeave func()
}

func newClickableCell(content fyne.CanvasObject, onTap func()) *clickableCell {
	c := &clickableCell{content: content, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *clickableCell) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *clickableCell) MouseIn(*desktop.MouseEvent) {
	if c.onHover != nil {
		c.onHover()
	}
}

func (c *clickableCell) MouseOut() {
	if c.onLeave != nil {
		c.onLeave()
	}
}

func (c *clickableCell) MouseMoved(*desktop.MouseEvent) {}

func (c *clickableCell) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.content)
}
