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
	container *fyne.Container
	selected  map[string]bool
	onChange  func(string)
	onHover   func(string, string)

	filter string
	search *widget.Entry
}

func NewWeaponGrid(initialStr string, onChange func(string), onHover func(string, string)) *WeaponGrid {
	wg := &WeaponGrid{
		selected: make(map[string]bool),
		onChange: onChange,
		onHover:  onHover,
	}
	wg.parseString(initialStr)
	wg.createUI()
	return wg
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

			// Wrap in HoverContainer
			hoverContainer := NewHoverContainer(check, func() {
				if wg.onHover != nil {
					wg.onHover(weaponID, w.Description)
				}
			})

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

// HoverContainer wraps a widget and detects mouse hover
type HoverContainer struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onHover func()
}

func NewHoverContainer(content fyne.CanvasObject, onHover func()) *HoverContainer {
	h := &HoverContainer{content: content, onHover: onHover}
	h.ExtendBaseWidget(h)
	return h
}

func (h *HoverContainer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(h.content)
}

func (h *HoverContainer) MouseIn(*desktop.MouseEvent) {
	if h.onHover != nil {
		h.onHover()
	}
}
func (h *HoverContainer) MouseOut()                      {}
func (h *HoverContainer) MouseMoved(*desktop.MouseEvent) {}
