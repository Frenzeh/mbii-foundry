package main

// Point Buy editor — models the MBII Legends 2.0 c_att_skill_N /
// c_att_names_N / c_att_ranks_N slot layout the way players see it
// in the loadout menu:
//
//   ┌──── Budget ──────────────────────────────────────────────┐
//   │ [x] Custom Build   mbPoints [100]   max-spend: 82 / 100  │
//   ├──────────────────────────────────────────────────────────┤
//   │ 0  [Header]   -Weapons-                                  │
//   │ 1  [Skill]    [🔫] MB_ATT_PISTOL    "Blaster Pistol:"    │
//   │                                     costs: 0, 4, 10      │
//   │ 2  [Skill]    [🔫] MB_ATT_BLASTER   "E-11 Blaster:"      │
//   │                                     costs: 5, 7, 9       │
//   │ 3  [Header]   -Abilities-                                │
//   │ 4  [Skill]    [🛡] MB_ATT_CCTRAINING "Close Combat:"     │
//   │                                     costs: 4, 2          │
//   │ ...                                                       │
//   │ 14 [Empty]                                                │
//   └──────────────────────────────────────────────────────────┘
//
// Each slot is one of three modes:
//   - Empty  : no skill, no name, no ranks — collapsed row
//   - Header : MB_ATT_INVALID + ranks "-1" — styled as section divider
//   - Skill  : real MB_ATT_ + name + rank costs array
//
// Header vs Skill is derived from the stored data (not a separate
// field), which matches the game's parsing and keeps the save format
// untouched. Authors can flip modes by editing the dropdown / rank
// value — the UI updates the visual layout accordingly.
//
// Budget tracker shows max-spend (sum of all rank costs across all
// slots) vs mbPoints. This is a *max* not an active count, since the
// in-game cost is what a player picks at runtime — but showing the
// ceiling gives authors an immediate check: if max-spend > mbPoints,
// the class is under-constrained (a player can't actually buy every
// upgrade); if max-spend < mbPoints × 0.5, it's over-budgeted (too
// many points for too few upgrades).

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

const pointBuySlots = 15 // c_att_skill_0 through c_att_skill_14

// KnownRankAttributes lists the `rank*` field names from Legends 2.0's
// expanded system — used by the "Add Rank Attribute" picker below.
// Sourced from the wiki's Legends 2.0 point-buy guide.
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

	// Header controls.
	customBuildCheck *widget.Check
	mbPointsEntry    *widget.Entry
	budgetLabel      *widget.Label

	// Per-slot state. slotMode tracks which of the three visual
	// modes is showing (empty/header/skill) so the row repaints
	// without losing user state when the data-driven mode flips.
	slotRows     []*fyne.Container
	slotModes    []*widget.Select
	slotSkills   []*widget.Select // MB_ATT_* picker (skill mode)
	slotIcons    []*widget.Icon   // icon beside the skill select
	slotNames    []*widget.Entry  // display name (both header + skill)
	slotRanks    []*widget.Entry  // rank cost CSV (skill mode)
	slotMaxCost  []*widget.Label  // derived max-cost per slot

	// Extra rank* attributes (Legends 2.0 stat modifiers).
	rankAttrContainer *fyne.Container
}

func NewPointBuyUI(editor *MBCHEditor) *PointBuyUI {
	p := &PointBuyUI{editor: editor}
	p.createUI()
	return p
}

func (p *PointBuyUI) createUI() {
	// --- Header: custom-build toggle + budget + live max-spend ---
	p.customBuildCheck = widget.NewCheck("Custom Build enabled", func(on bool) {
		if on {
			p.editor.character.IsCustomBuild = 1
		} else {
			p.editor.character.IsCustomBuild = 0
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
		p.editor.markDirty()
		p.refreshBudget()
	}

	p.budgetLabel = widget.NewLabel("max spend: 0 / 0")

	headerForm := widget.NewForm(
		widget.NewFormItem("Custom Build", p.customBuildCheck),
		widget.NewFormItem("mbPoints", p.mbPointsEntry),
		widget.NewFormItem("Budget check", p.budgetLabel),
	)

	// --- Slot list: one row per c_att_skill_N, 15 total ---
	slotBox := container.NewVBox()
	p.slotRows = make([]*fyne.Container, pointBuySlots)
	p.slotModes = make([]*widget.Select, pointBuySlots)
	p.slotSkills = make([]*widget.Select, pointBuySlots)
	p.slotIcons = make([]*widget.Icon, pointBuySlots)
	p.slotNames = make([]*widget.Entry, pointBuySlots)
	p.slotRanks = make([]*widget.Entry, pointBuySlots)
	p.slotMaxCost = make([]*widget.Label, pointBuySlots)

	attrOptions := pointBuyAttrOptions()
	for i := 0; i < pointBuySlots; i++ {
		p.slotRows[i] = p.buildSlotRow(i, attrOptions)
		slotBox.Add(p.slotRows[i])
	}

	slotScroll := container.NewVScroll(slotBox)
	slotScroll.SetMinSize(fyne.NewSize(0, 400))

	// --- Rank attributes (Legends 2.0 stat modifiers) ---
	p.rankAttrContainer = container.NewVBox()

	addRankAttrBtn := widget.NewButtonWithIcon("Add Rank Modifier", theme.ContentAddIcon(), func() {
		p.showAddRankAttrDialog()
	})

	p.container = container.NewVBox(
		widget.NewCard("Point Buy Budget",
			"Toggle Custom Build, set total mbPoints, and watch the max-spend ceiling.",
			container.NewPadded(headerForm),
		),
		widget.NewCard("Skill Slots",
			"15 slots mirror the in-game loadout menu. Pick an MB_ATT, name it, and list comma-separated per-rank costs. Use \"Header\" mode for section dividers.",
			slotScroll,
		),
		widget.NewCard("Rank Modifiers",
			"Per-rank stat overrides from Legends 2.0 (rankHealth, rankAP, rankROF, …). Values are comma-separated per rank.",
			container.NewVBox(addRankAttrBtn, p.rankAttrContainer),
		),
	)
}

// buildSlotRow renders one c_att_skill_N slot. Mode selector at the
// left flips between Empty / Header / Skill layouts; the right side
// swaps controls accordingly.
func (p *PointBuyUI) buildSlotRow(i int, attrOptions []string) *fyne.Container {
	slotLabel := canvas.NewText(fmt.Sprintf("%d", i), theme.PlaceHolderColor())
	slotLabel.TextSize = SizeSmall
	slotLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	mode := widget.NewSelect([]string{"Empty", "Header", "Skill"}, nil)
	mode.PlaceHolder = "Empty"
	p.slotModes[i] = mode

	skillSel := widget.NewSelect(attrOptions, nil)
	skillSel.PlaceHolder = "MB_ATT_..."
	p.slotSkills[i] = skillSel

	iconW := widget.NewIcon(theme.FileImageIcon())
	p.slotIcons[i] = iconW

	nameEntry := NewInputEntry()
	nameEntry.SetPlaceHolder("Display name (e.g. \"Blaster Pistol:\")")
	p.slotNames[i] = nameEntry

	rankEntry := NewInputEntry()
	rankEntry.SetPlaceHolder("0,4,10")
	p.slotRanks[i] = rankEntry

	maxCost := widget.NewLabel("")
	maxCost.TextStyle = fyne.TextStyle{Italic: true}
	p.slotMaxCost[i] = maxCost

	// --- Wire callbacks. All changes flow into the backing character
	// struct + mark dirty + recalc budget. Mode changes also rewrite
	// the underlying strings so the save format stays consistent
	// (e.g. switching to Header sets skill=MB_ATT_INVALID, ranks="-1").
	mode.OnChanged = func(s string) {
		switch s {
		case "Empty":
			p.editor.character.CustomSkills[i] = ""
			p.editor.character.CustomNames[i] = ""
			p.editor.character.CustomRanks[i] = ""
			skillSel.SetSelected("")
			nameEntry.SetText("")
			rankEntry.SetText("")
		case "Header":
			p.editor.character.CustomSkills[i] = "MB_ATT_INVALID"
			if p.editor.character.CustomNames[i] == "" {
				p.editor.character.CustomNames[i] = "-Section-"
				nameEntry.SetText("-Section-")
			}
			p.editor.character.CustomRanks[i] = "-1"
			skillSel.SetSelected("MB_ATT_INVALID")
			rankEntry.SetText("-1")
		case "Skill":
			if p.editor.character.CustomSkills[i] == "MB_ATT_INVALID" ||
				p.editor.character.CustomSkills[i] == "" {
				p.editor.character.CustomSkills[i] = ""
				skillSel.SetSelected("")
			}
			if p.editor.character.CustomRanks[i] == "-1" {
				p.editor.character.CustomRanks[i] = ""
				rankEntry.SetText("")
			}
		}
		p.refreshSlotLayout(i)
		p.editor.markDirty()
		p.refreshBudget()
	}

	skillSel.OnChanged = func(s string) {
		p.editor.character.CustomSkills[i] = s
		p.refreshSlotIcon(i)
		p.editor.markDirty()
	}

	nameEntry.OnChanged = func(s string) {
		p.editor.character.CustomNames[i] = s
		p.editor.markDirty()
	}

	rankEntry.OnChanged = func(s string) {
		p.editor.character.CustomRanks[i] = s
		p.refreshSlotMaxCost(i)
		p.refreshBudget()
		p.editor.markDirty()
	}

	// Layout — slot number and mode selector fixed-width on the left,
	// the editable controls fill the rest of the row. Stack in a
	// Border with a subtle bottom separator so rows read as distinct.
	controls := container.NewVBox()
	controls.Add(container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(28, 28), iconW),
		maxCost,
		skillSel,
	))
	controls.Add(nameEntry)
	controls.Add(rankEntry)

	left := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(24, 24),
			container.NewCenter(slotLabel)),
		container.NewGridWrap(fyne.NewSize(90, 36), mode),
	)

	row := container.NewBorder(nil, widget.NewSeparator(),
		left, nil, controls)
	return row
}

// refreshSlotLayout hides/shows controls in a row based on its mode.
// Called after mode changes or during UpdateUI.
func (p *PointBuyUI) refreshSlotLayout(i int) {
	mode := p.slotModes[i].Selected
	switch mode {
	case "Empty":
		p.slotSkills[i].Hide()
		p.slotIcons[i].Hide()
		p.slotNames[i].Hide()
		p.slotRanks[i].Hide()
		p.slotMaxCost[i].Hide()
	case "Header":
		p.slotSkills[i].Hide()
		p.slotIcons[i].Hide()
		p.slotNames[i].Show()
		p.slotRanks[i].Hide()
		p.slotMaxCost[i].Hide()
	case "Skill":
		p.slotSkills[i].Show()
		p.slotIcons[i].Show()
		p.slotNames[i].Show()
		p.slotRanks[i].Show()
		p.slotMaxCost[i].Show()
	}
	p.slotRows[i].Refresh()
}

// refreshSlotIcon pulls the embedded game icon for the current MB_ATT
// and paints it next to the skill picker. Falls back to a theme icon
// when the attribute has no art (most do via our alias table).
func (p *PointBuyUI) refreshSlotIcon(i int) {
	skill := p.editor.character.CustomSkills[i]
	if skill == "" || skill == "MB_ATT_INVALID" {
		p.slotIcons[i].SetResource(theme.FileImageIcon())
		return
	}
	if p.editor.iconResolver == nil || p.editor.assetBrowser == nil {
		p.slotIcons[i].SetResource(theme.FileImageIcon())
		return
	}
	path := p.editor.iconResolver.ResolveAttributeIcon(skill)
	if res := p.editor.assetBrowser.LoadIconResource(path); res != nil {
		p.slotIcons[i].SetResource(res)
		return
	}
	p.slotIcons[i].SetResource(theme.FileImageIcon())
}

// refreshSlotMaxCost computes sum(rank_costs) and shows it next to
// the slot. Gives the author a ceiling at a glance.
func (p *PointBuyUI) refreshSlotMaxCost(i int) {
	total := parseRankCostSum(p.editor.character.CustomRanks[i])
	if total <= 0 {
		p.slotMaxCost[i].SetText("")
	} else {
		p.slotMaxCost[i].SetText(fmt.Sprintf("max %d", total))
	}
}

// refreshBudget re-sums every slot's rank costs and writes the total
// next to mbPoints. Over-budget (max > mbPoints) is normal; the
// number just tells the author whether the class is *spendable* at
// all. Highlighted in accent color when the ceiling is below mbPoints
// (probably under-budgeted).
func (p *PointBuyUI) refreshBudget() {
	total := 0
	for i := 0; i < pointBuySlots; i++ {
		total += parseRankCostSum(p.editor.character.CustomRanks[i])
	}
	target := p.editor.character.MBPoints
	msg := fmt.Sprintf("max spend: %d / %d", total, target)
	if target > 0 && total < target/2 {
		msg += " · consider adding more purchasable skills"
	} else if target > 0 && total > target*3 {
		msg += " · costs far exceed budget — consider lowering"
	}
	p.budgetLabel.SetText(msg)
}

// parseRankCostSum parses a CSV of rank costs and returns the sum.
// Header rows ("-1") return 0. Malformed entries skip silently so
// the budget label stays useful during active typing.
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

// pointBuyAttrOptions returns the list of MB_ATT_* IDs available for
// the skill dropdown. Includes MB_ATT_INVALID (for Header rows that
// an author toggles via the mode picker). Filtered hidden list so
// non-live content doesn't leak into new point-buy builds.
func pointBuyAttrOptions() []string {
	opts := []string{"MB_ATT_INVALID"}
	attrs := GetAttributes() // already filters Hidden==true
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
	if p.editor.character.RankAttributes == nil {
		return
	}
	// Sort keys for a stable display order — Go map iteration is
	// random, which otherwise made the list bounce on every refresh.
	keys := make([]string, 0, len(p.editor.character.RankAttributes))
	for k := range p.editor.character.RankAttributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
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
	p.rankAttrContainer.Refresh()
}

func (p *PointBuyUI) GetContent() fyne.CanvasObject { return p.container }

// UpdateUI is called after a file loads (updateUI in mbch_editor).
// Repopulates every slot, detecting its mode from the stored values.
func (p *PointBuyUI) UpdateUI() {
	p.customBuildCheck.SetChecked(p.editor.character.IsCustomBuild == 1)
	if p.editor.character.MBPoints > 0 {
		p.mbPointsEntry.SetText(strconv.Itoa(p.editor.character.MBPoints))
	} else {
		p.mbPointsEntry.SetText("")
	}

	for i := 0; i < pointBuySlots; i++ {
		skill := p.editor.character.CustomSkills[i]
		name := p.editor.character.CustomNames[i]
		ranks := p.editor.character.CustomRanks[i]

		mode := detectSlotMode(skill, ranks)
		p.slotModes[i].SetSelected(mode)

		p.slotSkills[i].SetSelected(skill)
		p.slotNames[i].SetText(name)
		p.slotRanks[i].SetText(ranks)

		p.refreshSlotLayout(i)
		p.refreshSlotIcon(i)
		p.refreshSlotMaxCost(i)
	}
	p.refreshBudget()
	p.refreshRankAttributes()
}

// detectSlotMode returns the right UI mode for a stored slot. Empty
// rows have no skill set; Header rows carry MB_ATT_INVALID + "-1"
// ranks (matching how the game recognizes section dividers);
// anything else is a normal Skill row.
func detectSlotMode(skill, ranks string) string {
	if skill == "" {
		return "Empty"
	}
	if skill == "MB_ATT_INVALID" && strings.TrimSpace(ranks) == "-1" {
		return "Header"
	}
	return "Skill"
}

// _ keeps the layout import alive if we later re-add center-wrapped
// content; current usage is routed via container.NewCenter.
var _ = layout.NewSpacer
