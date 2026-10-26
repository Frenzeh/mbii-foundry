package main

// TilePanel — the launcher-style rounded-and-offset-stroke surface
// shared by the info-panel hero band, attribute toggle rows, weapon
// grid cards, and (eventually) every other panel chrome that needs
// to read as its own identity block. Pulled out of three inline
// copies in info_panel.go / widget_attribute_toggle.go / weapon_grid.go
// so the look stays consistent and changes land in one place.
//
// Design: a low-contrast tinted surface with one inset accent stroke.
// Rounded corners and restrained alpha separate content groups without
// turning every section into a competing callout.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// The outer radius is one pixel larger than the inset frame so the two
// curves remain visually parallel.
const (
	TileOuterCornerRadius = 7
	TileInnerCornerRadius = 6
	TileFrameInset        = 4
	TileFillAlpha         = 14
	TileStrokeAlpha       = 64
)

// TileOpts lets callers override the default accent-color identity
// (e.g. attribute rows pass per-category colors). Zero values pick
// the global theme defaults.
type TileOpts struct {
	// AccentColor drives both the fill tint and the stroke color.
	// nil → CurrentThemeColor. Pass per-category color for category
	// tiles (force/saber/weapons/etc).
	AccentColor color.Color
	// FillAlpha / StrokeAlpha override defaults if non-zero. Range
	// 0-255. Used to dial cards to lower visual weight than the
	// info-panel hero (cards get 14/60, hero stays 22/110).
	FillAlpha   uint8
	StrokeAlpha uint8
	// Padded wraps the content in a single Padded layer when true.
	// Most callers want this; the info-panel hero already does its
	// own double-padding for inner spacing.
	Padded bool
}

// TilePanelWidget is a dynamic tile container that supports runtime restyling.
type TilePanelWidget struct {
	*fyne.Container
	Bg    *canvas.Rectangle
	Frame *canvas.Rectangle
}

func (tp *TilePanelWidget) SetAccent(accent color.Color, fillAlpha, strokeAlpha uint8) {
	if accent == nil {
		accent = CurrentThemeColor
	}
	tp.Bg.FillColor = tintWithAlpha(accent, fillAlpha)
	tp.Frame.StrokeColor = tintWithAlpha(accent, strokeAlpha)
	tp.Bg.Refresh()
	tp.Frame.Refresh()
}

// NewDynamicTilePanel returns a TilePanelWidget that can update its stroke and fill colors dynamically.
func NewDynamicTilePanel(content fyne.CanvasObject, opts TileOpts) *TilePanelWidget {
	accent := opts.AccentColor
	if accent == nil {
		accent = CurrentThemeColor
	}
	fillAlpha := opts.FillAlpha
	if fillAlpha == 0 {
		fillAlpha = TileFillAlpha
	}
	strokeAlpha := opts.StrokeAlpha
	if strokeAlpha == 0 {
		strokeAlpha = TileStrokeAlpha
	}

	bg := canvas.NewRectangle(tintWithAlpha(accent, fillAlpha))
	bg.CornerRadius = TileOuterCornerRadius

	frame := canvas.NewRectangle(color.Transparent)
	frame.StrokeColor = tintWithAlpha(accent, strokeAlpha)
	frame.StrokeWidth = 1
	frame.CornerRadius = TileInnerCornerRadius
	framePadded := container.NewPadded(frame)

	body := content
	if opts.Padded {
		body = container.NewPadded(content)
	}

	stack := container.NewStack(bg, framePadded, body)
	return &TilePanelWidget{
		Container: stack,
		Bg:        bg,
		Frame:     frame,
	}
}

// NewTilePanel returns a Stack of (bg, inset frame, content). Use
// the result anywhere you want the launcher-style surface identity.
func NewTilePanel(content fyne.CanvasObject, opts TileOpts) *fyne.Container {
	return NewDynamicTilePanel(content, opts).Container
}
