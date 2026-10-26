package main

// CollapsedRail — the thin vertical strip that takes the place of a
// collapsed sidebar or source panel. When the user hovers anywhere on
// the rail, the entire strip fills with accent tint — communicating
// that the whole rail "represents" the collapsed panel. A click
// anywhere on the rail restores the panel.
//
// Width is fixed (28px). Height fills whatever vertical space the
// parent container allocates, so the hover-fill spans the full
// extent of where the panel used to be.

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type CollapsedRail struct {
	widget.BaseWidget

	icon    fyne.Resource
	tooltip string
	onTap   func()

	hovering bool
	bg       *canvas.Rectangle
	popUp    *widget.PopUp
}

func NewCollapsedRail(icon fyne.Resource, onTap func(), tooltip string) *CollapsedRail {
	r := &CollapsedRail{icon: icon, onTap: onTap, tooltip: tooltip}
	r.ExtendBaseWidget(r)
	return r
}

func (r *CollapsedRail) CreateRenderer() fyne.WidgetRenderer {
	// Faint resting background so the rail is visually findable as its
	// own element, not a void at the edge of the docTabs area.
	r.bg = canvas.NewRectangle(collapsedRailRestingFill())

	iconWidget := widget.NewIcon(r.icon)
	// Icon anchored at the top, centered horizontally. Spacer below
	// pushes it up so the rail feels like it's "pointing down from
	// the top bar" — mirrors how toolbar chrome reads.
	column := container.NewVBox(
		container.NewPadded(container.NewCenter(iconWidget)),
		layout.NewSpacer(),
	)

	return widget.NewSimpleRenderer(container.NewStack(r.bg, column))
}

func (r *CollapsedRail) MinSize() fyne.Size {
	return fyne.NewSize(28, 0)
}

func (r *CollapsedRail) Tapped(*fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}

func (r *CollapsedRail) MouseIn(*desktop.MouseEvent) {
	r.hovering = true
	r.applyStyle()
	if r.tooltip == "" {
		return
	}
	go func() {
		time.Sleep(400 * time.Millisecond)
		fyne.Do(func() {
			if !r.hovering || r.popUp != nil {
				return
			}
			c := fyne.CurrentApp().Driver().CanvasForObject(r)
			if c == nil {
				return
			}
			lbl := widget.NewLabel(r.tooltip)
			r.popUp = widget.NewPopUp(lbl, c)
			r.popUp.ShowAtRelativePosition(fyne.NewPos(r.Size().Width+4, 4), r)
		})
	}()
}

func (r *CollapsedRail) MouseOut() {
	r.hovering = false
	r.applyStyle()
	if r.popUp != nil {
		r.popUp.Hide()
		r.popUp = nil
	}
}

func (r *CollapsedRail) MouseMoved(*desktop.MouseEvent) {}

func (r *CollapsedRail) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (r *CollapsedRail) applyStyle() {
	if r.bg == nil {
		return
	}
	if r.hovering {
		// Full-strip accent fill — this is the whole point of the
		// widget: clarify that the rail *is* the collapsed panel.
		r.bg.FillColor = tintWithAlpha(CurrentThemeColor, 90)
	} else {
		r.bg.FillColor = collapsedRailRestingFill()
	}
	r.bg.Refresh()
}

func collapsedRailRestingFill() color.Color {
	return color.NRGBA{R: 255, G: 255, B: 255, A: 6}
}
