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
// for an attribute sub-group's section tile.
func sectionAccent(bucket string) color.Color {
	switch bucket {
	// Weapons & Explosives
	case "Pistols & Blasters":
		return color.NRGBA{R: 110, G: 180, B: 220, A: 255} // teal-blue
	case "Heavy Rifles & Cannons":
		return color.NRGBA{R: 110, G: 200, B: 130, A: 255} // green
	case "Launchers & Ordnance Attachments":
		return color.NRGBA{R: 230, G: 110, B: 80, A: 255} // red-orange
	case "Grenades & Mines":
		return color.NRGBA{R: 240, G: 140, B: 60, A: 255} // hot amber

	// Physicals, Agility & Defense
	case "Movement, Agility & Mobility":
		return color.NRGBA{R: 140, G: 210, B: 170, A: 255} // mint
	case "Melee & Hand-to-Hand":
		return color.NRGBA{R: 200, G: 130, B: 210, A: 255} // purple
	case "Armor Tech & Damage Reduction":
		return color.NRGBA{R: 120, G: 170, B: 230, A: 255} // steel blue
	case "Health, Armor & Regen Pools":
		return color.NRGBA{R: 220, G: 110, B: 130, A: 255} // coral
	case "Combat Multipliers & Tuning":
		return color.NRGBA{R: 180, G: 150, B: 220, A: 255} // lavender

	// Force Powers
	case "Core Powers":
		return color.NRGBA{R: 100, G: 190, B: 240, A: 255} // sky blue
	case "Light & Defensive":
		return color.NRGBA{R: 110, G: 220, B: 180, A: 255} // emerald
	case "Dark & Offensive":
		return color.NRGBA{R: 235, G: 90, B: 110, A: 255} // crimson
	case "Force Pool, Focus & Tuning":
		return color.NRGBA{R: 190, G: 150, B: 230, A: 255} // violet

	// Lightsaber Mastery
	case "Style Unlocks & Forms":
		return color.NRGBA{R: 230, G: 150, B: 90, A: 255} // amber
	case "Style Mastery & Proficiency":
		return color.NRGBA{R: 220, G: 110, B: 130, A: 255} // ruby
	case "Damage, Chains & Combos":
		return color.NRGBA{R: 210, G: 130, B: 170, A: 255} // rose

	// Class & Droid Tech
	case "Super Battle Droid (SBD)":
		return color.NRGBA{R: 200, G: 170, B: 110, A: 255} // brass
	case "Droideka (Deka)":
		return color.NRGBA{R: 210, G: 130, B: 90, A: 255} // bronze
	case "Clone Trooper & ARC":
		return color.NRGBA{R: 110, G: 180, B: 230, A: 255} // clone blue
	case "Mandalorian & Bounty Hunter":
		return color.NRGBA{R: 190, G: 180, B: 110, A: 255} // mando gold
	case "Hero, Commander & Support":
		return color.NRGBA{R: 130, G: 200, B: 140, A: 255} // olive
	case "Wookiee":
		return color.NRGBA{R: 180, G: 130, B: 90, A: 255} // brown

	// Supplies & Tactical Gear
	case "Field Dispensers & Drops":
		return color.NRGBA{R: 210, G: 170, B: 110, A: 255} // buff
	case "Medical & Portable Gear":
		return color.NRGBA{R: 110, G: 210, B: 170, A: 255} // sea green
	case "Tactical Utility & Infiltration":
		return color.NRGBA{R: 150, G: 170, B: 190, A: 255} // slate
	}

	// Stable hash fallback
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
	resolveIcon func(string) fyne.Resource
	onClickInfo func(string, string)

	classScalarsBuilder func() fyne.CanvasObject

	filter string
	search *widget.Entry
}

func (ag *AttributeGrid) SetOnUnhover(f func()) { ag.onUnhover = f }

func (ag *AttributeGrid) SetOnClickInfo(f func(string, string)) { ag.onClickInfo = f }

func (ag *AttributeGrid) SetClassScalarsBuilder(f func() fyne.CanvasObject) {
	ag.classScalarsBuilder = f
}

func NewAttributeGrid(initialStr string, onChange func(string), onHover func(string, string), resolveIcon func(string) fyne.Resource) *AttributeGrid {
	InitDefinitions()
	ag := &AttributeGrid{
		values:      parseAttributesString(initialStr),
		onChange:    onChange,
		onHover:     onHover,
		resolveIcon: resolveIcon,
	}
	ag.createUI()
	return ag
}

func isAttributeProper(id string) bool {
	if strings.HasPrefix(id, "EAS_") || strings.HasPrefix(id, "HI_") {
		return false
	}
	if strings.HasPrefix(id, "FP_") && !strings.HasPrefix(id, "MB_ATT_") {
		return false
	}
	return strings.HasPrefix(id, "MB_ATT_") || strings.HasPrefix(id, "MB_RES_")
}

func parseAttributesString(s string) map[string]int {
	res := make(map[string]int)
	if s == "" {
		return res
	}
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

func isWeaponOrExplosiveAttribute(a AttributeDef) bool {
	id := a.ID
	weaponTokens := []string{
		"PISTOL", "BLASTER", "A280", "CLONERIFLE", "WESTARM5", "PROJECTILE",
		"DLT", "T21", "E_22", "EE3", "EE4", "DISRUPTOR", "BOWCASTER", "REPEATER",
		"DEMP2", "FLECHETTE", "CONCUSSION", "SHOTGUN", "AMBAN", "MINIGUN", "THROWER",
		"FLAMETHROWER", "WPFLAMETHROWER", "ROCKET", "PLX1", "UGL", "MGL", "FRAGS",
		"THERMAL", "BASE_TD", "GRENADE", "DET_PACK", "TRIP_MINE", "STICKY_BOMB",
		"SONIC_DETONATOR", "WHISTLINGBIRD", "CLONEBLOBS", "STRONGBLOX", "STRONGBLOBS",
		"DART", "QUICKDRAW", "QUICKTHROW", "FIREPOWER", "FIRERATE",
	}
	for _, t := range weaponTokens {
		if strings.Contains(id, t) {
			return true
		}
	}
	return false
}

func classifyAttributeToTopCategory(a AttributeDef) string {
	id := a.ID

	// 1. Force Powers & Disciplines
	if strings.HasPrefix(id, "MB_ATT_FP_") || id == "MB_ATT_FORCEBLOCK" || id == "MB_ATT_FORCEFOCUS" || id == "MB_ATT_FORCEATTUNE" || id == "MB_ATT_INAIR_FORCE_REGEN" {
		if strings.HasPrefix(id, "MB_ATT_FP_SABER") {
			return "Lightsaber Mastery"
		}
		return "Force Powers"
	}

	// 2. Lightsaber Combat
	if strings.HasPrefix(id, "MB_ATT_SABER") || strings.HasPrefix(id, "MB_ATT_SS_") || id == "MB_ATT_DEFLECT" {
		return "Lightsaber Mastery"
	}

	// 3. Class & Droid Tech
	if strings.HasPrefix(id, "MB_ATT_SBD_") || strings.HasPrefix(id, "MB_ATT_DEKA_") ||
		strings.HasPrefix(id, "MB_ATT_WOOKIE") ||
		id == "MB_ATT_CCTRAINING" || id == "MB_ATT_ET_CCTRAINING" ||
		id == "MB_ATT_HULL_STRENGTH" || id == "MB_ATT_BATTERY" || id == "MB_ATT_RECHARGE" ||
		id == "MB_ATT_SHOCKWAVE" || id == "MB_ATT_TURN_RATE" || id == "MB_ATT_WRISTLASER" ||
		id == "MB_ATT_WRIST_AMMO" || id == "MB_ATT_JETPACK" || id == "MB_ATT_FUEL" || id == "MB_ATT_FUELREGEN" ||
		id == "MB_ATT_RALLY" || id == "MB_ATT_ASSEMBLE" {
		return "Class & Droid Tech"
	}

	// 4. Logistics, Supplies & Tactical Gear
	if strings.HasPrefix(id, "MB_ATT_DISP_") || strings.HasPrefix(id, "MB_ATT_DROP_") || strings.HasPrefix(id, "MB_ATT_STIM_") ||
		id == "MB_ATT_SUPPLYDROP" || id == "MB_ATT_BACTA" || id == "MB_ATT_BACTA_BIG" ||
		id == "MB_ATT_MEDI_PACK" || id == "MB_ATT_AMMO_PACK" || id == "MB_ATT_STIMPACK" ||
		id == "MB_ATT_SPAWNER" || id == "MB_ATT_FORCEFIELD" || id == "MB_ATT_EWEB" ||
		id == "MB_ATT_SENTRY" || id == "MB_ATT_LASERCOVER" || id == "MB_ATT_STEALTH" ||
		id == "MB_ATT_CLOAK" || id == "MB_ATT_USE_DISTANCE" || id == "MB_ATT_GOODIE_KEY" || id == "MB_ATT_SECURITY_KEY" {
		return "Supplies & Tactical Gear"
	}

	// 5. Weapons & Explosives
	if isWeaponOrExplosiveAttribute(a) {
		return "Weapons & Explosives"
	}

	// 6. Physicals, Agility & Defense (Default umbrella for health/armor/perks/multipliers)
	return "Physicals, Agility & Defense"
}

func (ag *AttributeGrid) createUI() {
	categories := make(map[string][]AttributeDef)
	attributes := GetAttributes()

	for _, attr := range attributes {
		if !isAttributeProper(attr.ID) {
			continue
		}
		topCat := classifyAttributeToTopCategory(attr)
		categories[topCat] = append(categories[topCat], attr)
	}

	// 6 Canonical Top-Level Categories
	catOrder := []string{
		"Weapons & Explosives",
		"Physicals, Agility & Defense",
		"Force Powers",
		"Lightsaber Mastery",
		"Class & Droid Tech",
		"Supplies & Tactical Gear",
	}

	defaultOpen := map[string]bool{
		"Weapons & Explosives":         true,
		"Physicals, Agility & Defense": true,
		"Force Powers":                 true,
		"Lightsaber Mastery":           true,
		"Class & Droid Tech":           true,
	}

	var content *fyne.Container

	if ag.container != nil {
		content = ag.content
		content.Objects = nil
	} else {
		content = container.NewVBox()
		ag.content = content

		ag.search = NewInputEntry()
		ag.search.SetPlaceHolder("Filter Attributes...")
		ag.search.OnChanged = func(s string) {
			ag.filter = s
			ag.Refresh()
		}

		scroll := container.NewVScroll(content)
		ag.container = container.NewBorder(ag.search, nil, nil, nil, scroll)
	}

	filterLower := strings.ToLower(ag.filter)
	accordion := widget.NewAccordion()
	accordion.MultiOpen = true

	for _, catName := range catOrder {
		attrs, ok := categories[catName]
		if !ok && (catName != "Physicals, Agility & Defense" || ag.classScalarsBuilder == nil) {
			continue
		}

		var visibleAttrs []AttributeDef
		for _, attr := range attrs {
			if filterLower == "" ||
				strings.Contains(strings.ToLower(attr.Name), filterLower) ||
				strings.Contains(strings.ToLower(attr.ID), filterLower) {
				visibleAttrs = append(visibleAttrs, attr)
			}
		}

		if len(visibleAttrs) == 0 && (catName != "Physicals, Agility & Defense" || ag.classScalarsBuilder == nil) {
			continue
		}

		var catContent fyne.CanvasObject
		switch catName {
		case "Weapons & Explosives":
			catContent = ag.buildWeaponsGroupedView(visibleAttrs)
		case "Physicals, Agility & Defense":
			catContent = ag.buildPhysicalsGroupedView(visibleAttrs)
		case "Force Powers":
			catContent = ag.buildForceGroupedView(visibleAttrs)
		case "Lightsaber Mastery":
			catContent = ag.buildSaberGroupedView(visibleAttrs)
		case "Class & Droid Tech":
			catContent = ag.buildClassTechGroupedView(visibleAttrs)
		case "Supplies & Tactical Gear":
			catContent = ag.buildSuppliesGroupedView(visibleAttrs)
		default:
			catContent = ag.buildAlphaSortedGrid(visibleAttrs)
		}

		title := fmt.Sprintf("%s (%d)", catName, len(visibleAttrs))
		item := widget.NewAccordionItem(title, catContent)
		if filterLower != "" || defaultOpen[catName] {
			item.Open = true
		}
		accordion.Append(item)
	}

	if len(accordion.Items) == 0 {
		emptyMsg := widget.NewLabel("No attributes match your filter.")
		emptyMsg.Alignment = fyne.TextAlignCenter
		content.Add(emptyMsg)
	} else {
		content.Add(accordion)
	}
}

func (ag *AttributeGrid) createAttributeItem(attr AttributeDef) fyne.CanvasObject {
	currentVal := ag.values[attr.ID]

	var icon fyne.Resource
	if ag.resolveIcon != nil {
		icon = ag.resolveIcon(attr.ID)
	}

	w := NewAttributeToggleWidget(attr, currentVal, func(val int) {
		ag.updateValue(attr.ID, val)
	}, ag.onHover, icon)

	if ag.onUnhover != nil {
		w.SetOnInfoLeave(ag.onUnhover)
	}
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
	if ag.onChange != nil {
		var parts []string
		for k, v := range ag.values {
			if v > 0 {
				parts = append(parts, fmt.Sprintf("%s,%d", k, v))
			}
		}
		sort.Strings(parts)
		ag.onChange(strings.Join(parts, "|"))
	}
}

func attrDisplayName(a AttributeDef) string {
	if a.Name != "" && a.Name != a.ID && !strings.HasPrefix(a.Name, "MB_ATT_") {
		return strings.ToLower(a.Name)
	}
	return strings.ToLower(prettyAttributeName(a.ID))
}

// buildSectionTile builds a responsive card container with compact 275x44 items.
func (ag *AttributeGrid) buildSectionTile(name string, members []AttributeDef, isOther bool) fyne.CanvasObject {
	headerStyle := fyne.TextStyle{Bold: true}
	if isOther {
		headerStyle = fyne.TextStyle{Bold: true, Italic: true}
	}
	header := widget.NewLabelWithStyle(
		fmt.Sprintf("%s  ·  %d", name, len(members)),
		fyne.TextAlignLeading, headerStyle)

	grid := container.NewGridWrap(fyne.NewSize(280, 48))
	for _, a := range members {
		grid.Add(ag.createAttributeItem(a))
	}
	body := container.NewVBox(header, grid)
	return NewTilePanel(body, TileOpts{
		AccentColor: sectionAccent(name),
		FillAlpha:   10,
		StrokeAlpha: 35,
		Padded:      true,
	})
}

func (ag *AttributeGrid) buildAlphaSortedGrid(attrs []AttributeDef) fyne.CanvasObject {
	sorted := make([]AttributeDef, len(attrs))
	copy(sorted, attrs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return attrDisplayName(sorted[i]) < attrDisplayName(sorted[j])
	})
	grid := container.NewGridWrap(fyne.NewSize(280, 48))
	for _, a := range sorted {
		grid.Add(ag.createAttributeItem(a))
	}
	return grid
}

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

// ── Category 1: Weapons & Explosives ────────────────────────────
func (ag *AttributeGrid) buildWeaponsGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	pistols := map[string]bool{
		"MB_ATT_PISTOL": true, "MB_ATT_HEAVY_PISTOL": true,
		"MB_ATT_BRYAR_OLD": true, "MB_ATT_CR2": true,
		"MB_ATT_CLONE_PISTOL": true, "MB_ATT_MANDO_PISTOL": true,
		"MB_ATT_IMP_PISTOL": true, "MB_ATT_QUICKDRAW": true,
		"MB_ATT_BLASTER": true, "MB_ATT_A280": true,
		"MB_ATT_DLT20A": true, "MB_ATT_DLT19": true,
		"MB_ATT_E_22": true, "MB_ATT_EE3": true,
		"MB_ATT_EE4": true, "MB_ATT_T21": true,
		"MB_ATT_DC_CARBINE": true, "MB_ATT_PROJECTILE_RIFLE": true,
	}
	heavy := map[string]bool{
		"MB_ATT_CLONERIFLE": true, "MB_ATT_WESTARM5": true,
		"MB_ATT_AMBAN": true, "MB_ATT_DISRUPTOR": true,
		"MB_ATT_BOWCASTER": true, "MB_ATT_TRAD_BOWCASTER": true,
		"MB_ATT_REPEATER": true, "MB_ATT_FLECHETTE": true,
		"MB_ATT_DEMP2": true, "MB_ATT_IONRIFLE": true,
		"MB_ATT_MINIGUN": true, "MB_ATT_SHOTGUN": true,
		"MB_ATT_CONCUSSION": true, "MB_ATT_THROWER": true,
		"MB_ATT_THROWER_LIGHTNING": true, "MB_ATT_THROWER_ICE": true,
		"MB_ATT_THROWER_PLASMA": true, "MB_ATT_THROWER_FLAME": true,
		"MB_ATT_THROWER_POISON": true, "MB_ATT_FLAMETHROWER": true,
		"MB_ATT_WPFLAMETHROWER": true, "MB_ATT_ARC_RIFLE_SCOPE": true,
	}
	launchers := map[string]bool{
		"MB_ATT_UGL": true, "MB_ATT_UGL_BURST": true,
		"MB_ATT_UGL_IMPACT": true, "MB_ATT_UGL_BURST_MIXED": true,
		"MB_ATT_MGL": true, "MB_ATT_MGL_BURST": true,
		"MB_ATT_MGL_IMPACT": true, "MB_ATT_ARC_RIFLE_GRENADELAUNCHER": true,
		"MB_ATT_ROCKET": true, "MB_ATT_ROCKET_LAUNCHER": true, "MB_ATT_PLX1": true,
		"MB_ATT_CLONEBLOBS": true, "MB_ATT_STRONGBLOX": true, "MB_ATT_STRONGBLOBS": true,
		"MB_ATT_MICRO_GRENADES": true, "MB_ATT_REPEATER_NADES": true,
		"MB_ATT_FLECHETTE_NADES": true,
	}
	grenades := map[string]bool{
		"MB_ATT_FRAGS": true, "MB_ATT_THERMAL": true, "MB_ATT_THERMALS": true,
		"MB_ATT_BASE_TD": true, "MB_ATT_FIRE_GRENADES": true,
		"MB_ATT_PULSE_GRENADES": true, "MB_ATT_CRYOBAN_GRENADES": true,
		"MB_ATT_SONIC_DETONATOR": true, "MB_ATT_QUICKTHROW": true,
		"MB_ATT_DET_PACK": true, "MB_ATT_TRIP_MINES": true,
		"MB_ATT_STICKY_BOMBS": true, "MB_ATT_REMOTE_DETONATE": true,
		"MB_ATT_WHISTLINGBIRD": true,
	}

	classify := func(a AttributeDef) string {
		switch {
		case launchers[a.ID]:
			return "Launchers & Ordnance Attachments"
		case grenades[a.ID]:
			return "Grenades & Mines"
		case heavy[a.ID]:
			return "Heavy Rifles & Cannons"
		case pistols[a.ID]:
			return "Pistols & Blasters"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Pistols & Blasters", "Heavy Rifles & Cannons", "Launchers & Ordnance Attachments", "Grenades & Mines"},
		classify)
}

// ── Category 2: Physicals, Agility & Defense ────────────────────
func (ag *AttributeGrid) buildPhysicalsGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	movement := map[string]bool{
		"MB_ATT_BASESPEED": true, "MB_ATT_DEXTERITY": true, "MB_ATT_ACROBACY": true,
		"MB_ATT_DASH": true, "MB_ATT_DASH_JUMP": true, "MB_ATT_BUNNY_HOP": true,
		"MB_ATT_FLOAT_HOP": true, "MB_ATT_GRAPPLE_HOP": true, "MB_ATT_GRAPPLE_HOOK": true,
		"MB_ATT_KNOCKDOWN_ROLL": true, "MB_ATT_DODGE": true,
	}
	melee := map[string]bool{
		"MB_ATT_WOOKIE_STRENGTH": true, "MB_ATT_WOOKIEE_FURY": true,
		"MB_ATT_WOOKIE_BALANCE": true, "MB_ATT_WOOKIEE_AGILITY": true,
		"MB_ATT_KNIFE": true, "MB_ATT_SWORD": true, "MB_ATT_STUN_BATON": true,
		"MB_ATT_ELECTRO_STAFF": true, "MB_ATT_GUNBASH": true, "MB_ATT_FLIPKICK": true,
		"MB_ATT_GETUPS": true, "MB_ATT_BACKSTAB": true,
	}
	armor := map[string]bool{
		"MB_ATT_BLAST_ARMOUR": true, "MB_ATT_MAGNETIC_PLATING": true,
		"MB_ATT_CORTOSIS": true, "MB_ATT_BESKAR": true, "MB_ATT_DURABILITY": true,
		"MB_ATT_GUN_DEFENSE": true, "MB_ATT_DEFLECT": true, "MB_ATT_SHIELD_RECHARGE": true,
		"MB_ATT_SHIELD_RECHARGE2": true, "MB_ATT_ANTI_MT": true,
	}
	pools := map[string]bool{
		"MB_ATT_HEALTH": true, "MB_ATT_ARMOUR": true, "MB_ATT_AMMO": true,
		"MB_ATT_POWER": true, "MB_ATT_STAMINA": true, "MB_ATT_REGEN": true,
		"MB_ATT_HEALING": true, "MB_ATT_RESPAWNS": true,
		"MB_ATT_HEALTH_REGEN_CAP": true, "MB_ATT_HEALTH_REGEN_AMOUNT": true, "MB_ATT_HEALTH_REGEN_RATE": true,
		"MB_ATT_ARMOUR_REGEN_CAP": true, "MB_ATT_ARMOUR_REGEN_AMOUNT": true, "MB_ATT_ARMOUR_REGEN_RATE": true,
		"MB_ATT_RESOURCE_REGEN_CAP": true, "MB_ATT_RESOURCE_REGEN_AMOUNT": true, "MB_ATT_RESOURCE_REGEN_RATE": true,
		"MB_ATT_BLOCK_REGEN_CAP": true, "MB_ATT_BLOCK_REGEN_AMOUNT": true, "MB_ATT_BLOCK_REGEN_RATE": true,
	}
	multipliers := map[string]bool{
		"MB_ATT_AP_MULTIPLIER": true, "MB_ATT_BP_MULTIPLIER": true,
		"MB_ATT_CS_MULTIPLIER": true, "MB_ATT_AS_MULTIPLIER": true,
		"MB_ATT_ROF_MULTIPLIER": true, "MB_ATT_ROF_MELEE_MULTIPLIER": true,
		"MB_ATT_STM_MULTIPLIER": true, "MB_ATT_HACK_MULTIPLIER": true,
		"MB_ATT_KB_TAKEN_MULTIPLIER": true, "MB_ATT_KB_GIVEN_MULTIPLIER": true,
		"MB_ATT_DMG_TAKEN_MULTIPLIER": true, "MB_ATT_DMG_GIVEN_MULTIPLIER": true,
		"MB_ATT_MODELSCALE_MULTIPLIER": true,
	}

	box := container.NewVBox()
	if ag.classScalarsBuilder != nil {
		box.Add(ag.classScalarsBuilder())
	}

	classify := func(a AttributeDef) string {
		switch {
		case movement[a.ID]:
			return "Movement, Agility & Mobility"
		case melee[a.ID]:
			return "Melee & Hand-to-Hand"
		case armor[a.ID]:
			return "Armor Tech & Damage Reduction"
		case pools[a.ID]:
			return "Health, Armor & Regen Pools"
		case multipliers[a.ID]:
			return "Combat Multipliers & Tuning"
		}
		return ""
	}

	subView := ag.buildSubGroupedView(attrs,
		[]string{
			"Movement, Agility & Mobility",
			"Melee & Hand-to-Hand",
			"Armor Tech & Damage Reduction",
			"Health, Armor & Regen Pools",
			"Combat Multipliers & Tuning",
		},
		classify)

	box.Add(subView)
	return box
}

// ── Category 3: Force Powers ────────────────────────────────────
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
		"MB_ATT_FP_DRAIN": true, "MB_ATT_FP_BLIND": true, "MB_ATT_FP_DESTRUCTION": true,
		"MB_ATT_FP_DEADLYSIGHT": true, "MB_ATT_FP_STASIS": true, "MB_ATT_FP_REPULSE": true,
		"MB_ATT_FP_MIRALUKA": true,
	}
	tuning := map[string]bool{
		"MB_ATT_FORCEBLOCK": true, "MB_ATT_FORCEFOCUS": true, "MB_ATT_FORCEATTUNE": true,
		"MB_ATT_FP_MULTIPLIER": true, "MB_ATT_INAIR_FORCE_REGEN": true, "MB_ATT_FP_BATTERY": true,
	}

	classify := func(a AttributeDef) string {
		switch {
		case core[a.ID]:
			return "Core Powers"
		case defensive[a.ID]:
			return "Light & Defensive"
		case offensive[a.ID]:
			return "Dark & Offensive"
		case tuning[a.ID]:
			return "Force Pool, Focus & Tuning"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Core Powers", "Light & Defensive", "Dark & Offensive", "Force Pool, Focus & Tuning"},
		classify)
}

// ── Category 4: Lightsaber Mastery ──────────────────────────────
func (ag *AttributeGrid) buildSaberGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	forms := map[string]bool{
		"MB_ATT_SS_FAST": true, "MB_ATT_SS_MEDIUM": true, "MB_ATT_SS_STRONG": true,
		"MB_ATT_SS_DESANN": true, "MB_ATT_SS_TAVION": true, "MB_ATT_SS_DUAL": true,
		"MB_ATT_SS_STAFF": true,
	}
	proficiency := map[string]bool{
		"MB_ATT_SABER_FAST": true, "MB_ATT_SABER_MEDIUM": true, "MB_ATT_SABER_STRONG": true,
		"MB_ATT_SABER_DOUBLES": true, "MB_ATT_SABER_MASTERY": true,
		"MB_ATT_FP_SABER_OFFENSE": true, "MB_ATT_FP_SABER_DEFENSE": true, "MB_ATT_FP_SABERTHROW": true,
	}
	damage := map[string]bool{
		"MB_ATT_SABER_DAMAGE": true, "MB_ATT_SABERTHROW_DAMAGE": true,
		"MB_ATT_SABERSPECIAL_DAMAGE": true, "MB_ATT_SABER_MAXCHAIN": true,
		"MB_ATT_SABER_COMBO": true, "MB_ATT_SABER_COMBO_NONE": true,
	}

	classify := func(a AttributeDef) string {
		switch {
		case forms[a.ID] || strings.HasPrefix(a.ID, "MB_ATT_SS_"):
			return "Style Unlocks & Forms"
		case proficiency[a.ID]:
			return "Style Mastery & Proficiency"
		case damage[a.ID]:
			return "Damage, Chains & Combos"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Style Unlocks & Forms", "Style Mastery & Proficiency", "Damage, Chains & Combos"},
		classify)
}

// ── Category 5: Class & Droid Tech ──────────────────────────────
func (ag *AttributeGrid) buildClassTechGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	sbd := map[string]bool{
		"MB_ATT_SBD_CANNON": true, "MB_ATT_HULL_STRENGTH": true,
		"MB_ATT_FIREPOWER": true, "MB_ATT_FIRERATE": true,
		"MB_ATT_BATTERY": true, "MB_ATT_RECHARGE": true,
		"MB_ATT_ZOOM": true, "MB_ATT_RADAR": true, "MB_ATT_SHOCKWAVE": true,
	}
	deka := map[string]bool{
		"MB_ATT_DEKA_SHIELD": true, "MB_ATT_DEKA_HULL": true,
		"MB_ATT_DEKA_DEPLOY": true, "MB_ATT_DEKA_POWER": true,
		"MB_ATT_TURN_RATE": true, "MB_ATT_DEKA_DISCHARGE": true,
	}
	clone := map[string]bool{
		"MB_ATT_CCTRAINING": true, "MB_ATT_ET_CCTRAINING": true,
		"MB_ATT_CLONEBLOBS": true, "MB_ATT_CLONERIFLE": true,
		"MB_ATT_ARC_RIFLE_SCOPE": true, "MB_ATT_ARC_RIFLE_GRENADELAUNCHER": true,
	}
	mando := map[string]bool{
		"MB_ATT_JETPACK": true, "MB_ATT_FUEL": true, "MB_ATT_FUELREGEN": true,
		"MB_ATT_BESKAR": true, "MB_ATT_WRISTLASER": true, "MB_ATT_WRIST_AMMO": true,
		"MB_ATT_DRONE": true, "MB_ATT_TRACKING_DART": true, "MB_ATT_POISON_DART": true,
	}
	hero := map[string]bool{
		"MB_ATT_RALLY": true, "MB_ATT_ASSEMBLE": true, "MB_ATT_DODGE": true,
		"MB_ATT_DASH": true, "MB_ATT_HEALING": true, "MB_ATT_RESPAWNS": true,
	}
	wookiee := map[string]bool{
		"MB_ATT_WOOKIE_HEALTH": true, "MB_ATT_WOOKIE_STRENGTH": true,
		"MB_ATT_WOOKIEE_FURY": true, "MB_ATT_WOOKIE_BALANCE": true,
		"MB_ATT_WOOKIEE_AGILITY": true,
	}

	classify := func(a AttributeDef) string {
		switch {
		case sbd[a.ID] || strings.HasPrefix(a.ID, "MB_ATT_SBD_"):
			return "Super Battle Droid (SBD)"
		case deka[a.ID] || strings.HasPrefix(a.ID, "MB_ATT_DEKA_"):
			return "Droideka (Deka)"
		case clone[a.ID] || strings.HasPrefix(a.ID, "MB_ATT_CLONE") || strings.HasPrefix(a.ID, "MB_ATT_ARC_"):
			return "Clone Trooper & ARC"
		case mando[a.ID] || strings.HasPrefix(a.ID, "MB_ATT_MANDO_"):
			return "Mandalorian & Bounty Hunter"
		case wookiee[a.ID] || strings.HasPrefix(a.ID, "MB_ATT_WOOKIE"):
			return "Wookiee"
		case hero[a.ID]:
			return "Hero, Commander & Support"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{
			"Super Battle Droid (SBD)",
			"Droideka (Deka)",
			"Clone Trooper & ARC",
			"Mandalorian & Bounty Hunter",
			"Hero, Commander & Support",
			"Wookiee",
		},
		classify)
}

// ── Category 6: Supplies & Tactical Gear ────────────────────────
func (ag *AttributeGrid) buildSuppliesGroupedView(attrs []AttributeDef) fyne.CanvasObject {
	dispensers := func(id string) bool {
		return strings.HasPrefix(id, "MB_ATT_DISP_") || strings.HasPrefix(id, "MB_ATT_DROP_") || strings.HasPrefix(id, "MB_ATT_STIM_") || id == "MB_ATT_SUPPLYDROP"
	}
	medical := map[string]bool{
		"MB_ATT_BACTA": true, "MB_ATT_BACTA_BIG": true,
		"MB_ATT_MEDI_PACK": true, "MB_ATT_AMMO_PACK": true,
		"MB_ATT_STIMPACK": true, "MB_ATT_SPAWNER": true,
	}
	utility := map[string]bool{
		"MB_ATT_STEALTH": true, "MB_ATT_CLOAK": true, "MB_ATT_LASERCOVER": true,
		"MB_ATT_FORCEFIELD": true, "MB_ATT_EWEB": true, "MB_ATT_SENTRY": true,
		"MB_ATT_USE_DISTANCE": true, "MB_ATT_GOODIE_KEY": true, "MB_ATT_SECURITY_KEY": true,
	}

	classify := func(a AttributeDef) string {
		switch {
		case dispensers(a.ID):
			return "Field Dispensers & Drops"
		case medical[a.ID]:
			return "Medical & Portable Gear"
		case utility[a.ID]:
			return "Tactical Utility & Infiltration"
		}
		return ""
	}
	return ag.buildSubGroupedView(attrs,
		[]string{"Field Dispensers & Drops", "Medical & Portable Gear", "Tactical Utility & Infiltration"},
		classify)
}

func (ag *AttributeGrid) GetContent() fyne.CanvasObject {
	return ag.container
}
