package main

// Point Buy simulator — lets the author test-drive their own budget
// configuration the way a real player would experience it in-game.
// Each slot in the active archetype renders its rank costs as
// clickable pills; clicking a pill "purchases" that rank and
// deducts its cost from the live budget. Unaffordable pills grey
// out. A Reset button clears the simulation.
//
// The simulator reads from the same MBCHCharacter as the editor but
// never writes back — selections are a throwaway preview. Authors
// use this to answer "can a real player actually build a balanced
// character with these costs and this mbPoints ceiling?"
//
// Renders as a second AppTab next to the editor's "Edit" view,
// sharing the PointBuyUI struct's data. The simulator pane is
// rebuilt whenever the underlying data changes (ranks, names,
// spec count) so the preview always reflects the current edit.

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// PointBuySimulator renders the click-to-buy preview. One instance
// per PointBuyUI (owned by the parent); it looks up slot data each
// time it rebuilds.
type PointBuySimulator struct {
	owner *PointBuyUI

	container *fyne.Container

	budgetLabel *widget.Label
	archetype   *widget.Select
	resetBtn    *widget.Button

	slotBox *fyne.Container

	// State: which rank the user has "bought" per slot. 0 = nothing
	// purchased, 1 = first rank bought, etc. Indexed by global slot.
	purchased map[int]int
	// Which archetype the simulator is currently previewing.
	activeSpec int
}

func NewPointBuySimulator(owner *PointBuyUI) *PointBuySimulator {
	s := &PointBuySimulator{
		owner:     owner,
		purchased: map[int]int{},
	}
	s.createUI()
	return s
}

func (s *PointBuySimulator) createUI() {
	s.budgetLabel = widget.NewLabel("")
	s.budgetLabel.TextStyle = fyne.TextStyle{Bold: true}

	s.archetype = widget.NewSelect(nil, func(string) {
		s.activeSpec = s.selectedSpecIndex()
		s.purchased = map[int]int{}
		s.rebuild()
	})

	s.resetBtn = widget.NewButtonWithIcon("Reset build", theme.ViewRefreshIcon(), func() {
		s.purchased = map[int]int{}
		s.rebuild()
	})

	s.slotBox = container.NewVBox()

	header := container.NewHBox(
		s.archetype,
		s.resetBtn,
		s.budgetLabel,
	)
	scroll := container.NewVScroll(s.slotBox)
	scroll.SetMinSize(fyne.NewSize(0, 420))

	s.container = container.NewBorder(header, nil, nil, nil, scroll)
}

// GetContent is the Fyne handle for embedding the simulator in a tab.
func (s *PointBuySimulator) GetContent() fyne.CanvasObject {
	return s.container
}

// Refresh tears down and rebuilds the simulator pane. Called by the
// parent PointBuyUI whenever state that affects the simulation
// changes (archetype count, slot costs, mbPoints, etc.).
func (s *PointBuySimulator) Refresh() {
	s.rebuildSpecOptions()
	s.rebuild()
}

// rebuildSpecOptions repopulates the archetype dropdown so it matches
// the current HasCustomSpec. Kept separate from rebuild() because the
// options only change when the user flips the archetype count.
func (s *PointBuySimulator) rebuildSpecOptions() {
	ch := s.owner.editor.character
	count := ch.HasCustomSpec
	if count < 2 {
		count = 1
	}
	if count > maxArchetypes {
		count = maxArchetypes
	}

	opts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		name := ch.CustomSpecNames[i]
		if name == "" {
			name = fmt.Sprintf("Spec %d", i+1)
		}
		opts = append(opts, name)
	}
	s.archetype.Options = opts

	if count == 1 {
		s.archetype.Hide()
	} else {
		s.archetype.Show()
	}

	if s.activeSpec >= count {
		s.activeSpec = 0
	}
	if len(opts) > 0 {
		s.archetype.SetSelected(opts[s.activeSpec])
	}
}

// selectedSpecIndex returns the current archetype index based on the
// dropdown's selection. Defaults to 0 if none / unknown.
func (s *PointBuySimulator) selectedSpecIndex() int {
	sel := s.archetype.Selected
	for i, opt := range s.archetype.Options {
		if opt == sel {
			return i
		}
	}
	return 0
}

// rebuild renders every slot row for the active archetype. Called
// on archetype switch + reset.
func (s *PointBuySimulator) rebuild() {
	s.slotBox.Objects = nil

	ch := s.owner.editor.character
	base := s.activeSpec * slotsPerArchetype
	for i := 0; i < slotsPerArchetype; i++ {
		idx := base + i
		row := s.buildSlotRow(idx, ch)
		if row != nil {
			s.slotBox.Add(row)
		}
	}
	s.slotBox.Refresh()
	s.refreshBudget()
}

// buildSlotRow produces a single simulator row. Returns nil for
// Empty slots so the layout doesn't show ghost rows.
func (s *PointBuySimulator) buildSlotRow(globalIdx int, ch *parsers.MBCHCharacter) fyne.CanvasObject {
	skill := ch.CustomSkills[globalIdx]
	name := ch.CustomNames[globalIdx]
	ranks := ch.CustomRanks[globalIdx]
	descs := ch.CustomDescs[globalIdx]

	if skill == "" {
		return nil // Empty slot — skip
	}

	// Header row: render as a visually-distinct section divider.
	if skill == "MB_ATT_INVALID" && strings.TrimSpace(ranks) == "-1" {
		headerText := name
		if headerText == "" {
			headerText = "-Section-"
		}
		hdr := canvas.NewText(headerText, CurrentThemeColor)
		hdr.TextSize = SizeSubtitle
		hdr.TextStyle = fyne.TextStyle{Bold: true}
		return container.NewPadded(hdr)
	}

	// Skill row: name on the left, rank pills on the right.
	nameLabel := widget.NewLabel(name)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	if name == "" {
		nameLabel.SetText(skill)
	}

	costs := parseRankCostList(ranks)
	currentRank := s.purchased[globalIdx]

	pills := container.NewHBox()
	// "Off" pill first — always available, sets rank to 0.
	pills.Add(s.buildRankPill(globalIdx, 0, 0, currentRank))
	for i, cost := range costs {
		pills.Add(s.buildRankPill(globalIdx, i+1, cost, currentRank))
	}

	body := container.NewVBox()
	body.Add(container.NewBorder(nil, nil, nameLabel, nil, pills))
	if descs != "" {
		desc := canvas.NewText(descs, theme.PlaceHolderColor())
		desc.TextSize = SizeSmall
		desc.TextStyle = fyne.TextStyle{Italic: true}
		body.Add(desc)
	}
	body.Add(widget.NewSeparator())
	return body
}

// buildRankPill constructs one clickable rank button for a slot.
// "rank" is 0..len(costs) (0 = Off); "cost" is this rank's cost
// (0 for Off). Unaffordable pills are disabled.
func (s *PointBuySimulator) buildRankPill(globalIdx, rank, cost, currentRank int) *widget.Button {
	var label string
	switch rank {
	case 0:
		label = "Off"
	default:
		if cost == 0 {
			label = fmt.Sprintf("R%d · free", rank)
		} else {
			label = fmt.Sprintf("R%d · %d", rank, cost)
		}
	}

	btn := widget.NewButton(label, func() {
		s.purchased[globalIdx] = rank
		s.rebuild()
	})

	if rank == currentRank {
		btn.Importance = widget.HighImportance
	} else {
		btn.Importance = widget.MediumImportance
	}

	// Disable rank pills that would push the build over budget —
	// gives the author immediate feedback about whether their
	// mbPoints ceiling actually lets a player buy the upgrade.
	if rank > 0 {
		ch := s.owner.editor.character
		tentative := s.purchasedCostExcluding(globalIdx) + s.costForRank(globalIdx, rank, ch)
		if tentative > ch.MBPoints {
			btn.Disable()
		}
	}

	return btn
}

// purchasedCostExcluding sums the purchased cost of every slot
// except the given one (used to determine if a prospective rank
// purchase would fit in the remaining budget).
func (s *PointBuySimulator) purchasedCostExcluding(skipIdx int) int {
	ch := s.owner.editor.character
	total := 0
	for idx, rank := range s.purchased {
		if idx == skipIdx {
			continue
		}
		total += s.costForRank(idx, rank, ch)
	}
	return total
}

// costForRank returns the cost for purchasing `rank` of slot idx.
// Rank 0 is free by definition; rank N reads the N-th entry of the
// slot's comma-separated rank-cost list (1-indexed: rank 1 → costs[0]).
func (s *PointBuySimulator) costForRank(globalIdx, rank int, ch *parsers.MBCHCharacter) int {
	if rank <= 0 {
		return 0
	}
	costs := parseRankCostList(ch.CustomRanks[globalIdx])
	if rank-1 >= len(costs) {
		return 0
	}
	return costs[rank-1]
}

// refreshBudget recomputes spent/total and updates the header label.
func (s *PointBuySimulator) refreshBudget() {
	spent := s.purchasedCostExcluding(-1) // nothing skipped
	target := s.owner.editor.character.MBPoints
	remaining := target - spent

	s.budgetLabel.SetText(fmt.Sprintf("Spent: %d / %d  ·  Remaining: %d", spent, target, remaining))
}

// parseRankCostList parses a CSV into an int slice. "-1" or empty
// return empty (header / empty slots have no rank costs). Negative
// and malformed entries are skipped rather than rejecting the whole
// list — matches the tolerant parsing the editor does.
func parseRankCostList(csv string) []int {
	csv = strings.TrimSpace(csv)
	if csv == "" || csv == "-1" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(csv, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

