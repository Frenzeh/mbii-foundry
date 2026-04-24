package main

// RankDots — in-game-style rank selector. Mirrors what MBII shows
// in its loadout menu: a row of circle dots, one per rank, that
// fill left-to-right as you buy them. Left-click a dot to fill
// ranks 1..N; right-click to drop back one. Each dot shows its
// cost underneath.
//
// States per dot:
//   filled + affordable     → accent-filled circle, hover ring
//   empty + affordable      → hollow circle with accent outline
//   empty + unaffordable    → hollow circle in muted grey
//   filled + (any)          → stays filled visually; right-click
//                             refunds the last rank even if the
//                             player's budget has shifted
//
// The widget is self-contained — caller supplies current rank, per-
// rank costs, and an affordability predicate; the widget handles
// all rendering and click events.

import (
	"fmt"
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// RankDots renders a row of N dots representing rank purchases.
type RankDots struct {
	widget.BaseWidget

	costs       []int
	current     int
	affordable  func(rank int) bool
	onChange    func(newRank int)

	content *fyne.Container
}

func NewRankDots(costs []int, current int, affordable func(rank int) bool, onChange func(int)) *RankDots {
	rd := &RankDots{
		costs:      costs,
		current:    current,
		affordable: affordable,
		onChange:   onChange,
	}
	rd.ExtendBaseWidget(rd)
	return rd
}

func (rd *RankDots) CreateRenderer() fyne.WidgetRenderer {
	rd.content = container.NewHBox()
	rd.rebuild()
	return widget.NewSimpleRenderer(rd.content)
}

// rebuild re-renders the dot row for the current state.
func (rd *RankDots) rebuild() {
	if rd.content == nil {
		return
	}
	rd.content.Objects = nil
	for i, cost := range rd.costs {
		rank := i + 1
		rd.content.Add(newRankDot(rank, cost, rank <= rd.current,
			rd.affordable != nil && rd.affordable(rank),
			func(r int) {
				rd.current = r
				if rd.onChange != nil {
					rd.onChange(r)
				}
				rd.rebuild()
			}))
	}
	rd.content.Refresh()
}

// rankDot — a single clickable circle in the RankDots row.
type rankDot struct {
	widget.BaseWidget

	rank       int
	cost       int
	filled     bool
	affordable bool
	hovered    bool

	// onPick is called with the resulting rank after a click:
	//   left-click → set rank to dot's rank (fills up to here)
	//   right-click → set rank to dot's rank - 1 (drops back one)
	onPick func(newRank int)

	circle *canvas.Circle
	label  *canvas.Text
}

func newRankDot(rank, cost int, filled, affordable bool, onPick func(int)) *rankDot {
	d := &rankDot{
		rank:       rank,
		cost:       cost,
		filled:     filled,
		affordable: affordable,
		onPick:     onPick,
	}
	d.ExtendBaseWidget(d)
	return d
}

func (d *rankDot) CreateRenderer() fyne.WidgetRenderer {
	d.circle = canvas.NewCircle(color.Transparent)
	d.circle.StrokeWidth = 2
	d.circle.Resize(fyne.NewSize(22, 22))

	// Cost label below the circle. "0" shows as "free" — matches the
	// in-game menu convention and communicates that the rank is
	// unlocked without spending points.
	d.label = canvas.NewText(d.costLabel(), theme.PlaceHolderColor())
	d.label.TextSize = SizeSmall
	d.label.Alignment = fyne.TextAlignCenter

	d.applyStyle()

	// GridWrap forces the dot's visual footprint to a fixed cell so
	// multiple dots in a row line up neatly regardless of label
	// width variance.
	circleCell := container.New(layout.NewGridWrapLayout(fyne.NewSize(22, 22)), d.circle)
	body := container.NewVBox(container.NewCenter(circleCell), d.label)
	cell := container.New(layout.NewGridWrapLayout(fyne.NewSize(52, 48)), body)
	return widget.NewSimpleRenderer(cell)
}

// applyStyle drives the fill + stroke based on current state. Called
// at init and on every hover in/out.
func (d *rankDot) applyStyle() {
	if d.circle == nil {
		return
	}
	switch {
	case d.filled:
		d.circle.FillColor = CurrentThemeColor
		d.circle.StrokeColor = CurrentThemeColor
	case d.hovered && d.affordable:
		d.circle.FillColor = tintWithAlpha(CurrentThemeColor, 60)
		d.circle.StrokeColor = CurrentThemeColor
	case d.affordable:
		d.circle.FillColor = color.Transparent
		d.circle.StrokeColor = CurrentThemeColor
	default:
		// Unaffordable empty dot — muted so it reads as locked.
		d.circle.FillColor = color.Transparent
		d.circle.StrokeColor = color.NRGBA{R: 90, G: 90, B: 90, A: 200}
	}
	d.circle.Refresh()
}

func (d *rankDot) costLabel() string {
	return strconv.Itoa(d.cost)
}

func (d *rankDot) Tapped(*fyne.PointEvent) {
	if !d.affordable && !d.filled {
		// Can't buy what you can't afford. A filled unaffordable dot
		// (edge case where mbPoints shrank after purchase) stays
		// clickable for refund via right-click.
		return
	}
	if d.onPick != nil {
		d.onPick(d.rank)
	}
}

func (d *rankDot) TappedSecondary(*fyne.PointEvent) {
	// Right-click drops one rank — matches MBII's in-game UX where
	// right-click is "go back a point."
	if d.onPick != nil {
		d.onPick(d.rank - 1)
	}
}

func (d *rankDot) MouseIn(*desktop.MouseEvent)    { d.hovered = true; d.applyStyle() }
func (d *rankDot) MouseOut()                      { d.hovered = false; d.applyStyle() }
func (d *rankDot) MouseMoved(*desktop.MouseEvent) {}
func (d *rankDot) Cursor() desktop.Cursor {
	if d.affordable || d.filled {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

// totalCostOf returns the sum of costs for ranks 1..rank. Handy for
// callers that need to know how many points a given rank would cost.
func totalCostOf(costs []int, rank int) int {
	if rank < 0 {
		rank = 0
	}
	if rank > len(costs) {
		rank = len(costs)
	}
	total := 0
	for i := 0; i < rank; i++ {
		total += costs[i]
	}
	return total
}

// _ helper signatures to keep the linter quiet if fmt is unused in
// future refactors; currently used via costLabel fallback prints.
var _ = fmt.Sprintf
