package main

// TilePanel — the launcher-style rounded-and-offset-stroke surface
// shared by the info-panel hero band, attribute toggle rows, weapon
// grid cards, and (eventually) every other panel chrome that needs
// to read as its own identity block. Pulled out of three inline
// copies in info_panel.go / widget_attribute_toggle.go / weapon_grid.go
// so the look stays consistent and changes land in one place.
//
// Design: a tinted fill rectangle at full extent + a 1px stroke
// rectangle inset by ~4px via container.NewPadded so the border
// reads as its own ring rather than sharing the bg silhouette.
// Content sits in a Padded wrapper above both. CornerRadius 6 on
// the bg / 5 on the inset frame is the canonical "offset stroke"
// shape the MBII launcher uses for its boxes and buttons.

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// TileOuterCornerRadius / TileInnerCornerRadius — the 6/5 split is
// intentional: the inner frame is fractionally tighter than the
// outer fill so the stroke reads as inset rather than overlaid.
const (
	TileOuterCornerRadius = 6
	TileInnerCornerRadius = 5
	TileFrameInset        = 4   // px the stroke is moved inward of the bg
	TileFillAlpha         = 22  // 0-255, applied via tintWithAlpha
	TileStrokeAlpha       = 110 // 0-255, applied via tintWithAlpha
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

// NewTilePanel returns a Stack of (bg, inset frame, content). Use
// the result anywhere you want the launcher-style surface identity.
func NewTilePanel(content fyne.CanvasObject, opts TileOpts) *fyne.Container {
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
	return container.NewStack(bg, framePadded, body)
}
