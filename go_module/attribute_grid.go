package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type AttributeGrid struct {
	container   *fyne.Container
	content     *fyne.Container
	values      map[string]int
	onChange    func(string)
	onHover     func(string, string)
	onUnhover   func()
	resolveIcon func(string) fyne.Resource // New callback

	filter string
	search *widget.Entry
}

// SetOnUnhover wires a mouse-leave callback for hover-based preview
// revert. Symmetric with WeaponGrid.SetOnUnhover — both feed the
// info panel's sticky/hover model.
func (ag *AttributeGrid) SetOnUnhover(f func()) { ag.onUnhover = f }

func NewAttributeGrid(initialStr string, onChange func(string), onHover func(string, string), resolveIcon func(string) fyne.Resource) *AttributeGrid {
	InitDefinitions() // Ensure docs are loaded
	ag := &AttributeGrid{
		values:      parseAttributesString(initialStr),
		onChange:    onChange,
		onHover:     onHover,
		resolveIcon: resolveIcon,
	}
	ag.createUI()
	return ag
}

func (ag *AttributeGrid) createUI() {
	// Group by Category
	categories := make(map[string][]AttributeDef)
	attributes := GetAttributes()
	for _, attr := range attributes {
		categories[attr.Category] = append(categories[attr.Category], attr)
	}

	// Category display order — informs the accordion row order.
	// Mirrors the buckets categorizeAttribute() assigns. Weapons /
	// Force / Saber up top because those are where most edits happen.
	// Advanced + General go last; Advanced holds engine-tuning attrs
	// that are rarely touched. Multipliers sits near the end alongside
	// Regen because those are "tweak the defaults" buckets, not "pick
	// what the class does" buckets.
	catOrder := []string{
		"Weapons",
		"Force",
		"Saber",
		"Class Specific",
		"Supply",
		"Regen",
		"Multipliers",
		"General",
		"Advanced",
	}

	// defaultOpen — categories that start expanded on first load.
	// Only the most-used ones; everything else opens on click so the
	// initial view stays calm and the user's eye lands on the things
	// they actually buy. Advanced especially stays closed — it's a
	// rare-edit backwater, not a primary section.
	defaultOpen := map[string]bool{
		"Weapons":        true,
		"Force":          true,
		"Saber":          true,
		"Class Specific": true,
	}

	// Re-use existing container if possible, or create new
	var content *fyne.Container

	if ag.container != nil {
		content = ag.content
		content.Objects = nil // Clear existing grid items
	} else {
		content = container.NewVBox()
		ag.content = content

		ag.search = NewInputEntry()
		ag.search.SetPlaceHolder("Filter Attributes...")
		ag.search.OnChanged = func(s string) {
			ag.filter = s
			ag.Refresh() // Rerender grid
		}

		scroll := container.NewVScroll(content)
		ag.container = container.NewBorder(ag.search, nil, nil, nil, scroll)
	}

	filterLower := strings.ToLower(ag.filter)

	// Accordion with MultiOpen so authors can fan out several
	// categories at once. When the user is filtering, expand every
	// section that has matches so they don't have to click through
	// the rows — the filter itself is the signal of intent.
	accordion := widget.NewAccordion()
	accordion.MultiOpen = true

	for _, catName := range catOrder {
		attrs, ok := categories[catName]
		if !ok {
			continue
		}

		// Filter attributes for this category
		var visibleAttrs []AttributeDef
		for _, attr := range attrs {
			if filterLower == "" ||
				strings.Contains(strings.ToLower(attr.Name), filterLower) ||
				strings.Contains(strings.ToLower(attr.ID), filterLower) {
				visibleAttrs = append(visibleAttrs, attr)
			}
		}

		if len(visibleAttrs) == 0 {
			continue
		}

		// Grid for this category — GridWrap so rows stack on narrow
		// viewports and flex on wide ones.
		catGrid := container.NewGridWrap(fyne.NewSize(480, 46))
		for _, attr := range visibleAttrs {
			catGrid.Add(ag.createAttributeItem(attr))
		}

		// Title with count suffix so the header carries quick context
		// even when the section is collapsed. "Weapons (42)" beats
		// "Weapons" for at-a-glance triage.
		title := fmt.Sprintf("%s (%d)", catName, len(visibleAttrs))
		item := widget.NewAccordionItem(title, catGrid)
		if filterLower != "" || defaultOpen[catName] {
			item.Open = true
		}
		accordion.Append(item)
	}

	content.Add(accordion)
}

func (ag *AttributeGrid) createAttributeItem(attr AttributeDef) fyne.CanvasObject {
	currentVal := ag.values[attr.ID]

	// Resolve Icon
	var icon fyne.Resource
	if ag.resolveIcon != nil {
		icon = ag.resolveIcon(attr.ID)
	}

	w := NewAttributeToggleWidget(attr, currentVal, func(newVal int) {
		ag.updateValue(attr.ID, newVal)
	}, ag.onHover, icon)

	// Mouse-leave → revert the info panel to whatever the user last
	// interacted with. Without this, the panel freezes on the last-
	// hovered attribute even after the mouse moves off.
	if ag.onUnhover != nil {
		w.SetOnInfoLeave(ag.onUnhover)
	}

	return w
}

func (ag *AttributeGrid) updateValue(id string, val int) {
	if val == 0 {
		delete(ag.values, id)
	} else {
		ag.values[id] = val
	}
	ag.TriggerChange()
}

func (ag *AttributeGrid) TriggerChange() {
	// Reconstruct string
	var keys []string
	for k := range ag.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, id := range keys {
		val := ag.values[id]
		parts = append(parts, fmt.Sprintf("%s,%d", id, val))
	}
	result := strings.Join(parts, "|")
	if ag.onChange != nil {
		ag.onChange(result)
	}
}

func parseAttributesString(s string) map[string]int {
	res := make(map[string]int)
	if s == "" {
		return res
	}

	// Format: MB_ATT_X,1|MB_ATT_Y,2
	parts := strings.Split(s, "|")
	for _, part := range parts {
		kv := strings.Split(part, ",")
		if len(kv) == 2 {
			val, _ := strconv.Atoi(kv[1])
			res[strings.TrimSpace(kv[0])] = val
		}
	}
	return res
}

func (ag *AttributeGrid) Refresh() {
	ag.createUI()
	if ag.container != nil {
		ag.container.Refresh()
	}
}

func (ag *AttributeGrid) GetContent() fyne.CanvasObject {
	return ag.container
}
