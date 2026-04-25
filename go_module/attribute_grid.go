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
		// viewports and flex on wide ones. Regen + Supply get extra
		// structure: Regen rows are visually grouped by base resource
		// (Health / Armour / Block / Resource) into amount/rate/cap
		// triplets; Supply rows split into a 3-row matrix (DISP /
		// DROP / STIM) of 5 sub-types each. The grouping is purely
		// visual — each underlying attribute toggle still owns its
		// own widget so existing handlers stay unchanged.
		var catContent fyne.CanvasObject
		switch catName {
		case "Regen":
			catContent = ag.buildRegenGroupedView(visibleAttrs)
		case "Supply":
			catContent = ag.buildSupplyMatrixView(visibleAttrs)
		default:
			grid := container.NewGridWrap(fyne.NewSize(480, 46))
			for _, attr := range visibleAttrs {
				grid.Add(ag.createAttributeItem(attr))
			}
			catContent = grid
		}

		// Title with count suffix so the header carries quick context
		// even when the section is collapsed. "Weapons (42)" beats
		// "Weapons" for at-a-glance triage.
		title := fmt.Sprintf("%s (%d)", catName, len(visibleAttrs))
		item := widget.NewAccordionItem(title, catContent)
		if filterLower != "" || defaultOpen[catName] {
			item.Open = true
		}
		accordion.Append(item)
	}

	if len(accordion.Items) == 0 {
		// All categories filtered to zero — show an empty-state tile
		// instead of a silently-blank scroll. The filter Entry stays
		// visible at the top so the user can edit/clear it.
		hint := "No attributes match the current filter."
		if filterLower != "" {
			hint = fmt.Sprintf("No attributes match \"%s\".", ag.filter)
		}
		content.Add(NewEmptyStateTile("NO RESULTS", hint, "Clear filter", func() {
			if ag.search != nil {
				ag.search.SetText("")
			}
		}))
	} else {
		content.Add(accordion)
	}
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

// buildRegenGroupedView arranges the Regen category as four base-
// resource sub-sections (Health / Armour / Block / Resource), each
// with its (amount, rate, cap) triplet shown together. Anything that
// doesn't match a base resource (defensive — shouldn't happen with
// the canonical *_REGEN_* set) falls into a trailing "Other" row.
func (ag *AttributeGrid) buildRegenGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	// MBII's regen families. Order is health-first because that's what
	// authors care about most.
	groups := []struct {
		title  string
		prefix string
	}{
		{"Health regen", "MB_ATT_HEALTH_REGEN_"},
		{"Armour regen", "MB_ATT_ARMOUR_REGEN_"},
		{"Block regen", "MB_ATT_BLOCK_REGEN_"},
		{"Resource regen", "MB_ATT_RESOURCE_REGEN_"},
	}
	box := container.NewVBox()
	used := map[string]bool{}
	for _, g := range groups {
		var members []AttributeDef
		for _, a := range attrs {
			if strings.HasPrefix(a.ID, g.prefix) {
				members = append(members, a)
				used[a.ID] = true
			}
		}
		if len(members) == 0 {
			continue
		}
		sub := widget.NewLabelWithStyle(g.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		grid := container.NewGridWrap(fyne.NewSize(480, 46))
		for _, a := range members {
			grid.Add(ag.createAttributeItem(a))
		}
		box.Add(sub)
		box.Add(grid)
	}
	// Catch any *_REGEN_* outside the canonical four bases.
	var leftovers []AttributeDef
	for _, a := range attrs {
		if !used[a.ID] {
			leftovers = append(leftovers, a)
		}
	}
	if len(leftovers) > 0 {
		sub := widget.NewLabelWithStyle("Other regen", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true})
		grid := container.NewGridWrap(fyne.NewSize(480, 46))
		for _, a := range leftovers {
			grid.Add(ag.createAttributeItem(a))
		}
		box.Add(sub)
		box.Add(grid)
	}
	return box
}

// buildSupplyMatrixView arranges the Supply category by delivery
// mode (Dispenser / Drop / Stim) so the 5×3 matrix structure the
// agents identified reads as a clear set instead of fifteen flat
// rows. Other supply-bucket entries (BACTA, MEDI/AMMO_PACK,
// SUPPLYDROP itself, SPAWNER) tail at the end.
func (ag *AttributeGrid) buildSupplyMatrixView(attrs []AttributeDef) fyne.CanvasObject {
	groups := []struct {
		title  string
		prefix string
	}{
		{"Dispenser", "MB_ATT_DISP_"},
		{"Drop", "MB_ATT_DROP_"},
		{"Stim", "MB_ATT_STIM_"},
	}
	box := container.NewVBox()
	used := map[string]bool{}
	for _, g := range groups {
		var members []AttributeDef
		for _, a := range attrs {
			if strings.HasPrefix(a.ID, g.prefix) {
				members = append(members, a)
				used[a.ID] = true
			}
		}
		if len(members) == 0 {
			continue
		}
		sub := widget.NewLabelWithStyle(g.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		grid := container.NewGridWrap(fyne.NewSize(480, 46))
		for _, a := range members {
			grid.Add(ag.createAttributeItem(a))
		}
		box.Add(sub)
		box.Add(grid)
	}
	var leftovers []AttributeDef
	for _, a := range attrs {
		if !used[a.ID] {
			leftovers = append(leftovers, a)
		}
	}
	if len(leftovers) > 0 {
		sub := widget.NewLabelWithStyle("Other supply", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true})
		grid := container.NewGridWrap(fyne.NewSize(480, 46))
		for _, a := range leftovers {
			grid.Add(ag.createAttributeItem(a))
		}
		box.Add(sub)
		box.Add(grid)
	}
	return box
}

func (ag *AttributeGrid) GetContent() fyne.CanvasObject {
	return ag.container
}
