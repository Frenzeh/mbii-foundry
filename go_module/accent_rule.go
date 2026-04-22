package main

// AccentRule — a thin theme-reactive horizontal bar painted in the
// current accent color. Used anywhere we want a subtle "section
// divider" that should repaint when the user swaps themes.
//
// Plain `canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 90))`
// captures the color at construction time and never updates. Fyne's
// theme-swap path calls Refresh() on the widget tree; this widget's
// Refresh re-reads CurrentThemeColor so the paint follows along.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// Alpha used for accent rules across the app. Centralized so every
// call site renders at the same weight.
const accentRuleAlpha = 90

type AccentRule struct {
	widget.BaseWidget
	rect *canvas.Rectangle
}

// NewAccentRule returns a 2-px-tall rule. Vertical padding around it
// is the caller's responsibility.
func NewAccentRule() *AccentRule {
	r := &AccentRule{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *AccentRule) CreateRenderer() fyne.WidgetRenderer {
	r.rect = canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, accentRuleAlpha))
	return widget.NewSimpleRenderer(r.rect)
}

// MinSize gives the rule its height. Width comes from the parent.
func (r *AccentRule) MinSize() fyne.Size { return fyne.NewSize(0, 2) }

// Refresh re-reads the accent color. Fyne invokes this on widgets
// during a theme-swap, so swapping e.g. Sith → Jedi repaints the
// rule from red to blue without needing a full layout rebuild.
func (r *AccentRule) Refresh() {
	if r.rect != nil {
		r.rect.FillColor = tintWithAlpha(CurrentThemeColor, accentRuleAlpha)
		r.rect.Refresh()
	}
	r.BaseWidget.Refresh()
}
