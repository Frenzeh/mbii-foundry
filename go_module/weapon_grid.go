package main

// WeaponGrid — the Inventory tab's weapon picker.
//
// Each weapon renders as a full-width card showing:
//   • 48px in-game HUD icon on the left
//   • Weapon name (bold) + WP_* ID (monospace, dim) below it
//   • Inline level pills (0..MaxLevel) bound to the paired MB_ATT_*
//     attribute — clicking a pill toggles weapon ownership *and*
//     sets the attribute level in lockstep, so the WP_X / MB_ATT_X
//     pair stays consistent without users having to bounce between
//     two grids
//   • Right-hand metadata column: "Flags (n)" badge if HELD_* flags
//     target this weapon, "Override" badge if a WeaponInfoN block
//     targets this weapon
//
// The richer layout replaces the previous 2-column grid of bare
// checkboxes — the icons weren't visible, the paired attribute lived
// in a sibling tab, and any HELD_* flags or per-class overrides on
// the same weapon were invisible from here.

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type WeaponGrid struct {
	container   *fyne.Container
	selected    map[string]bool
	onChange    func(string)
	onHover     func(string, string)
	onUnhover   func()
	resolveIcon func(string) fyne.Resource

	// Cross-tab integration. All optional — if any are nil the card
	// renders without the corresponding affordance.
	attrLevelGetter    func(attID string) int    // current level of the paired MB_ATT_*
	attrLevelSetter    func(attID string, n int) // set the paired MB_ATT_* level
	flagsCountGetter   func(wpID string) int     // # of HELD_* flags applied to wpID
	overrideExists     func(wpID string) bool    // true if a WeaponInfoN targets wpID
	onOverrideJump     func(wpID string)         // navigate to Weapon Mods + select
	onFlagsJump        func(wpID string)         // navigate to Flags + select
	onClickInfo        func(string, string)      // click on weapon icon → pin sidebar

	filter string
	search *widget.Entry
}

// NewWeaponGrid creates a weapon picker. resolveIcon is optional —
// when provided, it's called per-weapon with the WP_ ID and should
// return a fyne.Resource rendering the in-game icon (typically
// gfx/hud/w_icon_*.tga decoded + PNG-cached via AssetBrowser's
// LoadIconResource). Passing nil falls back to a plain checkbox
// with no leading image, matching the previous behavior.
func NewWeaponGrid(initialStr string, onChange func(string), onHover func(string, string), resolveIcon func(string) fyne.Resource) *WeaponGrid {
	wg := &WeaponGrid{
		selected:    make(map[string]bool),
		onChange:    onChange,
		onHover:     onHover,
		resolveIcon: resolveIcon,
	}
	wg.parseString(initialStr)
	wg.createUI()
	return wg
}

// SetOnUnhover registers a callback that fires when the mouse leaves
// a weapon row. Paired with the onHover already in the constructor,
// this lets the info panel revert its hover-view to whatever the
// user last interacted with — otherwise the panel sticks on the
// last weapon the mouse passed over, even after the user has moved
// on to a different field.
func (wg *WeaponGrid) SetOnUnhover(f func()) { wg.onUnhover = f }

// SetOnClickInfo wires the per-card click → sticky-context route.
// Click on the weapon icon (or anywhere on the card) pins the
// sidebar to that weapon's docs regardless of the hover toggle.
// Symmetric with AttributeGrid.SetOnClickInfo.
func (wg *WeaponGrid) SetOnClickInfo(f func(string, string)) {
	wg.onClickInfo = f
}

// SetAttributeBridge wires the WP_X ↔ MB_ATT_X cross-tab integration.
// Both getter and setter are required for the level pills to render —
// if either is nil, the card falls back to a plain checkbox.
func (wg *WeaponGrid) SetAttributeBridge(get func(string) int, set func(string, int)) {
	wg.attrLevelGetter = get
	wg.attrLevelSetter = set
}

// SetFlagsBridge wires the HELD_* flags badge. count returns how many
// flags target the given weapon; jump navigates to the Flags tab so
// the user can edit them. Either may be nil.
func (wg *WeaponGrid) SetFlagsBridge(count func(string) int, jump func(string)) {
	wg.flagsCountGetter = count
	wg.onFlagsJump = jump
}

// SetOverrideBridge wires the Override badge. exists reports whether
// any WeaponInfoN targets the given weapon; jump navigates to the
// Weapon Mods tab and selects that override.
func (wg *WeaponGrid) SetOverrideBridge(exists func(string) bool, jump func(string)) {
	wg.overrideExists = exists
	wg.onOverrideJump = jump
}

func (wg *WeaponGrid) parseString(s string) {
	wg.selected = make(map[string]bool)
	if s == "" {
		return
	}
	parts := strings.Split(s, "|")
	for _, p := range parts {
		wg.selected[strings.TrimSpace(p)] = true
	}
}

func (wg *WeaponGrid) createUI() {
	categories := make(map[string][]WeaponDef)
	weapons := GetWeapons()
	for _, w := range weapons {
		categories[w.Category] = append(categories[w.Category], w)
	}

	catOrder := []string{"Melee/Force", "Sidearms", "Rifles", "Heavy"}

	var content *fyne.Container
	var mainLayout *fyne.Container

	if wg.container != nil {
		mainLayout = wg.container
		// Border container's Objects slice doesn't have a guaranteed
		// order between center and edges. Scan for the Scroll rather
		// than indexing blindly.
		for _, obj := range mainLayout.Objects {
			if scroll, ok := obj.(*container.Scroll); ok {
				if c, ok := scroll.Content.(*fyne.Container); ok {
					content = c
					content.Objects = nil
				}
				break
			}
		}
	} else {
		content = container.NewVBox()

		wg.search = NewInputEntry()
		wg.search.SetPlaceHolder("Filter weapons (name or WP_ ID)…")
		wg.search.OnChanged = func(s string) {
			wg.filter = s
			wg.Refresh()
		}

		// Legend/header explaining the card layout to first-time users.
		legend := widget.NewLabelWithStyle(
			"Click a level pill to set the paired attribute. Off = weapon not on the class.",
			fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
		header := container.NewVBox(wg.search, legend)
		scroll := container.NewVScroll(content)
		mainLayout = container.NewBorder(header, nil, nil, nil, scroll)
		wg.container = mainLayout
	}

	filterLower := strings.ToLower(wg.filter)

	for _, catName := range catOrder {
		weaponsInCat, ok := categories[catName]
		if !ok {
			continue
		}

		var visible []WeaponDef
		for _, w := range weaponsInCat {
			if filterLower == "" ||
				strings.Contains(strings.ToLower(w.Name), filterLower) ||
				strings.Contains(strings.ToLower(w.ID), filterLower) {
				visible = append(visible, w)
			}
		}

		if len(visible) == 0 {
			continue
		}

		header := widget.NewLabelWithStyle(
			fmt.Sprintf("%s  (%d)", catName, len(visible)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		content.Add(header)

		// Single-column list of full-width cards. The 2-column grid was
		// too tight for the new metadata band — full-width gives icons
		// + name + level pills + badges room to breathe and reads more
		// like a loadout card than a checkbox grid.
		for _, w := range visible {
			content.Add(wg.buildCard(w))
		}
		content.Add(widget.NewSeparator())
	}

	if len(content.Objects) == 0 {
		hint := "No weapons match the current filter."
		if filterLower != "" {
			hint = fmt.Sprintf("No weapons match \"%s\".", wg.filter)
		}
		content.Add(NewEmptyStateTile("NO RESULTS", hint, "Clear filter", func() {
			if wg.search != nil {
				wg.search.SetText("")
			}
		}))
	}

	if wg.container != nil {
		wg.container.Refresh()
	}
}

// buildCard renders the icon-forward card for one weapon. The icon
// (when resolved) is the primary affordance: a 64×64 extracted PNG
// fills the left edge so authors recognize the weapon by its in-game
// art instead of by reading WP_*. Name + paired-attribute monospace
// caption sit beside it; level pills inline on the right; badges
// stacked on the far right. Surface fills brighter as the level
// climbs — Off → faint, 1 → moderate, 2/3 → saturated — so the
// loadout's hot-pick weapons stand out at a glance.
func (wg *WeaponGrid) buildCard(w WeaponDef) fyne.CanvasObject {
	weaponID := w.ID
	owned := wg.selected[weaponID]
	pair := CanonicalAttributeFor(weaponID)

	// Active level for the paired attribute — used to scale the
	// card's surface and pill emphasis. 0 when no bridge is wired.
	var activeLevel int
	if pair != "" && wg.attrLevelGetter != nil && owned {
		activeLevel = wg.attrLevelGetter(pair)
	} else if owned {
		activeLevel = 1
	}

	// Icon (64×64, embedded HUD art when available). Wrapped in a
	// clickableCell so clicking the icon pins this weapon's docs in
	// the sidebar via the sticky-context path, regardless of the
	// hover toggle. Hover on the icon also fires the transient
	// onHover preview — same affordance the (i) button used to give.
	var iconObj fyne.CanvasObject
	if wg.resolveIcon != nil {
		if res := wg.resolveIcon(weaponID); res != nil {
			iconObj = NewRasterIconFromResource(res, 64, 64)
		}
	}
	if iconObj == nil {
		iconObj = container.NewGridWrap(fyne.NewSize(64, 64),
			widget.NewIcon(theme.QuestionIcon()))
	}
	iconClickable := newClickableCell(iconObj, func() {
		if wg.onClickInfo != nil {
			wg.onClickInfo(weaponID, "")
		}
	})
	if wg.onHover != nil {
		iconClickable.onHover = func() { wg.onHover(weaponID, w.Description) }
	}
	if wg.onUnhover != nil {
		iconClickable.onLeave = wg.onUnhover
	}
	iconObj = iconClickable

	// Compact title block: name (bold) + paired MB_ATT_ caption
	// (small monospace). Dropped the duplicate WP_* line — the icon
	// + name carry that already, and the paired attribute is the
	// useful technical handle.
	nameLbl := widget.NewLabelWithStyle(w.Name,
		fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	var subtitle string
	if pair != "" {
		subtitle = pair
	} else {
		subtitle = weaponID
	}
	subLbl := canvas.NewText(subtitle, theme.PlaceHolderColor())
	subLbl.TextSize = SizeSmall
	subLbl.TextStyle = fyne.TextStyle{Monospace: true}
	titleStack := container.NewVBox(nameLbl, subLbl)

	// Level pills — render only when there's a paired MB_ATT_* AND
	// the bridge callbacks are wired. Without the bridge, the user
	// has no way to set the paired attribute from this card, so we
	// fall back to a plain Off/On toggle.
	var actionRow fyne.CanvasObject
	if pair != "" && wg.attrLevelGetter != nil && wg.attrLevelSetter != nil {
		actionRow = wg.buildLevelPills(weaponID, pair, w)
	} else {
		check := widget.NewCheck("Equip", func(on bool) {
			wg.toggleWeapon(weaponID, on)
		})
		check.Checked = owned
		actionRow = check
	}

	// Badge column — flag count + override-exists.
	var badges []fyne.CanvasObject
	if wg.flagsCountGetter != nil {
		if n := wg.flagsCountGetter(weaponID); n > 0 {
			tip := fmt.Sprintf("%d HELD_* flag(s) applied — click to edit on the Flags tab", n)
			badges = append(badges, NewTooltipButton(fmt.Sprintf("F·%d", n), theme.GridIcon(),
				func() {
					if wg.onFlagsJump != nil {
						wg.onFlagsJump(weaponID)
					}
				}, tip))
		}
	}
	if wg.overrideExists != nil && wg.overrideExists(weaponID) {
		badges = append(badges, NewTooltipButton("M", theme.SettingsIcon(),
			func() {
				if wg.onOverrideJump != nil {
					wg.onOverrideJump(weaponID)
				}
			}, "WeaponInfo override exists — click to edit on Weapon Mods"))
	}
	var badgeCol fyne.CanvasObject
	if len(badges) > 0 {
		badgeCol = container.NewHBox(badges...)
	}

	body := container.NewBorder(
		nil, nil,
		container.NewHBox(iconObj, container.NewPadded(titleStack)),
		badgeCol,
		container.New(layout.NewCenterLayout(), actionRow),
	)

	// Surface chrome scales with level: Off → faint, 1 → moderate,
	// 2 → strong, 3 → saturated. Gives the loadout a heat-map feel
	// where the most-leveled weapons pop visually.
	fillA, strokeA := uint8(8), uint8(40)
	switch {
	case activeLevel >= 3:
		fillA, strokeA = 50, 160
	case activeLevel == 2:
		fillA, strokeA = 36, 130
	case activeLevel == 1:
		fillA, strokeA = 22, 100
	}
	tile := NewTilePanel(body, TileOpts{
		AccentColor: w.AccentColor(),
		FillAlpha:   fillA,
		StrokeAlpha: strokeA,
		Padded:      true,
	})

	hover := NewHoverContainer(tile, func() {
		if wg.onHover != nil {
			wg.onHover(weaponID, w.Description)
		}
	})
	if wg.onUnhover != nil {
		hover.SetOnLeave(wg.onUnhover)
	}
	return hover
}

// buildLevelPills draws the 0..N pill row that controls both weapon
// ownership and the paired attribute level. Pill 0 = "Off" (weapon
// removed + attribute deleted); pills 1..N = own + set attribute level.
// MaxLevel comes from the AttributeDef when known, defaulting to 3
// (the common case for most weapon attributes).
func (wg *WeaponGrid) buildLevelPills(weaponID, attID string, w WeaponDef) fyne.CanvasObject {
	maxLevel := 3
	for _, a := range MBIIAttributes {
		if a.ID == attID && a.MaxLevel > 0 {
			maxLevel = a.MaxLevel
			break
		}
	}
	current := wg.attrLevelGetter(attID)
	owned := wg.selected[weaponID]
	if !owned {
		current = 0
	}

	pills := []fyne.CanvasObject{}
	// Refresh runs inline — fyne.Do from main thread deadlocks on
	// v2.7.1 (queue waits for main, main waits for queue). Inline
	// rebuild during a button click handler is supposedly risky
	// but Fyne handles widget tree replacement during event
	// dispatch fine in practice; the previously-feared crash was
	// theoretical, the deadlock is real and observed.
	off := widget.NewButton("Off", func() {
		wg.toggleWeapon(weaponID, false)
		wg.attrLevelSetter(attID, 0)
		wg.Refresh()
	})
	if current == 0 {
		off.Importance = widget.HighImportance
	}
	pills = append(pills, off)

	for i := 1; i <= maxLevel; i++ {
		level := i
		pill := widget.NewButton(fmt.Sprintf("%d", level), func() {
			wg.toggleWeapon(weaponID, true)
			wg.attrLevelSetter(attID, level)
			wg.Refresh()
		})
		if current == level {
			pill.Importance = widget.HighImportance
		}
		pills = append(pills, pill)
	}
	row := container.NewHBox(pills...)
	caption := widget.NewLabelWithStyle("paired: "+attID,
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true, Monospace: true})
	return container.NewVBox(row, caption)
}

func (wg *WeaponGrid) toggleWeapon(id string, on bool) {
	if on {
		wg.selected[id] = true
	} else {
		delete(wg.selected, id)
	}
	wg.TriggerChange()
}

func (wg *WeaponGrid) TriggerChange() {
	var parts []string
	for id := range wg.selected {
		parts = append(parts, id)
	}
	sort.Strings(parts)
	result := strings.Join(parts, "|")
	if wg.onChange != nil {
		wg.onChange(result)
	}
}

func (wg *WeaponGrid) Refresh() {
	wg.createUI()
}

func (wg *WeaponGrid) GetContent() fyne.CanvasObject {
	return wg.container
}

// HoverContainer wraps a widget and detects mouse enter/leave.
// Pairs MouseIn with MouseOut so the info panel's sticky/hover
// contract works: MouseIn pushes a transient hover into the panel,
// MouseOut reverts it to whatever the user last interacted with.
// Without the MouseOut half, the panel would freeze on the last
// hovered row and never go back to "what am I editing?".
type HoverContainer struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onHover func()
	onLeave func()
}

// NewHoverContainer constructs a hover-aware wrapper. onHover fires
// on MouseIn; onLeave fires on MouseOut. Either may be nil.
func NewHoverContainer(content fyne.CanvasObject, onHover func()) *HoverContainer {
	h := &HoverContainer{content: content, onHover: onHover}
	h.ExtendBaseWidget(h)
	return h
}

// SetOnLeave wires a MouseOut callback after construction — keeps
// the NewHoverContainer signature backward-compatible with older
// call sites that don't need the leave event.
func (h *HoverContainer) SetOnLeave(f func()) { h.onLeave = f }

func (h *HoverContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(h.content)
}

func (h *HoverContainer) MouseIn(*desktop.MouseEvent) {
	if h.onHover != nil {
		h.onHover()
	}
}
func (h *HoverContainer) MouseOut() {
	if h.onLeave != nil {
		h.onLeave()
	}
}
func (h *HoverContainer) MouseMoved(*desktop.MouseEvent) {}
