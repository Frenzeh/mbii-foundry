package main

// TileCard — a hoverable, tappable card with an icon + label + subtle
// sublabel. Used by the welcome screen's CREATE column (and anywhere
// else we want a chunkier call-to-action than a standard button).
//
// Chrome treatment (deliberately restrained — Fyne can't do real
// shadows or gradients, so we lean on layered flat colors):
//
//   * resting:   very faint fill (5% theme tint) + 1px subtle border
//   * hovering:  brighter fill (25% theme tint) + accent border + a
//                thin accent rule on the left edge that reads as a
//                "focus bar" — gives a sense of direction without
//                needing a glow we can't render
//   * pressed:   background briefly deeper; handled by widget.BaseWidget
//
// Sublabel is rendered in Hack (monospace) at SizeSmall to feel like
// a file-extension hint ("mbch") rather than fighting the primary
// label for attention.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type TileCard struct {
	widget.BaseWidget

	Label    string
	Sublabel string
	Icon     fyne.Resource
	OnTap    func()

	bg       *canvas.Rectangle
	border   *canvas.Rectangle
	accent   *canvas.Rectangle // thin left-edge rule that lights up on hover
	hovering bool
}

func NewTileCard(label, sublabel string, icon fyne.Resource, onTap func()) *TileCard {
	c := &TileCard{Label: label, Sublabel: sublabel, Icon: icon, OnTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *TileCard) CreateRenderer() fyne.WidgetRenderer {
	c.bg = canvas.NewRectangle(cardRestingFill())
	c.border = canvas.NewRectangle(color.Transparent)
	c.border.StrokeColor = cardRestingBorder()
	c.border.StrokeWidth = 1
	c.border.FillColor = color.Transparent

	c.accent = canvas.NewRectangle(color.Transparent)

	iconWidget := widget.NewIcon(c.Icon)

	label := canvas.NewText(c.Label, theme.ForegroundColor())
	label.TextSize = SizeBody
	label.TextStyle = fyne.TextStyle{Bold: true}

	sub := canvas.NewText(c.Sublabel, theme.PlaceHolderColor())
	sub.TextSize = SizeSmall
	sub.TextStyle = fyne.TextStyle{Monospace: true} // file-extension vibe

	// Icon on the left, label + sub stacked to the right of it.
	textStack := container.NewVBox(label, sub)
	row := container.NewBorder(nil, nil,
		container.NewPadded(iconWidget),
		nil,
		container.NewPadded(textStack))

	// Accent bar pinned to the left edge. 3px wide, full card height.
	accentRow := container.NewBorder(nil, nil,
		&fixedWidthSpacer{width: 3, child: c.accent},
		nil,
		row)

	return widget.NewSimpleRenderer(container.NewStack(c.bg, c.border, accentRow))
}

func (c *TileCard) MinSize() fyne.Size { return fyne.NewSize(240, 64) }

func (c *TileCard) Tapped(*fyne.PointEvent) {
	if c.OnTap != nil {
		c.OnTap()
	}
}

func (c *TileCard) MouseIn(*desktop.MouseEvent) {
	c.hovering = true
	c.applyStyle()
}

func (c *TileCard) MouseOut() {
	c.hovering = false
	c.applyStyle()
}

func (c *TileCard) MouseMoved(*desktop.MouseEvent) {}

// Cursor tells Fyne to flip to pointer when hovering — reads as
// "this is clickable" without needing a hover outline alone.
func (c *TileCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (c *TileCard) applyStyle() {
	if c.bg == nil {
		return
	}
	if c.hovering {
		c.bg.FillColor = cardHoverFill()
		c.border.StrokeColor = cardHoverBorder()
		c.accent.FillColor = tintWithAlpha(CurrentThemeColor, 220)
	} else {
		c.bg.FillColor = cardRestingFill()
		c.border.StrokeColor = cardRestingBorder()
		c.accent.FillColor = color.Transparent
	}
	c.bg.Refresh()
	c.border.Refresh()
	c.accent.Refresh()
}

// cardRestingFill / cardHoverFill — subtle layered tints designed to
// read as "there's a surface here" without demanding attention.
func cardRestingFill() color.Color  { return color.NRGBA{R: 255, G: 255, B: 255, A: 8} }
func cardHoverFill() color.Color    { return tintWithAlpha(CurrentThemeColor, 36) }
func cardRestingBorder() color.Color { return color.NRGBA{R: 255, G: 255, B: 255, A: 28} }
func cardHoverBorder() color.Color  { return tintWithAlpha(CurrentThemeColor, 180) }

// fixedWidthSpacer claims exactly `width` pixels horizontally and
// renders `child` stretched to fill. Used for the card's left-edge
// accent bar so it stays pinned regardless of text length.
type fixedWidthSpacer struct {
	widget.BaseWidget
	width float32
	child fyne.CanvasObject
}

func (s *fixedWidthSpacer) MinSize() fyne.Size { return fyne.NewSize(s.width, 0) }
func (s *fixedWidthSpacer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.child)
}
