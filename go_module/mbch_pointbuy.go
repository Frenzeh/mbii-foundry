package main

// Point Buy editor — exposes the parser's bounded custom-build layout.
// Runtime purchase and specialization mechanics remain Unverified.
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
// Archetype tabs edit the bounded slot windows. The budget tracker is an
// authoring aid over configured rank costs; it is not an engine-parity claim.

import (
	"fmt"
	"slices"
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

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// No local consts needed, using limits from parsers package

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
	editor  *MBCHEditor
	loading bool

	container   *fyne.Container
	pageWheel   wheelRoute
	windowWheel [parsers.PointbuyMaxArchetypes]wheelRoute

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

	// Per-slot widgets, indexed globally (0..parsers.PointbuyMaxTotalSlots-1). Each
	// archetype's tab owns slots [i*15, (i+1)*15).
	slotRows      []*fyne.Container
	slotModes     []*widget.Select
	slotSkills    []*widget.Select
	slotIcons     []*canvas.Image // raster-friendly; widget.Icon renders at theme-icon size and loses raster detail
	slotNames     []*slotEntry
	slotRanks     []*slotEntry
	slotDescs     []*slotEntry
	slotMaxCost   []*widget.Label
	slotForms     []*widget.Form       // one form per slot — rebuilt items drive mode-based visibility
	slotFormItems [][]*widget.FormItem // [slot][0..3] = Skill / Name / Costs / Description items
	slotFormHosts []*fyne.Container    // stable owner while a mode change swaps the form
	slotHelp      []*widget.Label      // wrapped help for the visible slot fields

	// Per-archetype spec header (name + icon + description) widgets.
	// Description was previously dropped — engine parses customSpecDesc_N
	// (bg_saga.c:2375) but Foundry's older write path skipped it.
	specNameEntries [parsers.PointbuyMaxArchetypes]*slotEntry
	specIconEntries [parsers.PointbuyMaxArchetypes]*slotEntry
	specDescEntries [parsers.PointbuyMaxArchetypes]*slotEntry

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
		if p.loading || p.editor.loading {
			return
		}
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
		if p.loading || p.editor.loading {
			return
		}
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

	p.budgetLabel = widget.NewLabel("max cumulative configured costs: 0 / 0 · runtime behavior Unverified")
	p.budgetLabel.Wrapping = fyne.TextWrapWord

	// Archetype count buttons. GridWrap forces all three to the same
	// width so "1 / 2 / 3" reads as a coherent toggle group, not three
	// differently-sized buttons.
	p.archetypeCountBtns = make([]*widget.Button, parsers.PointbuyMaxArchetypes)
	archetypeBtnRow := container.NewHBox()
	for i := 0; i < parsers.PointbuyMaxArchetypes; i++ {
		n := i + 1
		btn := widget.NewButton(strconv.Itoa(n), func() {
			p.selectArchetypeCount(n, true)
		})
		p.archetypeCountBtns[i] = btn
		archetypeBtnRow.Add(container.New(layout.NewGridWrapLayout(fyne.NewSize(44, 34)), btn))
	}

	headerFields := newFormColumns(2,
		NewFormSection("Custom Build",
			"If off, the class uses the classic fixed loadout instead of point-buy.",
			p.customBuildCheck),
		NewFormSection("mbPoints",
			"Configured budget for the local preview (e.g. 100); runtime purchase behavior is Unverified.",
			p.mbPointsEntry),
		NewFormSection("Archetypes",
			"Number of configured archetype slot windows.",
			archetypeBtnRow),
		NewFormSection("Budget check",
			"Sums configured costs through the selected rank, consistently with Simulate; runtime behavior is Unverified.",
			p.budgetLabel),
	)

	// --- Build all 45 slot widgets up front. Tabs only show the
	// slice relevant to each archetype; slots outside the active
	// archetype range stay unreferenced (but kept in memory so
	// toggling HasCustomSpec doesn't lose edits).
	attrOptions := pointBuyAttrOptions()
	p.slotRows = make([]*fyne.Container, parsers.PointbuyMaxTotalSlots)
	p.slotModes = make([]*widget.Select, parsers.PointbuyMaxTotalSlots)
	p.slotSkills = make([]*widget.Select, parsers.PointbuyMaxTotalSlots)
	p.slotIcons = make([]*canvas.Image, parsers.PointbuyMaxTotalSlots)
	p.slotNames = make([]*slotEntry, parsers.PointbuyMaxTotalSlots)
	p.slotRanks = make([]*slotEntry, parsers.PointbuyMaxTotalSlots)
	p.slotDescs = make([]*slotEntry, parsers.PointbuyMaxTotalSlots)
	p.slotMaxCost = make([]*widget.Label, parsers.PointbuyMaxTotalSlots)
	p.slotForms = make([]*widget.Form, parsers.PointbuyMaxTotalSlots)
	p.slotFormItems = make([][]*widget.FormItem, parsers.PointbuyMaxTotalSlots)
	p.slotFormHosts = make([]*fyne.Container, parsers.PointbuyMaxTotalSlots)
	p.slotHelp = make([]*widget.Label, parsers.PointbuyMaxTotalSlots)
	for i := 0; i < parsers.PointbuyMaxTotalSlots; i++ {
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
	p.simulator.container = buildResponsivePointBuySimulatorPane(p.simulator)
	p.simulator.resetBtn.OnTapped = func() {
		p.simulator.purchased = map[int]int{}
		p.refreshResponsiveSimulator()
	}

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
			p.refreshResponsiveSimulator()
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
		NewFormSection("Point Buy Budget",
			"Toggle Custom Build, set total mbPoints, and pick how many archetypes the class offers (1–3).",
			headerFields),
		NewFormSection("Custom Multipliers",
			"Point-buy multipliers — pick these from a slot's skill dropdown.",
			multHint),
		NewFormSection("Build & Simulator",
			"Edit skills here, then flip to Simulate to try the build.",
			p.modeTabs),
		NewFormSection("Rank Modifiers",
			"Per-rank stat overrides (rankHealth, rankAP, rankROF, …). Values are comma-separated per rank. Shared across all archetypes.",
			container.NewVBox(addRankAttrBtn, p.rankAttrContainer)),
	)
}

// buildResponsivePointBuySimulatorPane keeps the simulator's existing state and
// callbacks but lets its budget and archetype controls stack. The simulator's
// original left/right border made the hidden tab establish a wide minimum for
// the entire Point Buy AppTabs, even while authors were editing slots.
func buildResponsivePointBuySimulatorPane(simulator *PointBuySimulator) *fyne.Container {
	budgetHint := widget.NewLabelWithStyle(
		"Configured-cost preview only; runtime purchase behavior is Unverified.",
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	budgetHint.Wrapping = fyne.TextWrapWord
	budgetBlock := container.NewVBox(
		container.NewHBox(simulator.budgetSpent, simulator.budgetTotal),
		simulator.budgetRemaining,
		simulator.budgetFill,
	)

	// Reuse the simulator's dynamic button host. A width-first layout keeps
	// one, two, or three archetype choices reachable instead of pinning them
	// into one horizontal minimum.
	simulator.archetypeBar = newFormColumns(parsers.PointbuyMaxArchetypes)
	archetypeBlock := container.NewVBox(
		simulator.archetypeLabel,
		simulator.archetypeBar,
		simulator.resetBtn,
	)

	top := newFormColumns(2,
		NewFormSection("Points Spent", "", container.NewVBox(budgetHint, budgetBlock)),
		NewFormSection("Archetype", "Switching archetypes or resetting clears only the local preview.", archetypeBlock),
	)
	scroll := container.NewVScroll(simulator.slotBox)
	scroll.SetMinSize(fyne.NewSize(0, 440))
	return container.NewBorder(
		container.NewVBox(container.NewPadded(top), widget.NewSeparator()),
		nil, nil, nil,
		scroll,
	)
}

// refreshResponsiveSimulator mirrors the simulator's configured-cost behavior
// while rendering its dynamic controls with wrapped, stacking containers. All
// purchases remain ephemeral and archetype changes still reset the preview.
func (p *PointBuyUI) refreshResponsiveSimulator() {
	simulator := p.simulator
	character := p.editor.character
	count := character.HasCustomSpec
	if count < 2 {
		count = 1
	}
	if count > parsers.PointbuyMaxArchetypes {
		count = parsers.PointbuyMaxArchetypes
	}
	if simulator.activeSpec >= count {
		simulator.activeSpec = 0
	}

	simulator.specNames = make([]string, count)
	simulator.archetypeBar.Objects = nil
	for i := 0; i < count; i++ {
		name := character.CustomSpecNames[i]
		if name == "" {
			name = fmt.Sprintf("Spec %d", i+1)
		}
		simulator.specNames[i] = name
		index := i
		button := widget.NewButton(name, func() {
			if simulator.activeSpec == index {
				return
			}
			simulator.activeSpec = index
			simulator.purchased = map[int]int{}
			p.refreshResponsiveSimulator()
		})
		if i == simulator.activeSpec {
			button.Importance = widget.HighImportance
		} else {
			button.Importance = widget.MediumImportance
		}
		simulator.archetypeBar.Add(button)
	}
	if count == 1 {
		simulator.archetypeLabel.Hide()
		simulator.archetypeBar.Hide()
	} else {
		simulator.archetypeLabel.Show()
		simulator.archetypeBar.Show()
	}
	simulator.archetypeBar.Refresh()

	simulator.slotBox.Objects = nil
	base := simulator.activeSpec * parsers.PointbuySlotsPerArchetype
	for i := 0; i < parsers.PointbuySlotsPerArchetype; i++ {
		if row := p.buildResponsiveSimulatorSlotCard(base + i); row != nil {
			simulator.slotBox.Add(row)
		}
	}
	simulator.slotBox.Refresh()
	simulator.refreshBudget()
}

func (p *PointBuyUI) buildResponsiveSimulatorSlotCard(globalIdx int) fyne.CanvasObject {
	simulator := p.simulator
	character := p.editor.character
	skill := character.CustomSkills[globalIdx]
	name := character.CustomNames[globalIdx]
	ranks := character.CustomRanks[globalIdx]
	description := character.CustomDescs[globalIdx]
	if skill == "" {
		return nil
	}
	if skill == "MB_ATT_INVALID" && strings.TrimSpace(ranks) == "-1" {
		return buildHeaderStrip(name)
	}

	if name == "" {
		name = skill
	}
	title := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapWord
	title.SizeName = theme.SizeNameSubHeadingText
	text := container.NewVBox(title)
	if description != "" {
		desc := widget.NewLabelWithStyle(description, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
		desc.Wrapping = fyne.TextWrapWord
		desc.SizeName = theme.SizeNameCaptionText
		text.Add(desc)
	}
	identity := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(44, 44), simulator.buildSlotIcon(skill)),
		nil,
		text,
	)

	costs := parseRankCostList(ranks)
	currentRank := simulator.purchased[globalIdx]
	dots := NewRankDots(costs, currentRank,
		func(rank int) bool {
			tentative := simulator.purchasedCostExcluding(globalIdx) + simulator.costForRank(globalIdx, rank, character)
			return tentative <= character.MBPoints
		},
		func(newRank int) {
			if newRank < 0 {
				newRank = 0
			}
			if newRank > len(costs) {
				newRank = len(costs)
			}
			simulator.purchased[globalIdx] = newRank
			p.refreshResponsiveSimulator()
		},
	)

	background := canvas.NewRectangle(skillCardFill())
	background.StrokeColor = skillCardStroke()
	background.StrokeWidth = 1
	content := container.NewVBox(identity, dots)
	return container.NewPadded(container.NewStack(background, container.NewPadded(content)))
}

// buildSlotRow renders one c_att_skill_N slot (global index 0..44).
// The slot number shown in the UI is relative to the archetype
// (so archetype 2 slot 0 displays "0" even though its stored index
// is 15).
func (p *PointBuyUI) buildSlotRow(globalIdx int, attrOptions []string) *fyne.Container {
	displayIdx := globalIdx % parsers.PointbuySlotsPerArchetype
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

	wheel := &p.windowWheel[globalIdx/parsers.PointbuySlotsPerArchetype]
	nameEntry := NewSlotEntry(wheel)
	nameEntry.SetPlaceHolder("Display name (e.g. \"Blaster Pistol:\")")
	p.slotNames[globalIdx] = nameEntry

	rankEntry := NewSlotEntry(wheel)
	rankEntry.SetPlaceHolder("0,4,10")
	p.slotRanks[globalIdx] = rankEntry

	descEntry := NewSlotEntry(wheel)
	descEntry.SetPlaceHolder("Optional description (c_att_descs)")
	p.slotDescs[globalIdx] = descEntry

	maxCost := widget.NewLabel("")
	maxCost.TextStyle = fyne.TextStyle{Italic: true}
	maxCost.Wrapping = fyne.TextWrapWord
	p.slotMaxCost[globalIdx] = maxCost

	mode.OnChanged = func(s string) {
		if p.loading || p.editor.loading {
			return
		}
		p.loading = true
		p.refreshSlotLayout(globalIdx)
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
		p.loading = false
		p.refreshSlotMaxCost(globalIdx)
		p.customSkillsChanged()
	}

	skillSel.OnChanged = func(s string) {
		if p.loading || p.editor.loading {
			return
		}
		p.editor.character.CustomSkills[globalIdx] = s
		p.refreshSlotIcon(globalIdx)
		p.customSkillsChanged()
	}
	nameEntry.OnChanged = func(s string) {
		if p.loading || p.editor.loading {
			return
		}
		p.editor.character.CustomNames[globalIdx] = s
		p.customSkillsChanged()
	}
	rankEntry.OnChanged = func(s string) {
		if p.loading || p.editor.loading {
			return
		}
		p.editor.character.CustomRanks[globalIdx] = s
		p.refreshSlotMaxCost(globalIdx)
		p.customSkillsChanged()
	}
	descEntry.OnChanged = func(s string) {
		if p.loading || p.editor.loading {
			return
		}
		p.editor.character.CustomDescs[globalIdx] = s
		p.customSkillsChanged()
	}

	// Keep the live sum below the picker. Pinning it to the right of a long
	// attribute ID made the slot establish a wide minimum before its form
	// could stack for a narrow editor rail.
	skillRow := container.NewBorder(nil, nil,
		container.NewGridWrap(fyne.NewSize(28, 28), iconW),
		nil,
		skillSel,
	)
	skillControl := container.NewVBox(skillRow, maxCost)

	// Form captions stay immediately above their inputs. The complete field
	// help lives in the wrapped label below so it remains visible without
	// letting native unwrapped FormItem hints establish the outer width.
	// Keep each item and entry across mode changes. refreshSlotLayout changes
	// the form owner only when the visible item sequence changes, without
	// retaining stale form callbacks.
	skillItem := widget.NewFormItem("Skill", skillControl)
	nameItem := widget.NewFormItem("Name", nameEntry)
	costsItem := widget.NewFormItem("Costs", rankEntry)
	descItem := widget.NewFormItem("Description", descEntry)

	form := widget.NewForm(skillItem, nameItem, costsItem, descItem)
	form.Orientation = widget.Vertical
	p.slotForms[globalIdx] = form
	p.slotFormItems[globalIdx] = []*widget.FormItem{skillItem, nameItem, costsItem, descItem}
	formHost := container.NewStack(form)
	p.slotFormHosts[globalIdx] = formHost
	help := widget.NewLabel("")
	help.TextStyle = fyne.TextStyle{Italic: true}
	help.Wrapping = fyne.TextWrapWord
	help.Hide()
	p.slotHelp[globalIdx] = help

	left := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(24, 24), container.NewCenter(slotLabel)),
		container.NewGridWrap(fyne.NewSize(90, 36), mode),
	)

	// At roomy widths the selector and fields can share a row; at the 360px
	// boundary they stack before either side can force an outer scroller.
	fields := container.NewVBox(formHost, help)
	return container.NewVBox(newFormColumns(2, left, fields), widget.NewSeparator())
}

func (p *PointBuyUI) customSkillsChanged() {
	if p.editor.customSkillsUI != nil {
		p.editor.customSkillsUI.LoadFromCharacter(p.editor.character)
	}
	p.refreshBudget()
	p.editor.markDirty()
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
	if n > parsers.PointbuyMaxArchetypes {
		n = parsers.PointbuyMaxArchetypes
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
	p.rebuildArchetypeTabs()
	p.customSkillsChanged()
	if p.simulator != nil {
		p.refreshResponsiveSimulator()
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
	if count > parsers.PointbuyMaxArchetypes {
		count = parsers.PointbuyMaxArchetypes
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
		nameEntry := NewSlotEntry(&p.pageWheel)
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

		iconEntry := NewSlotEntry(&p.pageWheel)
		iconEntry.SetPlaceHolder("Icon path (e.g. gfx/menus/alpha/icon_weap_accuracy)")
		iconEntry.SetText(p.editor.character.CustomSpecIcons[spec])
		iconEntry.OnChanged = func(s string) {
			p.editor.character.CustomSpecIcons[spec] = s
			p.editor.markDirty()
		}
		p.specIconEntries[spec] = iconEntry

		descEntry := NewSlotEntry(&p.pageWheel)
		descEntry.SetPlaceHolder("Description (shown in-menu under the spec name)")
		descEntry.SetText(p.editor.character.CustomSpecDescs[spec])
		descEntry.OnChanged = func(s string) {
			p.editor.character.CustomSpecDescs[spec] = s
			p.editor.markDirty()
		}
		p.specDescEntries[spec] = descEntry

		header := widget.NewForm(
			widget.NewFormItem("Spec Name", nameEntry),
			widget.NewFormItem("Spec Icon", iconEntry),
			widget.NewFormItem("Spec Desc", descEntry),
		)
		header.Orientation = widget.Vertical
		content.Add(NewFormSection(
			fmt.Sprintf("Archetype %d details", spec+1),
			"The name becomes the tab title, the icon accepts a game asset path, and the description appears beneath the name in-menu.",
			header,
		))
		content.Add(widget.NewSeparator())
	}

	// Slot rows — 15 from the right window.
	base := spec * parsers.PointbuySlotsPerArchetype
	slotBox := container.NewVBox()
	for i := 0; i < parsers.PointbuySlotsPerArchetype; i++ {
		slotBox.Add(p.slotRows[base+i])
	}
	scroll := container.NewVScroll(slotBox)
	scroll.SetMinSize(fyne.NewSize(0, 420))
	p.windowWheel[spec].target = scroll
	content.Add(scroll)

	return content
}

// refreshSlotLayout shows only the current mode's fields. Structural changes
// replace the form owner, not the existing entry widgets; ordinary value
// refreshes retain both the form and its validation bindings.
//
//	Empty  → no fields at all
//	Header → just the Name entry (the section label text)
//	Skill  → all four: Skill, Name, Costs, Description
func (p *PointBuyUI) refreshSlotLayout(i int) {
	form := p.slotForms[i]
	items := p.slotFormItems[i]
	if form == nil || items == nil {
		return
	}
	var visible []*widget.FormItem
	switch p.slotModes[i].Selected {
	case "Header":
		visible = items[1:2]
		p.slotHelp[i].SetText("Name: section label shown in the in-game loadout menu.")
		p.slotHelp[i].Show()
	case "Skill":
		visible = items
		p.slotHelp[i].SetText("Skill: MB_ATT_* attribute granted by this row. Name: in-game loadout label. Costs: comma-separated per-rank costs; the local preview sums 0,4,10 to 14 through rank 3, while runtime behavior remains Unverified. Description: optional in-game rank tooltip.")
		p.slotHelp[i].Show()
	default:
		p.slotHelp[i].Hide()
	}
	if !slices.Equal(form.Items, visible) {
		// Fyne 2.8.2 can leave truncated entries bound to obsolete indices,
		// and its sequential detach/rebind can unbind a moved retained entry.
		// Retire all old ownership before a new form binds the shared entries.
		for _, item := range form.Items {
			if entry, ok := item.Widget.(fyne.Validatable); ok {
				entry.SetOnValidationChanged(nil)
			}
			if entry, ok := item.Widget.(fyne.Requireable); ok {
				entry.SetOnRequiredChanged(nil)
			}
		}
		oldForm := form
		oldForm.Items = nil
		oldForm.Refresh()
		form = widget.NewForm(visible...)
		form.Orientation = widget.Vertical
		p.slotForms[i] = form
		p.slotFormHosts[i].Remove(oldForm)
		p.slotFormHosts[i].Add(form)
	}
	form.Refresh()
	p.slotFormHosts[i].Refresh()
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

// refreshBudget recomputes the largest configured rank-cost sum among the
// active archetype windows. This is an editor consistency check only; engine
// purchase behavior remains Unverified.
func (p *PointBuyUI) refreshBudget() {
	specs := p.editor.character.HasCustomSpec
	if specs < 2 {
		specs = 1
	}
	if specs > parsers.PointbuyMaxArchetypes {
		specs = parsers.PointbuyMaxArchetypes
	}
	worst := 0
	for s := 0; s < specs; s++ {
		total := 0
		for i := 0; i < parsers.PointbuySlotsPerArchetype; i++ {
			idx := s*parsers.PointbuySlotsPerArchetype + i
			if detectSlotMode(p.editor.character.CustomSkills[idx], p.editor.character.CustomRanks[idx]) == "Skill" {
				total += parseRankCostSum(p.editor.character.CustomRanks[idx])
			}
		}
		if total > worst {
			worst = total
		}
	}
	target := p.editor.character.MBPoints
	p.budgetLabel.SetText(fmt.Sprintf("max cumulative configured costs: %d / %d · runtime behavior Unverified", worst, target))
}

// parseRankCostSum parses a CSV of rank costs and returns the sum.
// Header rows ("-1") return 0.
func parseRankCostSum(csv string) int {
	costs := parseRankCostList(csv)
	return totalCostOf(costs, len(costs))
}

// pointBuyAttrOptions returns the list of MB_ATT_* IDs available.
// Includes MB_ATT_INVALID for Header rows.
func pointBuyAttrOptions() []string {
	opts := []string{"MB_ATT_INVALID"}
	attrs := GetAttributes()
	ids := make([]string, 0, len(attrs))
	for _, a := range attrs {
		if isAttributeProper(a.ID) && a.ID != "MB_ATT_INVALID" {
			ids = append(ids, a.ID)
		}
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

			entry := NewSlotEntry(&p.pageWheel)
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
			label.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
			label.Wrapping = fyne.TextWrapBreak
			help := widget.NewLabelWithStyle("Comma-separated values, one configured value per rank.",
				fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
			help.Wrapping = fyne.TextWrapWord
			header := container.NewBorder(nil, nil, nil, delBtn, label)
			row := container.NewVBox(header, help, entry)
			p.rankAttrContainer.Add(row)
		}
	}
	p.rankAttrContainer.Refresh()
}

func (p *PointBuyUI) GetContent() fyne.CanvasObject { return p.container }

// UpdateUI is called after a file loads. Repopulates every slot +
// archetype header and redraws the tab strip to match HasCustomSpec.
func (p *PointBuyUI) UpdateUI() {
	wasLoading := p.loading
	p.loading = true
	defer func() { p.loading = wasLoading }()
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
	if archetypeCount > parsers.PointbuyMaxArchetypes {
		archetypeCount = parsers.PointbuyMaxArchetypes
	}
	// fromUser=false so loading a file doesn't mark it dirty.
	p.selectArchetypeCount(archetypeCount, false)

	attrOptions := pointBuyAttrOptions()
	for i := 0; i < parsers.PointbuyMaxTotalSlots; i++ {
		skill := p.editor.character.CustomSkills[i]
		name := p.editor.character.CustomNames[i]
		ranks := p.editor.character.CustomRanks[i]
		descs := p.editor.character.CustomDescs[i]

		mode := detectSlotMode(skill, ranks)
		p.slotModes[i].SetSelected(mode)
		p.refreshSlotLayout(i)
		setLoadedCatalogSelection(p.slotSkills[i], attrOptions, skill)
		p.slotNames[i].SetText(name)
		p.slotRanks[i].SetText(ranks)
		p.slotDescs[i].SetText(descs)

		p.refreshSlotIcon(i)
		p.refreshSlotMaxCost(i)
	}

	p.rebuildArchetypeTabs()
	p.refreshBudget()
	p.refreshRankAttributes()
	if p.simulator != nil {
		p.refreshResponsiveSimulator()
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
