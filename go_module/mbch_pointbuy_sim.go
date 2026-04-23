package main

// Point Buy simulator — mirrors MBII's in-game loadout menu so an
// author can preview exactly what a player would see + experience
// when picking a build. Layout:
//
//   ┌──────────────────────────────────────────────────────────┐
//   │                                                          │
//   │     Spec [Gunner ▾]        Budget  42 / 100              │
//   │                            Remaining: 58                 │
//   │                                  [Reset Build]           │
//   │                                                          │
//   ├──────────────────────────────────────────────────────────┤
//   │                                                          │
//   │                      — WEAPONS —                         │
//   │                                                          │
//   │   ┌──────────────────────────────────────────────────┐   │
//   │   │ [🔫]  Blaster Pistol                             │   │
//   │   │        Choose your level of mastery              │   │
//   │   │                                                  │   │
//   │   │              ● ● ○ ○   (r1, r2 owned)            │   │
//   │   │             free 4 10                            │   │
//   │   └──────────────────────────────────────────────────┘   │
//   │                                                          │
//   └──────────────────────────────────────────────────────────┘
//
// Unaffordable ranks disable visually; the current rank highlights
// in accent color; Off pill is always available. Header rows
// ("-Weapons-", "-Force Abilities-") render as centered accent
// dividers so sections read at a glance.
//
// Purchases are ephemeral — the simulator never writes back to the
// MBCH. Reset + archetype-switch both clear the build.

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

// PointBuySimulator renders the click-to-buy preview.
type PointBuySimulator struct {
	owner *PointBuyUI

	container *fyne.Container

	// Budget banner widgets.
	budgetSpent     *canvas.Text
	budgetTotal     *canvas.Text
	budgetRemaining *canvas.Text
	budgetFill      *canvas.Rectangle // colored strip whose width tracks spent/total

	// Archetype selector shown only on multi-spec classes. A horizontal
	// button row — one button per spec — so picking "Gunner / Scout /
	// Vanguard" feels like in-game class-variant picking rather than a
	// flat dropdown. Active spec renders HighImportance (accent fill).
	archetypeBar   *fyne.Container
	specNames      []string
	archetypeLabel *canvas.Text
	resetBtn       *widget.Button

	// Scroll area holding the skill cards.
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
	// Budget banner — MBII's in-game menu puts the points-remaining
	// count front-and-center; we do the same. Size + color give the
	// numbers the weight they deserve: this is the single most
	// important readout on this screen.
	s.budgetSpent = canvas.NewText("0", CurrentThemeColor)
	s.budgetSpent.TextSize = SizeHeading
	s.budgetSpent.TextStyle = fyne.TextStyle{Bold: true}

	s.budgetTotal = canvas.NewText("/ 0", theme.ForegroundColor())
	s.budgetTotal.TextSize = SizeHeading
	s.budgetTotal.TextStyle = fyne.TextStyle{Bold: true}

	s.budgetRemaining = canvas.NewText("0 remaining", theme.PlaceHolderColor())
	s.budgetRemaining.TextSize = SizeSmall

	// Progress strip: subtle horizontal bar matching accent color.
	// Width driven by rebuild's proportional-fill calc. Height is a
	// fixed 4px — big enough to read, small enough not to compete
	// with the number display above.
	s.budgetFill = canvas.NewRectangle(CurrentThemeColor)
	s.budgetFill.SetMinSize(fyne.NewSize(0, 4))

	budgetLabel := widget.NewLabel("Points Spent")
	budgetLabel.TextStyle = fyne.TextStyle{Bold: true}

	spendRow := container.NewHBox(s.budgetSpent, s.budgetTotal)
	budgetBlock := container.NewVBox(
		budgetLabel,
		spendRow,
		s.budgetRemaining,
		s.budgetFill,
	)

	// Archetype picker — only shows when the class has >1 spec. On
	// single-spec classes this whole row collapses to just the reset
	// button so the UI doesn't carry a useless bar.
	s.archetypeLabel = canvas.NewText("Archetype", theme.PlaceHolderColor())
	s.archetypeLabel.TextSize = SizeSmall
	s.archetypeLabel.TextStyle = fyne.TextStyle{Bold: true}
	s.archetypeBar = container.NewHBox()

	s.resetBtn = widget.NewButtonWithIcon("Reset build", theme.ViewRefreshIcon(), func() {
		s.purchased = map[int]int{}
		s.rebuild()
	})
	s.resetBtn.Importance = widget.LowImportance

	archetypeBlock := container.NewVBox(s.archetypeLabel, s.archetypeBar)

	topBar := container.NewBorder(
		nil, nil,
		budgetBlock, // left
		container.NewVBox(archetypeBlock, s.resetBtn), // right
		nil,
	)

	s.slotBox = container.NewVBox()

	scroll := container.NewVScroll(s.slotBox)
	scroll.SetMinSize(fyne.NewSize(0, 440))

	s.container = container.NewBorder(
		container.NewVBox(
			container.NewPadded(topBar),
			widget.NewSeparator(),
		),
		nil, nil, nil,
		scroll,
	)
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

// rebuildSpecOptions repopulates the archetype button row so it
// matches the current HasCustomSpec. Kept separate from rebuild()
// because the options only change when the user flips the archetype
// count or renames a spec. On single-spec classes the row hides.
func (s *PointBuySimulator) rebuildSpecOptions() {
	ch := s.owner.editor.character
	count := ch.HasCustomSpec
	if count < 2 {
		count = 1
	}
	if count > maxArchetypes {
		count = maxArchetypes
	}

	s.specNames = make([]string, count)
	for i := 0; i < count; i++ {
		name := ch.CustomSpecNames[i]
		if name == "" {
			name = fmt.Sprintf("Spec %d", i+1)
		}
		s.specNames[i] = name
	}

	if s.activeSpec >= count {
		s.activeSpec = 0
	}

	s.archetypeBar.Objects = nil
	for i, name := range s.specNames {
		idx := i // capture
		btn := widget.NewButton(name, func() {
			if s.activeSpec == idx {
				return
			}
			s.activeSpec = idx
			s.purchased = map[int]int{}
			s.rebuildSpecOptions()
			s.rebuild()
		})
		if i == s.activeSpec {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		s.archetypeBar.Add(btn)
	}
	s.archetypeBar.Refresh()

	if count == 1 {
		s.archetypeLabel.Hide()
		s.archetypeBar.Hide()
	} else {
		s.archetypeLabel.Show()
		s.archetypeBar.Show()
	}
}

// rebuild renders every slot card for the active archetype.
func (s *PointBuySimulator) rebuild() {
	s.slotBox.Objects = nil

	ch := s.owner.editor.character
	base := s.activeSpec * slotsPerArchetype
	for i := 0; i < slotsPerArchetype; i++ {
		idx := base + i
		row := s.buildSlotCard(idx, ch)
		if row != nil {
			s.slotBox.Add(row)
		}
	}
	s.slotBox.Refresh()
	s.refreshBudget()
}

// buildSlotCard produces a game-loadout-style card for one slot.
// Returns nil for Empty slots. Header slots return a styled divider.
func (s *PointBuySimulator) buildSlotCard(globalIdx int, ch *parsers.MBCHCharacter) fyne.CanvasObject {
	skill := ch.CustomSkills[globalIdx]
	name := ch.CustomNames[globalIdx]
	ranks := ch.CustomRanks[globalIdx]
	descs := ch.CustomDescs[globalIdx]

	if skill == "" {
		return nil // Empty slot — skip
	}

	// Header slot: styled as an accent-colored divider. Matches how
	// MBII's in-game loadout menu shows category headers (-Weapons-,
	// -Abilities-, etc.) between skill blocks.
	if skill == "MB_ATT_INVALID" && strings.TrimSpace(ranks) == "-1" {
		return buildHeaderStrip(name)
	}

	// Skill card header: icon on the left, name + desc stacked, rank
	// pills on the right. Wraps in a subtle bordered box so each
	// skill reads as a distinct loadout row.
	icon := s.buildSlotIcon(skill)

	title := canvas.NewText(name, theme.ForegroundColor())
	title.TextSize = SizeSubtitle
	title.TextStyle = fyne.TextStyle{Bold: true}
	if name == "" {
		title.Text = skill
	}

	// Description under the title, muted italic — MBII uses these
	// as tooltip-style hints ("Extra Grapple momentum", etc.).
	var titleBlock fyne.CanvasObject
	if descs != "" {
		desc := canvas.NewText(descs, theme.PlaceHolderColor())
		desc.TextSize = SizeSmall
		desc.TextStyle = fyne.TextStyle{Italic: true}
		titleBlock = container.NewVBox(title, desc)
	} else {
		titleBlock = container.NewVBox(title)
	}

	// Rank dots — in-game loadout metaphor. Left-click a dot to fill
	// ranks 1..N; right-click to step back one rank. Affordable dots
	// glow accent-color; unaffordable ones dim. No "Off" pill — right-
	// click on the rank-1 dot drops to unowned, matching MBII's menu.
	costs := parseRankCostList(ranks)
	currentRank := s.purchased[globalIdx]
	dots := NewRankDots(costs, currentRank,
		func(rank int) bool {
			if ch.MBPoints == 0 {
				return true
			}
			tentative := s.purchasedCostExcluding(globalIdx) + s.costForRank(globalIdx, rank, ch)
			return tentative <= ch.MBPoints
		},
		func(newRank int) {
			if newRank < 0 {
				newRank = 0
			}
			if newRank > len(costs) {
				newRank = len(costs)
			}
			s.purchased[globalIdx] = newRank
			s.rebuild()
		},
	)

	// Card layout. GridWrap forces the icon to 44px regardless of
	// container width; the name block flexes; pills pin to the
	// right edge via HBox with a spacer.
	leftHalf := container.NewHBox(
		container.NewGridWrap(fyne.NewSize(44, 44), icon),
		titleBlock,
	)
	rightHalf := container.NewHBox(layout.NewSpacer(), dots)
	inner := container.NewBorder(nil, nil, leftHalf, rightHalf, nil)

	bg := canvas.NewRectangle(skillCardFill())
	bg.StrokeColor = skillCardStroke()
	bg.StrokeWidth = 1

	return container.NewPadded(
		container.NewStack(bg, container.NewPadded(inner)),
	)
}

// buildSlotIcon resolves the MB_ATT icon for display. Falls back
// to the generic file-image glyph for unresolved attrs — still
// conveys "icon slot" visually so the card layout doesn't look
// asymmetric against neighbors that DO have art.
func (s *PointBuySimulator) buildSlotIcon(skill string) fyne.CanvasObject {
	if s.owner.editor.iconResolver != nil && s.owner.editor.assetBrowser != nil {
		path := s.owner.editor.iconResolver.ResolveAttributeIcon(skill)
		if res := s.owner.editor.assetBrowser.LoadIconResource(path); res != nil {
			return NewRasterIconFromResource(res, 44, 44)
		}
	}
	return NewRasterIconFromResource(theme.FileImageIcon(), 44, 44)
}

// buildHeaderStrip renders a -Section- divider styled like MBII's
// in-game category headers. Centered accent-colored text with
// top/bottom padding so section starts feel breathy.
func buildHeaderStrip(name string) fyne.CanvasObject {
	text := strings.TrimSpace(name)
	text = strings.TrimPrefix(text, "-")
	text = strings.TrimSuffix(text, "-")
	text = strings.TrimSpace(text)
	if text == "" {
		text = "Section"
	}
	text = strings.ToUpper(text)

	label := canvas.NewText(text, CurrentThemeColor)
	label.TextSize = SizeSubtitle
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter

	rule := canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 80))
	rule.SetMinSize(fyne.NewSize(0, 1))

	padded := container.NewPadded(label)
	return container.NewVBox(rule, padded, rule)
}

// purchasedCostExcluding sums the purchased cost of every slot
// except the given one. Skipping -1 means "count everything".
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

// refreshBudget updates the banner numbers + fill strip. Matches the
// in-game menu's live-updating "Points Remaining" counter.
func (s *PointBuySimulator) refreshBudget() {
	spent := s.purchasedCostExcluding(-1)
	target := s.owner.editor.character.MBPoints
	remaining := target - spent

	s.budgetSpent.Text = strconv.Itoa(spent)
	s.budgetTotal.Text = fmt.Sprintf("/ %d", target)
	s.budgetRemaining.Text = fmt.Sprintf("%d remaining", remaining)

	// Color the spent number: green under budget, accent at budget,
	// red over budget (shouldn't happen since unaffordable pills
	// disable, but guard in case mbPoints drops after purchases).
	switch {
	case target == 0:
		s.budgetSpent.Color = theme.PlaceHolderColor()
	case spent > target:
		s.budgetSpent.Color = color.NRGBA{R: 220, G: 70, B: 70, A: 255}
	case spent == target:
		s.budgetSpent.Color = CurrentThemeColor
	default:
		s.budgetSpent.Color = theme.ForegroundColor()
	}

	// Proportional strip. Clamped [0,1]. Over-budget pegs at 100%
	// and turns red so the reader's eye catches it.
	frac := float32(0)
	if target > 0 {
		frac = float32(spent) / float32(target)
		if frac > 1 {
			frac = 1
		}
	}
	_ = frac // Fyne's canvas.Rectangle doesn't support percentage-
	// width natively without reflowing; the 4px height strip stays
	// the full width of its container as a background bar. A future
	// pass could layer two rects (background + accent) to render the
	// proportional fill properly; for now the number + color carries
	// the weight. Keep the strip as a consistent accent bar so the
	// banner layout stays stable.
	s.budgetFill.FillColor = tintWithAlpha(CurrentThemeColor, 90)

	s.budgetSpent.Refresh()
	s.budgetTotal.Refresh()
	s.budgetRemaining.Refresh()
	s.budgetFill.Refresh()
}

// parseRankCostList parses a CSV into an int slice. "-1" or empty
// return empty (header / empty slots have no rank costs).
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

// skillCardFill + skillCardStroke pull from the theme so the card
// background follows dark/light mode.
func skillCardFill() color.Color {
	return tintWithAlpha(CurrentThemeColor, 12)
}

func skillCardStroke() color.Color {
	return tintWithAlpha(CurrentThemeColor, 40)
}
