package main

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type WeaponGrid struct {
	container   *fyne.Container
	selected    map[string]bool
	onChange    func(string)
	onHover     func(string, string)
	onUnhover   func()
	resolveIcon func(string) fyne.Resource

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
	// Group by Category
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
		wg.search.SetPlaceHolder("Filter Weapons...")
		wg.search.OnChanged = func(s string) {
			wg.filter = s
			wg.Refresh()
		}

		scroll := container.NewVScroll(content)
		mainLayout = container.NewBorder(wg.search, nil, nil, nil, scroll)
		wg.container = mainLayout
	}

	filterLower := strings.ToLower(wg.filter)

	for _, catName := range catOrder {
		weapons, ok := categories[catName]
		if !ok {
			continue
		}

		var visibleWeapons []WeaponDef
		for _, w := range weapons {
			if filterLower == "" ||
				strings.Contains(strings.ToLower(w.Name), filterLower) ||
				strings.Contains(strings.ToLower(w.ID), filterLower) {
				visibleWeapons = append(visibleWeapons, w)
			}
		}

		if len(visibleWeapons) == 0 {
			continue
		}

		header := widget.NewLabelWithStyle(catName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		content.Add(header)

		catGrid := container.NewGridWithColumns(2)

		for _, w := range visibleWeapons {
			weaponID := w.ID

			check := widget.NewCheck(w.Name, func(on bool) {
				wg.toggleWeapon(weaponID, on)
			})
			check.Checked = wg.selected[weaponID]

			// In-game icon sits immediately to the left of the check.
			// Replaces the old emoji prefix (💣, 🔫 etc. on weapon
			// names) with the real w_icon_*.png the game ships —
			// embedded at build time from assets/icons/weapons/. When
			// no art is available the row renders as a plain check.
			var row fyne.CanvasObject = check
			if wg.resolveIcon != nil {
				if res := wg.resolveIcon(weaponID); res != nil {
					iconW := widget.NewIcon(res)
					row = container.NewHBox(iconW, check)
				}
			}

			// Wrap in HoverContainer. Pair the enter event with a
			// leave event so the info panel's sticky context reverts
			// when the mouse moves off the row — otherwise the panel
			// would freeze on whatever weapon the mouse last grazed.
			hoverContainer := NewHoverContainer(row, func() {
				if wg.onHover != nil {
					wg.onHover(weaponID, w.Description)
				}
			})
			if wg.onUnhover != nil {
				hoverContainer.SetOnLeave(wg.onUnhover)
			}

			catGrid.Add(hoverContainer)
		}
		content.Add(catGrid)
		content.Add(widget.NewSeparator())
	}

	if wg.container != nil {
		wg.container.Refresh()
	}
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
	content  fyne.CanvasObject
	onHover  func()
	onLeave  func()
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
