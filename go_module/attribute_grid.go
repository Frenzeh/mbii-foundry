package main

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// sectionAccent returns a deterministic, low-saturation accent color
// for an attribute sub-group's section tile. Hand-mapped for the
// well-known bucket names so the same family always reads the same
// color across rebuilds; falls back to a hash-derived hue for
// anything new (custom buckets added by future categorization).
//
// All colors are tuned for the dark theme — moderate value, lower
// saturation than the per-category WeaponDef accents. The section
// fill (alpha 20) renders as a tinted area background; the stroke
// (alpha 70) gives the section a soft 1px outline so adjacent
// sections stay distinct without screaming for attention.
func sectionAccent(bucket string) color.Color {
	switch bucket {
	// Weapons sub-buckets
	case "Pistols":
		return color.NRGBA{R: 110, G: 180, B: 220, A: 255} // teal-blue
	case "Rifles":
		return color.NRGBA{R: 110, G: 200, B: 130, A: 255} // green
	case "Heavy":
		return color.NRGBA{R: 220, G: 160, B: 90, A: 255} // amber
	case "Grenades":
		return color.NRGBA{R: 200, G: 120, B: 90, A: 255} // burnt orange
	case "Launchers":
		return color.NRGBA{R: 230, G: 100, B: 80, A: 255} // red
	case "Explosives":
		return color.NRGBA{R: 240, G: 130, B: 60, A: 255} // hot orange
	case "Melee":
		return color.NRGBA{R: 180, G: 130, B: 220, A: 255} // violet
	case "Specials":
		return color.NRGBA{R: 220, G: 200, B: 100, A: 255} // gold
	// Force sub-buckets
	case "Core":
		return color.NRGBA{R: 130, G: 170, B: 220, A: 255}
	case "Defensive":
		return color.NRGBA{R: 110, G: 200, B: 170, A: 255}
	case "Offensive":
		return color.NRGBA{R: 220, G: 110, B: 130, A: 255}
	case "Saber-gating", "Saber-side":
		return color.NRGBA{R: 200, G: 160, B: 230, A: 255}
	case "Tuning":
		return color.NRGBA{R: 160, G: 160, B: 170, A: 255}
	// Saber sub-buckets
	case "Damage", "Proficiency":
		return color.NRGBA{R: 220, G: 140, B: 100, A: 255}
	case "Combo":
		return color.NRGBA{R: 200, G: 110, B: 140, A: 255}
	case "Style unlocks":
		return color.NRGBA{R: 180, G: 170, B: 230, A: 255}
	// General sub-buckets
	case "Essentials":
		return color.NRGBA{R: 140, G: 200, B: 190, A: 255}
	case "Defense":
		return color.NRGBA{R: 130, G: 170, B: 220, A: 255}
	case "Movement":
		return color.NRGBA{R: 180, G: 220, B: 140, A: 255}
	case "Jetpack & Fuel":
		return color.NRGBA{R: 210, G: 180, B: 110, A: 255}
	case "Stamina":
		return color.NRGBA{R: 170, G: 200, B: 130, A: 255}
	case "Supply & deployables":
		return color.NRGBA{R: 200, G: 160, B: 110, A: 255}
	case "Utility":
		return color.NRGBA{R: 160, G: 180, B: 200, A: 255}
	// Advanced sub-buckets
	case "Movement tech":
		return color.NRGBA{R: 150, G: 210, B: 170, A: 255}
	case "Internals":
		return color.NRGBA{R: 170, G: 170, B: 180, A: 255}
	case "Niche force":
		return color.NRGBA{R: 200, G: 170, B: 220, A: 255}
	case "Melee tech":
		return color.NRGBA{R: 200, G: 130, B: 200, A: 255}
	case "Misc":
		return color.NRGBA{R: 160, G: 160, B: 160, A: 255}
	// Regen / Supply sub-buckets
	case "Health regen":
		return color.NRGBA{R: 220, G: 110, B: 130, A: 255}
	case "Armour regen":
		return color.NRGBA{R: 130, G: 170, B: 220, A: 255}
	case "Block regen":
		return color.NRGBA{R: 180, G: 170, B: 230, A: 255}
	case "Resource regen":
		return color.NRGBA{R: 220, G: 200, B: 100, A: 255}
	case "Dispenser":
		return color.NRGBA{R: 200, G: 160, B: 110, A: 255}
	case "Drop":
		return color.NRGBA{R: 220, G: 140, B: 100, A: 255}
	case "Stim":
		return color.NRGBA{R: 110, G: 200, B: 170, A: 255}
	case "Other", "Other resources", "Other regen", "Other supply":
		return color.NRGBA{R: 130, G: 130, B: 140, A: 255}
	// Resources umbrella sub-buckets
	case "Force pool":
		return color.NRGBA{R: 130, G: 170, B: 230, A: 255}
	case "Fuel":
		return color.NRGBA{R: 220, G: 180, B: 100, A: 255}
	case "Battery / Reserves":
		return color.NRGBA{R: 220, G: 220, B: 100, A: 255}
	}
	// Hash-fallback for unrecognized buckets — keeps the color stable
	// across rebuilds for any future sub-group name we haven't mapped.
	var h uint32 = 5381
	for _, r := range bucket {
		h = ((h << 5) + h) + uint32(r)
	}
	r := uint8(120 + h%80)
	g := uint8(120 + (h>>8)%80)
	b := uint8(160 + (h>>16)%70)
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

type AttributeGrid struct {
	container   *fyne.Container
	content     *fyne.Container
	values      map[string]int
	onChange    func(string)
	onHover     func(string, string)
	onUnhover   func()
	resolveIcon func(string) fyne.Resource // New callback
	// onClickInfo wires the (i) button on each row to the App's
	// showStickyContext path, so clicks pin the sidebar regardless
	// of hover-toggle state. Without this, (i) clicks went through
	// the hover dispatcher and were ignored when hover was OFF.
	onClickInfo func(string, string)

	// classScalarsBuilder, when set, returns the rendered "Class
	// Scalars" form (static apMultiplier / bpMultiplier / etc. entry
	// fields). Embedded into the Resources top-level category so
	// authors see all pool/scalar tweakers in one place. Optional —
	// the editor wires this; tests / standalone usage don't.
	classScalarsBuilder func() fyne.CanvasObject

	filter string
	search *widget.Entry
}

// SetOnUnhover wires a mouse-leave callback for hover-based preview
// revert. Symmetric with WeaponGrid.SetOnUnhover — both feed the
// info panel's sticky/hover model.
func (ag *AttributeGrid) SetOnUnhover(f func()) { ag.onUnhover = f }

// SetOnClickInfo wires the per-row (i) button to a sticky-context
// callback. The widget's OnInfoClick fires this on click — bypasses
// the hover toggle so users can pin the sidebar by clicking even
// when "hover swaps context" is OFF.
func (ag *AttributeGrid) SetOnClickInfo(f func(string, string)) { ag.onClickInfo = f }

// SetClassScalarsBuilder wires the Resources-section embedded form
// for the static class scalar fields (apMult, bpMult, csMult, asMult,
// forceRegen, speed). When set, the Resources category renders the
// builder's content as the first sub-tile so scalars sit alongside
// pool/regen attributes instead of floating in their own banner.
func (ag *AttributeGrid) SetClassScalarsBuilder(f func() fyne.CanvasObject) {
	ag.classScalarsBuilder = f
}

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
	// Group by Category. Skip everything that isn't actually an
	// attribute: EAS_* (class specials), HI_* (holdables), bare FP_*
	// (force-power enums distinct from MB_ATT_FP_*). Those rode along
	// in data/attributes.json historically because they share the
	// picker shape, but they have their own UI surfaces — leaking
	// them into the Attributes grid created the duplicate-row look
	// the user flagged (e.g. "Forcefield" appearing twice as
	// MB_ATT_FORCEFIELD and HI_SHIELD).
	categories := make(map[string][]AttributeDef)
	attributes := GetAttributes()
	for _, attr := range attributes {
		if !isAttributeProper(attr.ID) {
			continue
		}
		// Resources umbrella: re-bucket anything that controls a pool
		// or regen rate (force pool, fuel, stamina, battery, BP regen,
		// health regen, armor regen) so authors find pool tweakers in
		// one place. Original Category is preserved on the AttributeDef
		// itself; this only changes which top-level section the row
		// renders under.
		if isResourceAttribute(attr) {
			categories["Resources"] = append(categories["Resources"], attr)
			continue
		}
		categories[attr.Category] = append(categories[attr.Category], attr)
	}
	// Synthesize a Resources bucket entry even if every concrete attr
	// went elsewhere — the Class Scalars sub-tile alone is reason to
	// render the section.
	if ag.classScalarsBuilder != nil {
		if _, ok := categories["Resources"]; !ok {
			categories["Resources"] = nil
		}
	}

	// Category display order — informs the accordion row order.
	// Resources first (everything that tweaks pools/regen sits there,
	// including the Class Scalars sub-tile). Weapons / Force / Saber
	// next because those are where most edits happen. Advanced +
	// General go last; Advanced holds engine-tuning attrs rarely
	// touched.
	catOrder := []string{
		"Resources",
		"Weapons",
		"Force",
		"Saber",
		"Class Specific",
		"Supply",
		"Multipliers",
		"General",
		"Advanced",
	}
	// Note: "Regen" merged into Resources (regen rates are pool
	// tweakers — they belong with the rest of the resource section).

	// defaultOpen — categories that start expanded on first load.
	// Only the most-used ones; everything else opens on click so the
	// initial view stays calm and the user's eye lands on the things
	// they actually buy. Advanced especially stays closed — it's a
	// rare-edit backwater, not a primary section.
	defaultOpen := map[string]bool{
		"Resources":      true,
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
			// Resources is special: even if no MB_ATT_* attribute lives
			// in this build, the Class Scalars sub-tile is reason
			// enough to render the section. Skip the empty-skip *only*
			// for non-Resources categories so we don't lose the
			// scalars surface.
			if catName != "Resources" || ag.classScalarsBuilder == nil {
				continue
			}
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
		case "Resources":
			catContent = ag.buildResourcesView(visibleAttrs)
		case "Supply":
			catContent = ag.buildSupplyMatrixView(visibleAttrs)
		case "Force":
			catContent = ag.buildForceGroupedView(visibleAttrs)
		case "Saber":
			catContent = ag.buildSaberGroupedView(visibleAttrs)
		case "Weapons":
			catContent = ag.buildWeaponsGroupedView(visibleAttrs)
		case "Class Specific":
			catContent = ag.buildClassSpecificGroupedView(visibleAttrs)
		case "General":
			catContent = ag.buildGeneralGroupedView(visibleAttrs)
		case "Advanced":
			catContent = ag.buildAdvancedGroupedView(visibleAttrs)
		default:
			catContent = ag.buildAlphaSortedGrid(visibleAttrs)
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

	// Sticky-click route — the (i) button uses this so clicks pin
	// the sidebar even with the hover toggle off (the default).
	// Reuses ag.onClickInfo if wired by the editor; otherwise the
	// widget falls back to the hover dispatcher.
	if ag.onClickInfo != nil {
		w.OnInfoClick = ag.onClickInfo
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

// IsPointBuyMultiplier reports whether an attribute ID names a
// point-buy multiplier primitive (MB_ATT_*_MULTIPLIER). These are
// NOT loadout attributes — they're cost-side definitions that the
// custom-build (point-buy) editor wires into individual slots so
// authors can let a player buy custom multipliers. They belong in
// the Point Buy tab, not the Attributes grid. The Attributes grid
// filters them out via this helper.
func IsPointBuyMultiplier(id string) bool {
	return strings.HasPrefix(id, "MB_ATT_") && strings.HasSuffix(id, "_MULTIPLIER")
}

// isAttributeProper reports whether an ID is a real MB_ATT_* /
// MB_RES_* attribute and not one of the inventory-shaped enums
// (HI_*, EAS_*, bare FP_*) that sit alongside them in the data
// file. The Attributes grid only renders the proper ones; the
// others are picked from their own surfaces.
func isAttributeProper(id string) bool {
	if IsPointBuyMultiplier(id) {
		// Multipliers are point-buy primitives — surfaced in the Point
		// Buy tab (and the named-field Multipliers banner above the
		// grid), not as loadout toggles in the Attributes grid.
		return false
	}
	return strings.HasPrefix(id, "MB_ATT_") || strings.HasPrefix(id, "MB_RES_")
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

// isResourceAttribute reports whether an attribute belongs in the
// "Resources" umbrella section (anything that tweaks a pool size or
// regen rate — force pool, fuel, stamina, battery, BP regen, health
// regen, armor regen, etc.). Pulled out of its source Category so
// authors find pool tweakers in one place rather than hunting across
// Regen / Class Specific / Force / General.
func isResourceAttribute(a AttributeDef) bool {
	if a.Category == "Regen" {
		return true
	}
	id := a.ID
	// Pool / energy / battery / stamina caps (not the abilities that
	// consume them). FORCEFOCUS, FORCE_REGEN, etc.
	tokens := []string{
		"FUEL", "STAMINA", "BATTERY",
		"FP_BATTERY", "FORCEPOOL", "FORCE_POOL", "FORCE_REGEN", "FORCEFOCUS",
		"_REGEN_", "RESOURCE_REGEN", "BLOCK_REGEN",
		"HEAT_DUMP", "HEATDUMP",
	}
	for _, t := range tokens {
		if strings.Contains(id, t) {
			return true
		}
	}
	return false
}

// buildResourcesView renders the Resources umbrella category. Layout:
//
//   1. Class Scalars sub-tile (static apMult/bpMult/csMult/asMult/
//      forceRegen/speed entry fields) — only when the editor has
//      provided a builder. First because authors want the float
//      scalars visible alongside pool/regen attributes.
//   2. Sub-buckets for force pool / fuel / stamina / battery / BP /
//      health regen / armor regen / resource regen, each as its own
//      sectionTile via buildSectionTile.
//
// The result reads as a single "Resources" panel where every attribute
// or scalar that influences a pool or regen rate sits together,
// instead of being scattered across Regen / Force / Class Specific.
func (ag *AttributeGrid) buildResourcesView(attrs []AttributeDef) fyne.CanvasObject {
	box := container.NewVBox()
	if ag.classScalarsBuilder != nil {
		box.Add(ag.classScalarsBuilder())
	}
	// Sub-bucket classifier — keep the order aligned with how authors
	// usually scan a build (defensive pools first, then offense pools,
	// then niche reserves).
	type bucketDef struct {
		name    string
		matches func(AttributeDef) bool
	}
	buckets := []bucketDef{
		{"Health regen", func(a AttributeDef) bool { return strings.Contains(a.ID, "HEALTH_REGEN") }},
		{"Armour regen", func(a AttributeDef) bool { return strings.Contains(a.ID, "ARMOUR_REGEN") || strings.Contains(a.ID, "ARMOR_REGEN") }},
		{"Block regen", func(a AttributeDef) bool { return strings.Contains(a.ID, "BLOCK_REGEN") }},
		{"Force pool", func(a AttributeDef) bool {
			return strings.Contains(a.ID, "FORCEPOOL") || strings.Contains(a.ID, "FORCE_POOL") ||
				strings.Contains(a.ID, "FORCE_REGEN") || strings.Contains(a.ID, "FORCEFOCUS") ||
				a.ID == "MB_ATT_FP_BATTERY"
		}},
		{"Fuel", func(a AttributeDef) bool { return strings.Contains(a.ID, "FUEL") }},
		{"Stamina", func(a AttributeDef) bool { return strings.Contains(a.ID, "STAMINA") }},
		{"Battery / Reserves", func(a AttributeDef) bool {
			return strings.Contains(a.ID, "BATTERY") || strings.Contains(a.ID, "HEAT_DUMP") ||
				strings.Contains(a.ID, "HEATDUMP")
		}},
		{"Resource regen", func(a AttributeDef) bool { return strings.Contains(a.ID, "RESOURCE_REGEN") }},
	}

	used := map[string]bool{}
	for _, bk := range buckets {
		var members []AttributeDef
		for _, a := range attrs {
			if used[a.ID] {
				continue
			}
			if bk.matches(a) {
				members = append(members, a)
				used[a.ID] = true
			}
		}
		if len(members) == 0 {
			continue
		}
		sort.SliceStable(members, func(i, j int) bool {
			return attrDisplayName(members[i]) < attrDisplayName(members[j])
		})
		box.Add(ag.buildSectionTile(bk.name, members, false))
	}
	// Anything that's marked Resource but didn't fit a sub-bucket —
	// keep it visible under "Other resources" rather than dropping.
	var leftovers []AttributeDef
	for _, a := range attrs {
		if !used[a.ID] {
			leftovers = append(leftovers, a)
		}
	}
	if len(leftovers) > 0 {
		sort.SliceStable(leftovers, func(i, j int) bool {
			return attrDisplayName(leftovers[i]) < attrDisplayName(leftovers[j])
		})
		box.Add(ag.buildSectionTile("Other resources", leftovers, true))
	}
	return box
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
		box.Add(ag.buildSectionTile(g.title, members, false))
	}
	// Catch any *_REGEN_* outside the canonical four bases.
	var leftovers []AttributeDef
	for _, a := range attrs {
		if !used[a.ID] {
			leftovers = append(leftovers, a)
		}
	}
	if len(leftovers) > 0 {
		box.Add(ag.buildSectionTile("Other regen", leftovers, true))
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
		box.Add(ag.buildSectionTile(g.title, members, false))
	}
	var leftovers []AttributeDef
	for _, a := range attrs {
		if !used[a.ID] {
			leftovers = append(leftovers, a)
		}
	}
	if len(leftovers) > 0 {
		box.Add(ag.buildSectionTile("Other supply", leftovers, true))
	}
	return box
}

// buildAlphaSortedGrid renders attributes in a flat grid sorted by
// the same display name the rows show. Used by every category whose
// shape isn't already sub-grouped explicitly. Without this, Fyne
// emits rows in struct-init order which on-screen reads as random.
func (ag *AttributeGrid) buildAlphaSortedGrid(attrs []AttributeDef) fyne.CanvasObject {
	sorted := make([]AttributeDef, len(attrs))
	copy(sorted, attrs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return attrDisplayName(sorted[i]) < attrDisplayName(sorted[j])
	})
	grid := container.NewGridWrap(fyne.NewSize(340, 60))
	for _, a := range sorted {
		grid.Add(ag.createAttributeItem(a))
	}
	return grid
}

// attrDisplayName returns the same name the AttributeToggleWidget
// shows on its primary label — display name when set, prettied enum
// otherwise. Used by group sorters so on-screen order matches what
// the user reads.
func attrDisplayName(a AttributeDef) string {
	if a.Name != "" && a.Name != a.ID && !strings.HasPrefix(a.Name, "MB_ATT_") {
		return strings.ToLower(a.Name)
	}
	return strings.ToLower(prettyAttributeName(a.ID))
}

// buildSubGroupedView is the shared engine for the Force / Saber /
// Weapons / Class Specific sub-accordions. Buckets each attribute
// via classify(); renders each non-empty bucket alphabetized under
// a bold header; tails leftovers as "Other".
//
// Visual hierarchy is *section-first*: each bucket gets its own
// TilePanel (visible fill + stroke) acting as the area background,
// with the attribute rows inside rendered nearly flat — only a thin
// hairline accent + barely-visible fill, so the eye reads "rows in a
// section" rather than "individual cards." This is the inverse of
// the older render where every attribute was a chunky card and the
// section header was a bare text label, which made the screen feel
// like a wall of identical tiles regardless of grouping.
func (ag *AttributeGrid) buildSubGroupedView(attrs []AttributeDef, bucketOrder []string, classify func(AttributeDef) string) fyne.CanvasObject {
	groups := map[string][]AttributeDef{}
	used := map[string]bool{}
	for _, a := range attrs {
		bucket := classify(a)
		if bucket != "" {
			groups[bucket] = append(groups[bucket], a)
			used[a.ID] = true
		}
	}
	box := container.NewVBox()
	for _, name := range bucketOrder {
		members, ok := groups[name]
		if !ok || len(members) == 0 {
			continue
		}
		sort.SliceStable(members, func(i, j int) bool {
			return attrDisplayName(members[i]) < attrDisplayName(members[j])
		})
		box.Add(ag.buildSectionTile(name, members, false))
	}
	var leftovers []AttributeDef
	for _, a := range attrs {
		if !used[a.ID] {
			leftovers = append(leftovers, a)
		}
	}
	if len(leftovers) > 0 {
		sort.SliceStable(leftovers, func(i, j int) bool {
			return attrDisplayName(leftovers[i]) < attrDisplayName(leftovers[j])
		})
		box.Add(ag.buildSectionTile("Other", leftovers, true))
	}
	return box
}

// buildSectionTile wraps one sub-group (header + grid of rows) into a
// section-level TilePanel. The section's fill/stroke is the *primary*
// visual marker for the group; per-attribute rows inside use minimal
// chrome so the section reads as an "area" rather than a row of
// identical individual cards. Accent color is derived deterministically
// from the bucket name so re-renders are visually stable.
func (ag *AttributeGrid) buildSectionTile(name string, members []AttributeDef, isOther bool) fyne.CanvasObject {
	headerStyle := fyne.TextStyle{Bold: true}
	if isOther {
		headerStyle = fyne.TextStyle{Bold: true, Italic: true}
	}
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("%s  ·  %d", name, len(members)),
		fyne.TextAlignLeading, headerStyle)
	grid := container.NewGridWrap(fyne.NewSize(340, 60))
	for _, a := range members {
		grid.Add(ag.createAttributeItem(a))
	}
	body := container.NewVBox(header, grid)
	return NewTilePanel(body, TileOpts{
		AccentColor: sectionAccent(name),
		FillAlpha:   20,
		StrokeAlpha: 70,
		Padded:      true,
	})
}

func (ag *AttributeGrid) buildForceGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	core := map[string]bool{
		"MB_ATT_FP_PUSH": true, "MB_ATT_FP_PULL": true, "MB_ATT_FP_LEVITATION": true,
		"MB_ATT_FP_SPEED": true, "MB_ATT_FP_SEE": true, "MB_ATT_FP_TELEPATHY": true,
	}
	defensive := map[string]bool{
		"MB_ATT_FP_HEAL": true, "MB_ATT_FP_PROTECT": true, "MB_ATT_FP_ABSORB": true,
		"MB_ATT_FP_TEAM_HEAL": true, "MB_ATT_FP_TEAM_FORCE": true,
	}
	offensive := map[string]bool{
		"MB_ATT_FP_GRIP": true, "MB_ATT_FP_LIGHTNING": true, "MB_ATT_FP_RAGE": true,
		"MB_ATT_FP_DRAIN": true, "MB_ATT_FP_BLIND": true,
		"MB_ATT_FP_DESTRUCTION": true, "MB_ATT_FP_DEADLYSIGHT": true,
		"MB_ATT_FP_STASIS": true,
	}
	saberGate := map[string]bool{
		"MB_ATT_FP_SABER_OFFENSE": true, "MB_ATT_FP_SABER_DEFENSE": true,
		"MB_ATT_FP_SABERTHROW": true,
	}
	classify := func(a AttributeDef) string {
		switch {
		case core[a.ID]:
			return "Core"
		case defensive[a.ID]:
			return "Defensive"
		case offensive[a.ID]:
			return "Offensive"
		case saberGate[a.ID]:
			return "Saber-gating"
		case a.ID == "MB_ATT_FP_MULTIPLIER":
			return "Tuning"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Core", "Defensive", "Offensive", "Saber-gating", "Tuning"},
		classify)
}

func (ag *AttributeGrid) buildSaberGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	damage := map[string]bool{
		"MB_ATT_SABER_DAMAGE":        true,
		"MB_ATT_SABERTHROW_DAMAGE":   true,
		"MB_ATT_SABERSPECIAL_DAMAGE": true,
		"MB_ATT_SABER_MAXCHAIN":      true,
	}
	// "Proficiency" = the 1-3 mastery levels per style + Doubles +
	// Mastery. These are damage/BP/AP-shaping ranks, not training in
	// the EX-combo sense.
	proficiency := map[string]bool{
		"MB_ATT_SABER_FAST":    true,
		"MB_ATT_SABER_MEDIUM":  true,
		"MB_ATT_SABER_STRONG":  true,
		"MB_ATT_SABER_DOUBLES": true,
		"MB_ATT_SABER_MASTERY": true,
	}
	// "Combo" = the EX-saber-training and No-saber-training toggles.
	combo := map[string]bool{
		"MB_ATT_SABER_COMBO":      true,
		"MB_ATT_SABER_COMBO_NONE": true,
	}
	classify := func(a AttributeDef) string {
		switch {
		case damage[a.ID]:
			return "Damage"
		case proficiency[a.ID]:
			return "Proficiency"
		case combo[a.ID]:
			return "Combo"
		case strings.HasPrefix(a.ID, "MB_ATT_SS_"):
			return "Style unlocks"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Damage", "Proficiency", "Combo", "Style unlocks"},
		classify)
}

func (ag *AttributeGrid) buildWeaponsGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	pistols := map[string]bool{
		"MB_ATT_PISTOL": true, "MB_ATT_HEAVY_PISTOL": true,
		"MB_ATT_BRYAR_OLD": true, "MB_ATT_CR2": true,
		"MB_ATT_CLONE_PISTOL": true, "MB_ATT_MANDO_PISTOL": true,
		"MB_ATT_IMP_PISTOL": true,
	}
	rifles := map[string]bool{
		"MB_ATT_BLASTER": true, "MB_ATT_A280": true, "MB_ATT_DLT20A": true,
		"MB_ATT_DLT19": true, "MB_ATT_E_22": true, "MB_ATT_EE3": true,
		"MB_ATT_EE4": true, "MB_ATT_T21": true, "MB_ATT_DC_CARBINE": true,
		"MB_ATT_AMBAN": true, "MB_ATT_CLONERIFLE": true,
		"MB_ATT_PROJECTILE_RIFLE": true, "MB_ATT_WESTARM5": true,
	}
	heavy := map[string]bool{
		"MB_ATT_DISRUPTOR": true, "MB_ATT_BOWCASTER": true,
		"MB_ATT_TRAD_BOWCASTER": true, "MB_ATT_REPEATER": true,
		"MB_ATT_FLECHETTE": true, "MB_ATT_DEMP2": true,
		"MB_ATT_IONRIFLE": true, "MB_ATT_MINIGUN": true,
		"MB_ATT_SHOTGUN": true, "MB_ATT_CONCUSSION": true,
	}
	grenades := map[string]bool{
		"MB_ATT_THERMAL": true, "MB_ATT_THERMALS": true,
		"MB_ATT_FRAGS": true, "MB_ATT_PULSE_GRENADES": true,
		"MB_ATT_FIRE_GRENADES": true, "MB_ATT_CRYOBAN_GRENADES": true,
		"MB_ATT_MICRO_GRENADES": true, "MB_ATT_SONIC_DETONATOR": true,
		"MB_ATT_BASE_TD": true, "MB_ATT_REPEATER_NADES": true,
		"MB_ATT_FLECHETTE_NADES": true, "MB_ATT_WHISTLINGBIRD": true,
	}
	launchers := map[string]bool{
		"MB_ATT_ROCKET": true, "MB_ATT_ROCKET_LAUNCHER": true,
		"MB_ATT_PLX1": true, "MB_ATT_UGL": true,
		"MB_ATT_UGL_BURST": true, "MB_ATT_UGL_IMPACT": true,
		"MB_ATT_UGL_BURST_MIXED": true, "MB_ATT_MGL": true,
		"MB_ATT_MGL_IMPACT": true, "MB_ATT_MGL_BURST": true,
	}
	explosives := map[string]bool{
		"MB_ATT_DET_PACK": true, "MB_ATT_STICKY_BOMBS": true,
		"MB_ATT_TRIP_MINES": true, "MB_ATT_REMOTE_DETONATE": true,
	}
	melee := map[string]bool{
		"MB_ATT_KNIFE": true, "MB_ATT_ELECTRO_STAFF": true,
		"MB_ATT_STUN_BATON": true, "MB_ATT_SWORD": true,
	}
	specials := map[string]bool{
		"MB_ATT_DRONE": true, "MB_ATT_BESKAR": true,
		"MB_ATT_TRACKING_DART": true, "MB_ATT_POISON_DART": true,
		"MB_ATT_QUICKDRAW": true, "MB_ATT_QUICKTHROW": true,
		"MB_ATT_THROWER": true, "MB_ATT_THROWER_LIGHTNING": true,
		"MB_ATT_THROWER_ICE": true, "MB_ATT_THROWER_PLASMA": true,
		"MB_ATT_THROWER_FLAME": true, "MB_ATT_THROWER_POISON": true,
		"MB_ATT_FLAMETHROWER": true, "MB_ATT_WPFLAMETHROWER": true,
	}
	classify := func(a AttributeDef) string {
		switch {
		case pistols[a.ID]:
			return "Pistols"
		case rifles[a.ID]:
			return "Rifles"
		case heavy[a.ID]:
			return "Heavy"
		case grenades[a.ID]:
			return "Grenades"
		case launchers[a.ID]:
			return "Launchers"
		case explosives[a.ID]:
			return "Explosives"
		case melee[a.ID]:
			return "Melee"
		case specials[a.ID]:
			return "Specials"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Pistols", "Rifles", "Heavy", "Grenades", "Launchers", "Explosives", "Melee", "Specials"},
		classify)
}

// buildGeneralGroupedView splits the General bucket into more
// meaningful sub-sections so the most-used attributes (resource caps,
// armor, regen-summary, defense gates) sit at the top in Essentials,
// followed by Defense / Movement / Jetpack & Fuel / Stamina / Saber-
// side passives / Supply / Misc. Force Block / Force Focus / Force
// Attunement live here despite being FP_ in flavor because they aren't
// MB_ATT_FP_ — they're flat caps the user wanted promoted.
func (ag *AttributeGrid) buildGeneralGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	essentials := map[string]bool{
		"MB_ATT_HEALTH":      true,
		"MB_ATT_ARMOUR":      true,
		"MB_ATT_AMMO":        true,
		"MB_ATT_POWER":       true,
		"MB_ATT_REGEN":       true,
		"MB_ATT_HEALING":     true,
		"MB_ATT_BASESPEED":   true,
		"MB_ATT_FORCEBLOCK":  true,
		"MB_ATT_FORCEFOCUS":  true,
		"MB_ATT_FORCEATTUNE": true,
		"MB_ATT_RESPAWNS":    true,
	}
	defense := map[string]bool{
		"MB_ATT_DURABILITY":       true,
		"MB_ATT_BLAST_ARMOUR":     true,
		"MB_ATT_MAGNETIC_PLATING": true,
		"MB_ATT_CORTOSIS":         true,
		"MB_ATT_GUN_DEFENSE":      true,
		"MB_ATT_DEFLECT":          true,
		"MB_ATT_SHIELD_RECHARGE":  true,
		"MB_ATT_DODGE":            true,
		"MB_ATT_KNOCKDOWN_ROLL":   true,
	}
	movement := map[string]bool{
		"MB_ATT_DEXTERITY": true,
		"MB_ATT_ACROBACY":  true,
		"MB_ATT_DASH":      true,
		"MB_ATT_DASH_JUMP": true,
		"MB_ATT_BACKSTAB":  true,
		"MB_ATT_STEALTH":   true,
		"MB_ATT_CLOAK":     true,
	}
	jetpack := map[string]bool{
		"MB_ATT_JETPACK":      true,
		"MB_ATT_FUEL":         true,
		"MB_ATT_FUELREGEN":    true,
		"MB_ATT_FLAMETHROWER": true,
	}
	stamina := map[string]bool{
		"MB_ATT_STAMINA":   true,
		"MB_ATT_TURN_RATE": true,
	}
	saberSide := map[string]bool{
		"MB_ATT_GETUPS":   true,
		"MB_ATT_FLIPKICK": true,
	}
	supply := map[string]bool{
		"MB_ATT_MEDI_PACK":     true,
		"MB_ATT_AMMO_PACK":     true,
		"MB_ATT_STIMPACK":      true,
		"MB_ATT_BACTA":         true,
		"MB_ATT_BACTA_BIG":     true,
		"MB_ATT_SUPPLYDROP":    true,
		"MB_ATT_USE_DISTANCE":  true,
		"MB_ATT_SPAWNER":       true,
		"MB_ATT_FORCEFIELD":    true,
		"MB_ATT_EWEB":          true,
		"MB_ATT_SENTRY":        true,
		"MB_ATT_LASERCOVER":    true,
	}
	misc := map[string]bool{
		"MB_ATT_RADAR":          true,
		"MB_ATT_ZOOM":           true,
		"MB_ATT_QUICKDRAW":      true,
		"MB_ATT_QUICKTHROW":     true,
		"MB_ATT_GRAPPLE_HOOK":   true,
		"MB_ATT_LIGHTS_BEACON":  true,
		"MB_ATT_TRACKING_DART":  true,
		"MB_ATT_POISON_DART":    true,
	}
	classify := func(a AttributeDef) string {
		switch {
		case essentials[a.ID]:
			return "Essentials"
		case defense[a.ID]:
			return "Defense"
		case movement[a.ID]:
			return "Movement"
		case jetpack[a.ID]:
			return "Jetpack & Fuel"
		case stamina[a.ID]:
			return "Stamina"
		case saberSide[a.ID]:
			return "Saber-side"
		case supply[a.ID]:
			return "Supply & deployables"
		case misc[a.ID]:
			return "Utility"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{
			"Essentials",
			"Defense",
			"Movement",
			"Jetpack & Fuel",
			"Stamina",
			"Saber-side",
			"Supply & deployables",
			"Utility",
		},
		classify)
}

// buildAdvancedGroupedView buckets the Advanced engine-tuning bag
// into Movement tech / Internals / Niche force / Misc taunts so the
// section reads at a glance instead of as one alphabetical scroll.
func (ag *AttributeGrid) buildAdvancedGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	movement := map[string]bool{
		"MB_ATT_BUNNY_HOP":   true,
		"MB_ATT_FLOAT_HOP":   true,
		"MB_ATT_GRAPPLE_HOP": true,
	}
	internals := map[string]bool{
		"MB_ATT_INAIR_FORCE_REGEN": true,
		"MB_ATT_TURN_RATE":         true,
		"MB_ATT_USE_DISTANCE":      true,
		"MB_ATT_KNOCKDOWN_ROLL":    true,
		"MB_ATT_TRACKING_BEACON":   true,
		"MB_ATT_SHIELD_RECHARGE2":  true,
		"MB_ATT_WRIST_AMMO":        true,
	}
	nicheForce := map[string]bool{
		"MB_ATT_FP_MIRALUKA": true,
		"MB_ATT_FP_REPULSE":  true,
	}
	melee := map[string]bool{
		"MB_ATT_GUNBASH":  true,
		"MB_ATT_FLIPKICK": true,
	}
	classify := func(a AttributeDef) string {
		switch {
		case movement[a.ID]:
			return "Movement tech"
		case internals[a.ID]:
			return "Internals"
		case nicheForce[a.ID]:
			return "Niche force"
		case melee[a.ID]:
			return "Melee tech"
		case a.ID == "MB_ATT_ROSHTAUNT" || a.ID == "MB_ATT_ANTI_MT":
			return "Misc"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Movement tech", "Internals", "Niche force", "Melee tech", "Misc"},
		classify)
}

func (ag *AttributeGrid) buildClassSpecificGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	classify := func(a AttributeDef) string {
		switch {
		case strings.HasPrefix(a.ID, "MB_ATT_WOOKIE"):
			return "Wookiee"
		case strings.HasPrefix(a.ID, "MB_ATT_DEKA"):
			return "Droideka"
		case strings.HasPrefix(a.ID, "MB_ATT_CLONE"),
			strings.HasPrefix(a.ID, "MB_ATT_ARC_RIFLE_"),
			a.ID == "MB_ATT_CCTRAINING",
			a.ID == "MB_ATT_ET_CCTRAINING":
			return "Clone / ARC / ET"
		case strings.HasPrefix(a.ID, "MB_ATT_MANDO_"):
			return "Mandalorian"
		case strings.HasPrefix(a.ID, "MB_ATT_IMP_"):
			return "Imperial"
		case a.ID == "MB_ATT_STRONGBLOBS",
			a.ID == "MB_ATT_HULL_STRENGTH",
			a.ID == "MB_ATT_ASSEMBLE",
			a.ID == "MB_ATT_RALLY",
			a.ID == "MB_ATT_WRIST_AMMO",
			a.ID == "MB_ATT_WRISTLASER":
			return "Misc class kit"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Wookiee", "Droideka", "Clone / ARC / ET", "Mandalorian", "Imperial", "Misc class kit"},
		classify)
}

func (ag *AttributeGrid) GetContent() fyne.CanvasObject {
	return ag.container
}
