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
		case "Force":
			catContent = ag.buildForceGroupedView(visibleAttrs)
		case "Saber":
			catContent = ag.buildSaberGroupedView(visibleAttrs)
		case "Weapons":
			catContent = ag.buildWeaponsGroupedView(visibleAttrs)
		case "Class Specific":
			catContent = ag.buildClassSpecificGroupedView(visibleAttrs)
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

// isAttributeProper reports whether an ID is a real MB_ATT_* /
// MB_RES_* attribute and not one of the inventory-shaped enums
// (HI_*, EAS_*, bare FP_*) that sit alongside them in the data
// file. The Attributes grid only renders the proper ones; the
// others are picked from their own surfaces.
func isAttributeProper(id string) bool {
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
		grid := container.NewGridWrap(fyne.NewSize(480, 64))
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
		grid := container.NewGridWrap(fyne.NewSize(480, 64))
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
		grid := container.NewGridWrap(fyne.NewSize(480, 64))
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
		grid := container.NewGridWrap(fyne.NewSize(480, 64))
		for _, a := range leftovers {
			grid.Add(ag.createAttributeItem(a))
		}
		box.Add(sub)
		box.Add(grid)
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
	grid := container.NewGridWrap(fyne.NewSize(480, 64))
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
		sub := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		grid := container.NewGridWrap(fyne.NewSize(480, 64))
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
		sort.SliceStable(leftovers, func(i, j int) bool {
			return attrDisplayName(leftovers[i]) < attrDisplayName(leftovers[j])
		})
		sub := widget.NewLabelWithStyle("Other", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Italic: true})
		grid := container.NewGridWrap(fyne.NewSize(480, 64))
		for _, a := range leftovers {
			grid.Add(ag.createAttributeItem(a))
		}
		box.Add(sub)
		box.Add(grid)
	}
	return box
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
		"MB_ATT_SABER_DAMAGE": true, "MB_ATT_SABERTHROW_DAMAGE": true,
		"MB_ATT_SABERSPECIAL_DAMAGE": true, "MB_ATT_SABER_MAXCHAIN": true,
	}
	training := map[string]bool{
		"MB_ATT_SABER_FAST": true, "MB_ATT_SABER_MEDIUM": true,
		"MB_ATT_SABER_STRONG": true, "MB_ATT_SABER_DOUBLES": true,
		"MB_ATT_SABER_MASTERY": true, "MB_ATT_SABER_COMBO": true,
		"MB_ATT_SABER_COMBO_NONE": true,
	}
	classify := func(a AttributeDef) string {
		switch {
		case damage[a.ID]:
			return "Damage"
		case training[a.ID]:
			return "Style training"
		case strings.HasPrefix(a.ID, "MB_ATT_SS_"):
			return "Style unlocks"
		case a.ID == "MB_ATT_SABER":
			return "Saber"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Saber", "Damage", "Style training", "Style unlocks"},
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
