package main

// Point Buy editor — models the MBII Legends 2.0 loadout system:
//
//   * Single archetype (hasCustomSpec <= 1): 15 skill slots
//   * Multi archetype (hasCustomSpec 2–3): 15 slots per archetype,
//     stored contiguously in CustomSkills[] — spec 1 uses 0-14,
//     spec 2 uses 15-29, spec 3 uses 30-44.
//
// Each slot is one of three modes: Empty, Header (section divider),
// Skill (real MB_ATT_ pick with costs). Slot view:
//
//   ┌─── Archetype: "Gunner"  icon [📎]  [rename] ───┐
//   │ 0  [Header]  -Weapons-                         │
//   │ 1  [Skill ]  [🔫] MB_ATT_PISTOL  "Pistol:"    │
//   │                    0,4,10   max 14             │
//   │                    desc: "Extra shots unlock"  │
//   │ 2  [Skill ]  ...                               │
//   │ ... 15 rows per archetype ...                  │
//   └────────────────────────────────────────────────┘
//
// Archetype tabs sit at the top; count picker adds/removes
// archetypes by toggling HasCustomSpec. Budget tracker sums
// max-spend across the currently-visible archetype vs mbPoints.
//
// Rank modifiers (rank*) live in their own collapsible section
// below — they apply across all archetypes since MBII doesn't
// namespace them per spec.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	slotsPerArchetype = 15 // c_att_skill_0..14 per spec
	maxArchetypes     = 3
	maxTotalSlots     = slotsPerArchetype * maxArchetypes // 45
)

// KnownRankAttributes lists the `rank*` field names from Legends 2.0's
// expanded system — used by the "Add Rank Modifier" picker.
var KnownRankAttributes = []string{
	"rankHealth", "rankArmor",
	"rankAP", "rankBP", "rankCS", "rankAS",
	"rankROF", "rankSTM",
	"rankKbTaken", "rankKbGiven",
	"rankDmgTaken", "rankDmgGiven",
	"rankBaseSpeed", "rankSaberDamage", "rankSaberThrowDamage",
	"rankSaberMaxChain",
	"rankModelScale", "rankROFMelee",
	"rankHealthRegenAmount", "rankHealthRegenRate", "rankHealthRegenCap",
	"rankArmourRegenAmount", "rankArmourRegenRate", "rankArmourRegenCap",
	"rankBlockRegenAmount", "rankBlockRegenRate", "rankBlockRegenCap",
	"rankResourceRegenAmount", "rankResourceRegenRate", "rankResourceRegenCap",
	"rankForcePool", "rankForceRegen",
	"rankHack",
}

type PointBuyUI struct {
	editor *MBCHEditor

	container *fyne.Container

	// Header controls (apply to all archetypes).
	customBuildCheck *widget.Check
	mbPointsEntry    *widget.Entry
	budgetLabel      *widget.Label

	// Archetype strip — count picker + spec-tab container. Re-rendered
	// when the count changes so tab count matches HasCustomSpec. The
	// count picker is a button row (1 / 2 / 3) so flipping between
	// single- and multi-spec feels tactile like the in-game class-
	// variant picker rather than a generic dropdown.
	archetypeCountBtns []*widget.Button
	specTabs           *container.AppTabs
	archetypeHost      *fyne.Container // holds specTabs + the count picker row

	// Per-slot widgets, indexed globally (0..maxTotalSlots-1). Each
	// archetype's tab owns slots [i*15, (i+1)*15).
	slotRows      []*fyne.Container
	slotModes     []*widget.Select
	slotSkills    []*widget.Select
	slotIcons     []*canvas.Image // raster-friendly; widget.Icon renders at theme-icon size and loses raster detail
	slotNames     []*widget.Entry
	slotRanks     []*widget.Entry
	slotDescs     []*widget.Entry
	slotMaxCost   []*widget.Label
	slotForms     []*widget.Form        // one form per slot — rebuilt items drive mode-based visibility
	slotFormItems [][]*widget.FormItem  // [slot][0..3] = Skill / Name / Costs / Description items

	// Per-archetype spec header (name + icon) widgets.
	specNameEntries [maxArchetypes]*widget.Entry
	specIconEntries [maxArchetypes]*widget.Entry

	// Rank modifiers (applies across archetypes).
	rankAttrContainer *fyne.Container

	// Edit / Simulate tab strip. Simulator is read-only and lets
	// the author test-drive the budget the way a real player would.
	modeTabs  *container.AppTabs
	simulator *PointBuySimulator
}

func NewPointBuyUI(editor *MBCHEditor) *PointBuyUI {
	p := &PointBuyUI{editor: editor}
	p.createUI()
	return p
}

func (p *PointBuyUI) createUI() {
	// --- Header: custom-build toggle + budget ---
	p.customBuildCheck = widget.NewCheck("Custom Build enabled", func(on bool) {
		if on {
			p.editor.character.IsCustomBuild = 1
		} else {
			p.editor.character.IsCustomBuild = 0
		}
		// Mirror to the legacy Identity-accordion checkbox so updateCharacterFromUI's
		// save-time read of e.isCustomCheck.Checked doesn't clobber this value.
		// SetChecked does not re-fire OnChanged in Fyne, so this won't loop.
		if p.editor.isCustomCheck != nil {
			p.editor.isCustomCheck.SetChecked(on)
		}
		p.editor.markDirty()
	})

	p.mbPointsEntry = NewInputEntry()
	p.mbPointsEntry.SetPlaceHolder("100")
	p.mbPointsEntry.OnChanged = func(s string) {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return
		}
		p.editor.character.MBPoints = n
		// Mirror to the legacy Identity-accordion entry — same reason as above.
		if p.editor.mbPointsEntry != nil && p.editor.mbPointsEntry.Text != s {
			p.editor.mbPointsEntry.SetText(s)
		}
		p.editor.markDirty()
		p.refreshBudget()
	}

	p.budgetLabel = widget.NewLabel("max spend: 0 / 0")

	// Archetype count buttons. GridWrap forces all three to the same
	// width so "1 / 2 / 3" reads as a coherent toggle group, not three
	// differently-sized buttons.
	p.archetypeCountBtns = make([]*widget.Button, maxArchetypes)
	archetypeBtnRow := container.NewHBox()
	for i := 0; i < maxArchetypes; i++ {
		n := i + 1
		btn := widget.NewButton(strconv.Itoa(n), func() {
			p.selectArchetypeCount(n, true)
		})
		p.archetypeCountBtns[i] = btn
		archetypeBtnRow.Add(container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 34)), btn))
	}

	headerForm := widget.NewForm(
		widget.NewFormItem("Custom Build", p.customBuildCheck),
		widget.NewFormItem("mbPoints", p.mbPointsEntry),
		widget.NewFormItem("Archetypes", archetypeBtnRow),
		widget.NewFormItem("Budget check", p.budgetLabel),
	)
	headerForm.Items[0].HintText = "If off, the class uses the classic fixed loadout instead of point-buy"
	headerForm.Items[1].HintText = "Total points the player spends across the active archetype (e.g. 100)"
	headerForm.Items[2].HintText = "How many specs the player picks from — single, dual, or three archetypes"
	headerForm.Items[3].HintText = "Live check of the priciest spec's max spend vs the mbPoints ceiling"

	// --- Build all 45 slot widgets up front. Tabs only show the
	// slice relevant to each archetype; slots outside the active
	// archetype range stay unreferenced (but kept in memory so
	// toggling HasCustomSpec doesn't lose edits).
	attrOptions := pointBuyAttrOptions()
	p.slotRows = make([]*fyne.Container, maxTotalSlots)
	p.slotModes = make([]*widget.Select, maxTotalSlots)
	p.slotSkills = make([]*widget.Select, maxTotalSlots)
	p.slotIcons = make([]*canvas.Image, maxTotalSlots)
	p.slotNames = make([]*widget.Entry, maxTotalSlots)
	p.slotRanks = make([]*widget.Entry, maxTotalSlots)
	p.slotDescs = make([]*widget.Entry, maxTotalSlots)
	p.slotMaxCost = make([]*widget.Label, maxTotalSlots)
	p.slotForms = make([]*widget.Form, maxTotalSlots)
	p.slotFormItems = make([][]*widget.FormItem, maxTotalSlots)
	for i := 0; i < maxTotalSlots; i++ {
		p.slotRows[i] = p.buildSlotRow(i, attrOptions)
	}

	// --- Archetype tabs. Rebuilt by rebuildArchetypeTabs so the
	// number of tabs tracks HasCustomSpec.
	p.specTabs = container.NewAppTabs()
	p.archetypeHost = container.NewStack(p.specTabs)

	// --- Rank modifiers.
	p.rankAttrContainer = container.NewVBox()

	addRankAttrBtn := widget.NewButtonWithIcon("Add Rank Modifier", theme.ContentAddIcon(), func() {
		p.showAddRankAttrDialog()
	})

	// Simulator pane — renders the editor's data as click-to-buy
	// rank pills against the live mbPoints ceiling. Rebuilt on tab
	// change or when the edit data mutates.
	p.simulator = NewPointBuySimulator(p)

	// Edit / Simulate tabs. Edit holds the archetype + slot editor;
	// Simulate lets the author play the build to test balance.
	p.modeTabs = container.NewAppTabs(
		container.NewTabItem("Edit", p.archetypeHost),
		container.NewTabItem("Simulate", p.simulator.GetContent()),
	)
	// Refresh simulator state each time its tab gains focus so
	// recent edits propagate without requiring a file reload.
	p.modeTabs.OnSelected = func(tab *container.TabItem) {
		if tab == p.modeTabs.Items[1] { // Simulate
			p.simulator.Refresh()
		}
	}

	// Multiplier hint card — explains where MB_ATT_*_MULTIPLIER
	// attributes belong. Authors used to look for them in the main
	// Attributes grid; they're now filtered out (since they're
	// point-buy primitives, not loadout toggles) and live in the slot
	// picker dropdown below. The hint avoids a "where did the
	// multipliers go?" hunt.
	multHint := widget.NewLabelWithStyle(
		"Custom multipliers (MB_ATT_AP_MULTIPLIER, MB_ATT_BP_MULTIPLIER, MB_ATT_DMG_GIVEN_MULTIPLIER, etc.) "+
			"are point-buy primitives — pick them from any slot's skill dropdown below to let players buy "+
			"into custom multiplier ranks. They're intentionally not in the Attributes grid because they "+
			"are bought, not toggled.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	multHint.Wrapping = fyne.TextWrapWord

	p.container = container.NewVBox(
		widget.NewCard("Point Buy Budget",
			"Toggle Custom Build, set total mbPoints, and pick how many archetypes the class offers (1–3).",
			container.NewPadded(headerForm),
		),
		widget.NewCard("Custom Multipliers",
			"Point-buy multipliers — pick these from a slot's skill dropdown.",
			container.NewPadded(multHint),
		),
		widget.NewCard("Build & Simulator",
			"Edit skills here, then flip to Simulate to try the build.",
			p.modeTabs,
		),
		widget.NewCard("Rank Modifiers",
			"Per-rank stat overrides (rankHealth, rankAP, rankROF, …). Values are comma-separated per rank. Shared across all archetypes.",
			container.NewVBox(addRankAttrBtn, p.rankAttrContainer),
		),
	)
}

// buildSlotRow renders one c_att_skill_N slot (global index 0..44).
// The slot number shown in the UI is relative to the archetype
// (so archetype 2 slot 0 displays "0" even though its stored index
// is 15).
func (p *PointBuyUI) buildSlotRow(globalIdx int, attrOptions []string) *fyne.Container {
	displayIdx := globalIdx % slotsPerArchetype
	slotLabel := canvas.NewText(fmt.Sprintf("%d", displayIdx), theme.PlaceHolderColor())
	slotLabel.TextSize = SizeSmall
	slotLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	mode := widget.NewSelect([]string{"Empty", "Header", "Skill"}, nil)
	mode.PlaceHolder = "Empty"
	p.slotModes[globalIdx] = mode

	skillSel := widget.NewSelect(attrOptions, nil)
	skillSel.PlaceHolder = "MB_ATT_..."
	p.slotSkills[globalIdx] = skillSel

	iconW := canvas.NewImageFromResource(theme.FileImageIcon())
	iconW.FillMode = canvas.ImageFillContain
	iconW.ScaleMode = canvas.ImageScaleSmooth
	iconW.SetMinSize(fyne.NewSize(24, 24))
	p.slotIcons[globalIdx] = iconW

	nameEntry := NewInputEntry()
	nameEntry.SetPlaceHolder("Display name (e.g. \"Blaster Pistol:\")")
	p.slotNames[globalIdx] = nameEntry

	rankEntry := NewInputEntry()
	rankEntry.SetPlaceHolder("0,4,10")
	p.slotRanks[globalIdx] = rankEntry

	descEntry := NewInputEntry()
	descEntry.SetPlaceHolder("Optional description (c_att_descs)")
	p.slotDescs[globalIdx] = descEntry

	maxCost := widget.NewLabel("")
	maxCost.TextStyle = fyne.TextStyle{Italic: true}
	p.slotMaxCost[globalIdx] = maxCost

	mode.OnChanged = func(s string) {
		switch s {
		case "Empty":
			p.editor.character.CustomSkills[globalIdx] = ""
			p.editor.character.CustomNames[globalIdx] = ""
			p.editor.character.CustomRanks[globalIdx] = ""
			p.editor.character.CustomDescs[globalIdx] = ""
			skillSel.SetSelected("")
			nameEntry.SetText("")
			rankEntry.SetText("")
			descEntry.SetText("")
		case "Header":
			p.editor.character.CustomSkills[globalIdx] = "MB_ATT_INVALID"
			if p.editor.character.CustomNames[globalIdx] == "" {
				p.editor.character.CustomNames[globalIdx] = "-Section-"
				nameEntry.SetText("-Section-")
			}
			p.editor.character.CustomRanks[globalIdx] = "-1"
			skillSel.SetSelected("MB_ATT_INVALID")
			rankEntry.SetText("-1")
		case "Skill":
			if p.editor.character.CustomSkills[globalIdx] == "MB_ATT_INVALID" ||
				p.editor.character.CustomSkills[globalIdx] == "" {
				p.editor.character.CustomSkills[globalIdx] = ""
				skillSel.SetSelected("")
			}
			if p.editor.character.CustomRanks[globalIdx] == "-1" {
				p.editor.character.CustomRanks[globalIdx] = ""
				rankEntry.SetText("")
			}
		}
		p.refreshSlotLayout(globalIdx)
		p.editor.markDirty()
		p.refreshBudget()
	}

	skillSel.OnChanged = func(s string) {
		p.editor.character.CustomSkills[globalIdx] = s
		p.refreshSlotIcon(globalIdx)
		p.editor.markDirty()
	}
	nameEntry.OnChanged = func(s string) {
		p.editor.character.CustomNames[globalIdx] = s
		p.editor.markDirty()
	}
	rankEntry.OnChanged = func(s string) {
		p.editor.character.CustomRanks[globalIdx] = s
		p.refreshSlotMaxCost(globalIdx)
		p.refreshBudget()
		p.editor.markDirty()
	}
	descEntry.OnChanged = func(s string) {
		p.editor.character.CustomDescs[globalIdx] = s
		p.editor.markDirty()
	}

	// Skill row packs the attribute dropdown between the resolved icon
	// (for visual recall) and the max-cost label (live sum readout).
	skillRow := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(28, 28), iconW),
		maxCost,
		skillSel,
	)

	// Form carries the per-field label + HintText so authors can see
	// what each input does even after they've typed into it (unlike
	// placeholders, which vanish the moment the field has content).
	// Items stay stashed on the slot so refreshSlotLayout can rebuild
	// form.Items based on mode (Empty → nothing, Header → just Name,
	// Skill → all four) — hiding the widget inside a form item leaves
	// the label + hint visible, so rebuilding is the right fix.
	skillItem := widget.NewFormItem("Skill", skillRow)
	skillItem.HintText = "MB_ATT_* attribute ID — what this row grants (weapon, ability, perk)"
	nameItem := widget.NewFormItem("Name", nameEntry)
	nameItem.HintText = "Display label shown in the in-game loadout menu (e.g. \"A280 Blaster Rifle:\")"
	costsItem := widget.NewFormItem("Costs", rankEntry)
	costsItem.HintText = "Per-rank point cost, comma-separated. \"0,4,10\" → rank 1 free, rank 2 costs 4, rank 3 costs 10"
	descItem := widget.NewFormItem("Description", descEntry)
	descItem.HintText = "Optional tooltip shown under the rank in-game (e.g. \"Adds secondary fire\")"

	form := widget.NewForm(skillItem, nameItem, costsItem, descItem)
	p.slotForms[globalIdx] = form
	p.slotFormItems[globalIdx] = []*widget.FormItem{skillItem, nameItem, costsItem, descItem}

	left := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(24, 24), container.NewCenter(slotLabel)),
		container.NewGridWrap(fyne.NewSize(90, 36), mode),
	)

	row := container.NewBorder(nil, widget.NewSeparator(),
		left, nil, form)
	return row
}

// selectArchetypeCount applies the N-archetype choice: updates the
// stored HasCustomSpec (only on user-initiated clicks, not when
// UpdateUI is syncing load), restyles the button row so the active
// count renders as HighImportance, and rebuilds the tab strip. The
// fromUser flag prevents load-time sync from dirtying the file.
func (p *PointBuyUI) selectArchetypeCount(n int, fromUser bool) {
	if n < 1 {
		n = 1
	}
	if n > maxArchetypes {
		n = maxArchetypes
	}
	for i, btn := range p.archetypeCountBtns {
		if i+1 == n {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}
	if !fromUser {
		// UpdateUI path — caller already rebuilds tabs + refreshes
		// budget/simulator after populating slot data. Avoid double
		// work and don't mark the file dirty on load.
		return
	}
	// HasCustomSpec > 1 means multi-archetype; single archetype
	// leaves it at 0 or 1 (both generate identically).
	if n > 1 {
		p.editor.character.HasCustomSpec = n
	} else {
		p.editor.character.HasCustomSpec = 0
	}
	p.editor.markDirty()
	p.rebuildArchetypeTabs()
	p.refreshBudget()
	if p.simulator != nil {
		p.simulator.Refresh()
	}
}

// rebuildArchetypeTabs re-renders the tab strip so it has exactly
// HasCustomSpec tabs (min 1). Each tab holds a VBox of 15 slot rows
// from the appropriate window into the global slot arrays. A spec-
// header row (name + icon) sits above each tab's slots when there's
// more than one archetype — otherwise the header's pointless clutter.
func (p *PointBuyUI) rebuildArchetypeTabs() {
	count := p.editor.character.HasCustomSpec
	if count < 2 {
		count = 1
	}
	if count > maxArchetypes {
		count = maxArchetypes
	}

	p.specTabs.Items = nil
	for spec := 0; spec < count; spec++ {
		p.specTabs.Append(container.NewTabItem(p.specTabTitle(spec, count), p.buildSpecPane(spec, count)))
	}
	p.specTabs.Refresh()
}

func (p *PointBuyUI) specTabTitle(spec, count int) string {
	if count == 1 {
		return "Skills"
	}
	name := p.editor.character.CustomSpecNames[spec]
	if name == "" {
		return fmt.Sprintf("Spec %d", spec+1)
	}
	return name
}

// buildSpecPane builds the contents of one archetype tab: the optional
// spec-header row (name + icon) + the 15 slot rows.
func (p *PointBuyUI) buildSpecPane(spec, totalSpecs int) fyne.CanvasObject {
	content := container.NewVBox()

	if totalSpecs > 1 {
		nameEntry := NewInputEntry()
		nameEntry.SetPlaceHolder(fmt.Sprintf("Spec %d name (shown as tab title)", spec+1))
		nameEntry.SetText(p.editor.character.CustomSpecNames[spec])
		nameEntry.OnChanged = func(s string) {
			p.editor.character.CustomSpecNames[spec] = s
			// Update tab title live so the UI matches.
			if spec < len(p.specTabs.Items) {
				p.specTabs.Items[spec].Text = p.specTabTitle(spec, totalSpecs)
				p.specTabs.Refresh()
			}
			p.editor.markDirty()
		}
		p.specNameEntries[spec] = nameEntry

		iconEntry := NewInputEntry()
		iconEntry.SetPlaceHolder("Icon path (e.g. gfx/menus/alpha/icon_weap_accuracy)")
		iconEntry.SetText(p.editor.character.CustomSpecIcons[spec])
		iconEntry.OnChanged = func(s string) {
			p.editor.character.CustomSpecIcons[spec] = s
			p.editor.markDirty()
		}
		p.specIconEntries[spec] = iconEntry

		header := widget.NewForm(
			widget.NewFormItem("Spec Name", nameEntry),
			widget.NewFormItem("Spec Icon", iconEntry),
		)
		content.Add(header)
		content.Add(widget.NewSeparator())
	}

	// Slot rows — 15 from the right window.
	base := spec * slotsPerArchetype
	slotBox := container.NewVBox()
	for i := 0; i < slotsPerArchetype; i++ {
		slotBox.Add(p.slotRows[base+i])
	}
	scroll := container.NewVScroll(slotBox)
	scroll.SetMinSize(fyne.NewSize(0, 420))
	content.Add(scroll)

	return content
}

// refreshSlotLayout rebuilds the slot's form.Items so only the fields
// relevant to the current mode render. Unlike Hide() on a FormItem's
// content widget, rebuilding the slice actually removes the label +
// hint rows that would otherwise leave empty blocks behind.
//   Empty  → no fields at all
//   Header → just the Name entry (the section label text)
//   Skill  → all four: Skill, Name, Costs, Description
func (p *PointBuyUI) refreshSlotLayout(i int) {
	form := p.slotForms[i]
	items := p.slotFormItems[i]
	if form == nil || items == nil {
		return
	}
	mode := p.slotModes[i].Selected
	switch mode {
	case "Empty":
		form.Items = nil
	case "Header":
		// Items index 1 = Name.
		form.Items = []*widget.FormItem{items[1]}
	case "Skill":
		form.Items = items
	}
	form.Refresh()
	p.slotRows[i].Refresh()
}

func (p *PointBuyUI) refreshSlotIcon(i int) {
	setRes := func(res fyne.Resource) {
		p.slotIcons[i].Resource = res
		p.slotIcons[i].Refresh()
	}
	skill := p.editor.character.CustomSkills[i]
	if skill == "" || skill == "MB_ATT_INVALID" {
		setRes(theme.FileImageIcon())
		return
	}
	if p.editor.iconResolver == nil || p.editor.assetBrowser == nil {
		setRes(theme.FileImageIcon())
		return
	}
	path := p.editor.iconResolver.ResolveAttributeIcon(skill)
	if res := p.editor.assetBrowser.LoadIconResource(path); res != nil {
		setRes(res)
		return
	}
	setRes(theme.FileImageIcon())
}

func (p *PointBuyUI) refreshSlotMaxCost(i int) {
	total := parseRankCostSum(p.editor.character.CustomRanks[i])
	if total <= 0 {
		p.slotMaxCost[i].SetText("")
	} else {
		p.slotMaxCost[i].SetText(fmt.Sprintf("max %d", total))
	}
}

// refreshBudget recomputes the max-spend ceiling across ALL
// archetypes combined. Per-archetype budget isn't how MBII's runtime
// works — a player only buys within one archetype at a time, so the
// real budget comparison is mbPoints vs max-of-any-spec. Show the
// max-of-any instead of a sum for that reason.
func (p *PointBuyUI) refreshBudget() {
	specs := p.editor.character.HasCustomSpec
	if specs < 2 {
		specs = 1
	}
	if specs > maxArchetypes {
		specs = maxArchetypes
	}
	worst := 0
	for s := 0; s < specs; s++ {
		total := 0
		for i := 0; i < slotsPerArchetype; i++ {
			total += parseRankCostSum(p.editor.character.CustomRanks[s*slotsPerArchetype+i])
		}
		if total > worst {
			worst = total
		}
	}
	target := p.editor.character.MBPoints
	msg := fmt.Sprintf("max spend per spec: %d / %d", worst, target)
	if target > 0 && worst < target/2 {
		msg += " · consider adding more purchasable skills"
	} else if target > 0 && worst > target*3 {
		msg += " · costs far exceed budget"
	}
	p.budgetLabel.SetText(msg)
}

// parseRankCostSum parses a CSV of rank costs and returns the sum.
// Header rows ("-1") return 0.
func parseRankCostSum(csv string) int {
	csv = strings.TrimSpace(csv)
	if csv == "" || csv == "-1" {
		return 0
	}
	total := 0
	for _, part := range strings.Split(csv, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			continue
		}
		total += n
	}
	return total
}

// pointBuyAttrOptions returns the list of MB_ATT_* IDs available.
// Includes MB_ATT_INVALID for Header rows.
func pointBuyAttrOptions() []string {
	opts := []string{"MB_ATT_INVALID"}
	attrs := GetAttributes()
	ids := make([]string, 0, len(attrs))
	for _, a := range attrs {
		ids = append(ids, a.ID)
	}
	sort.Strings(ids)
	opts = append(opts, ids...)
	return opts
}

func (p *PointBuyUI) showAddRankAttrDialog() {
	keySelect := widget.NewSelectEntry(KnownRankAttributes)
	keySelect.PlaceHolder = "Select or type attribute (e.g., rankHealth)"
	valueEntry := NewInputEntry()
	valueEntry.PlaceHolder = "Comma-separated values (e.g., 100,120,150)"

	dialog.ShowCustomConfirm("Add Rank Modifier", "Add", "Cancel", container.NewVBox(
		widget.NewLabel("Modifier:"),
		keySelect,
		widget.NewLabel("Per-rank values (comma-separated):"),
		valueEntry,
	), func(ok bool) {
		if ok && keySelect.Text != "" && valueEntry.Text != "" {
			if p.editor.character.RankAttributes == nil {
				p.editor.character.RankAttributes = map[string]string{}
			}
			p.editor.character.RankAttributes[keySelect.Text] = valueEntry.Text
			p.refreshRankAttributes()
			p.editor.markDirty()
		}
	}, p.editor.app.mainWindow)
}

func (p *PointBuyUI) refreshRankAttributes() {
	p.rankAttrContainer.Objects = nil
	if p.editor.character.RankAttributes == nil || len(p.editor.character.RankAttributes) == 0 {
		p.rankAttrContainer.Refresh()
		return
	}

	// Group modifiers by domain so the list doesn't read as a
	// 30-row alphabetical soup. Order is: Core (the "everybody
	// uses this" stats) → Combat → Regen → Resource → Other.
	groups := []struct {
		name string
		keys []string
	}{
		{"Core", nil},
		{"Combat", nil},
		{"Regen", nil},
		{"Resource", nil},
		{"Other", nil},
	}
	for k := range p.editor.character.RankAttributes {
		idx := 4 // Other
		switch {
		case strings.Contains(k, "Regen"):
			idx = 2
		case strings.HasPrefix(k, "rankAP"),
			strings.HasPrefix(k, "rankBP"),
			strings.HasPrefix(k, "rankCS"),
			strings.HasPrefix(k, "rankAS"),
			strings.HasPrefix(k, "rankSaber"):
			idx = 1
		case strings.HasPrefix(k, "rankForce"),
			strings.HasPrefix(k, "rankResource"):
			idx = 3
		case strings.HasPrefix(k, "rankHealth"),
			strings.HasPrefix(k, "rankArmour"),
			strings.HasPrefix(k, "rankArmor"),
			strings.HasPrefix(k, "rankBaseSpeed"),
			strings.HasPrefix(k, "rankModelScale"),
			strings.HasPrefix(k, "rankROF"),
			strings.HasPrefix(k, "rankSTM"),
			strings.HasPrefix(k, "rankDmg"),
			strings.HasPrefix(k, "rankKb"),
			strings.HasPrefix(k, "rankHack"):
			idx = 0
		}
		groups[idx].keys = append(groups[idx].keys, k)
	}
	for i := range groups {
		sort.Strings(groups[i].keys)
	}

	for _, g := range groups {
		if len(g.keys) == 0 {
			continue
		}
		header := widget.NewLabelWithStyle(g.name,
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		p.rankAttrContainer.Add(header)
		for _, key := range g.keys {
			k := key
			val := p.editor.character.RankAttributes[k]

			entry := NewInputEntry()
			entry.SetText(val)
			entry.OnChanged = func(s string) {
				p.editor.character.RankAttributes[k] = s
				p.editor.markDirty()
			}

			delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
				delete(p.editor.character.RankAttributes, k)
				p.refreshRankAttributes()
				p.editor.markDirty()
			})

			label := widget.NewLabel(k)
			label.TextStyle = fyne.TextStyle{Monospace: true}

			row := container.NewBorder(nil, nil,
				container.NewGridWrap(fyne.NewSize(220, 28), label),
				delBtn,
				entry,
			)
			p.rankAttrContainer.Add(row)
		}
	}
	p.rankAttrContainer.Refresh()
}

func (p *PointBuyUI) GetContent() fyne.CanvasObject { return p.container }

// UpdateUI is called after a file loads. Repopulates every slot +
// archetype header and redraws the tab strip to match HasCustomSpec.
func (p *PointBuyUI) UpdateUI() {
	p.customBuildCheck.SetChecked(p.editor.character.IsCustomBuild == 1)
	if p.editor.character.MBPoints > 0 {
		p.mbPointsEntry.SetText(strconv.Itoa(p.editor.character.MBPoints))
	} else {
		p.mbPointsEntry.SetText("")
	}

	archetypeCount := p.editor.character.HasCustomSpec
	if archetypeCount < 1 {
		archetypeCount = 1
	}
	if archetypeCount > maxArchetypes {
		archetypeCount = maxArchetypes
	}
	// fromUser=false so loading a file doesn't mark it dirty.
	p.selectArchetypeCount(archetypeCount, false)

	for i := 0; i < maxTotalSlots; i++ {
		skill := p.editor.character.CustomSkills[i]
		name := p.editor.character.CustomNames[i]
		ranks := p.editor.character.CustomRanks[i]
		descs := p.editor.character.CustomDescs[i]

		mode := detectSlotMode(skill, ranks)
		p.slotModes[i].SetSelected(mode)
		p.slotSkills[i].SetSelected(skill)
		p.slotNames[i].SetText(name)
		p.slotRanks[i].SetText(ranks)
		p.slotDescs[i].SetText(descs)

		p.refreshSlotLayout(i)
		p.refreshSlotIcon(i)
		p.refreshSlotMaxCost(i)
	}

	p.rebuildArchetypeTabs()
	p.refreshBudget()
	p.refreshRankAttributes()
	if p.simulator != nil {
		p.simulator.Refresh()
	}
}

// detectSlotMode returns the right UI mode for a stored slot.
func detectSlotMode(skill, ranks string) string {
	if skill == "" {
		return "Empty"
	}
	if skill == "MB_ATT_INVALID" && strings.TrimSpace(ranks) == "-1" {
		return "Header"
	}
	return "Skill"
}

var _ = layout.NewSpacer
