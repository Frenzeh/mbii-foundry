package main

// Typography scale for MBII Foundry.
//
// Sizes are built on a golden-ratio progression (φ ≈ 1.618) from a body
// baseline of 14, then rounded to whole pixels for crisp rendering.
// Everything under 11 pt clamps to 11 — Fyne's default system fonts
// render unreadably below that on high-DPI displays, and the user's
// rule is "at least 11pt everywhere."
//
// Spacing uses a parallel golden scale from a base of 8 pixels.
//
// Use these constants everywhere instead of raw font sizes / paddings.
// Keeps the visual rhythm consistent across the app without every
// call-site making a bespoke choice.

import (
	"fyne.io/fyne/v2"
)

// Font sizes. Approx golden ratio from 14:
//
//	14 · φ ≈ 22.6
//	22.6 · φ ≈ 36.5
//	36.5 · φ ≈ 59.1
//
// Rule: sizes **below 11** render in the bundled monospace (Hack).
// Hack is designed to stay legible at small sizes in a way proportional
// faces like Jost don't. At ≥11 the default (Jost) is used. Use
// SmallText() below when you need a sub-11 size — it picks the right
// TextStyle automatically.
const (
	// SizeSmall is the normal "low-emphasis chrome" size — byte
	// counters, footer notes. Stays in Jost because it's ≥11.
	SizeSmall = float32(11)

	// SizeBody is the default for form labels, tree rows, button text.
	SizeBody = float32(14)

	// SizeSubtitle is a notch up — section headers, info-panel titles.
	SizeSubtitle = float32(18)

	// SizeHeading is "h2" — prominent section headers on launch screen
	// or modal dialogs.
	SizeHeading = float32(23)

	// SizeTitle is "h1" — page-level titles (tab name when no other
	// chrome is present).
	SizeTitle = float32(36)

	// SizeDisplay is the big branded hero number — reserved for the
	// welcome screen's "MBII FOUNDRY" title.
	SizeDisplay = float32(54)
)

// TextStyleForSize returns the right TextStyle to pair with a given
// target size. Sizes below 11 get Monospace=true so the theme's
// Font() returns Hack (readable at small sizes) instead of Jost.
func TextStyleForSize(size float32) fyne.TextStyle {
	if size < 11 {
		return fyne.TextStyle{Monospace: true}
	}
	return fyne.TextStyle{}
}

// Spacing tokens — golden scale from an 8-pixel base.
const (
	SpaceXS = float32(4)  // hairline gap between tight-related items
	SpaceSM = float32(8)  // default inter-widget padding
	SpaceMD = float32(13) // roughly SpaceSM × φ
	SpaceLG = float32(21) // section-level spacing
	SpaceXL = float32(34) // top-level layout breathing room
)

// Gap returns a transparent spacer the size of the named token. Use
// in container.NewVBox / container.NewHBox where you'd otherwise
// reach for a Separator but want breathing room, not a visible line.
func Gap(token float32) fyne.CanvasObject {
	sp := &fyneSpacer{size: fyne.NewSize(token, token)}
	return sp
}

// fyneSpacer is a CanvasObject that simply takes up space. Used by
// Gap() to force layout breathing room without visual chrome.
type fyneSpacer struct {
	size       fyne.Size
	position   fyne.Position
	visible    bool
	cachedSize fyne.Size
}

func (s *fyneSpacer) MinSize() fyne.Size      { return s.size }
func (s *fyneSpacer) Size() fyne.Size         { return s.cachedSize }
func (s *fyneSpacer) Resize(sz fyne.Size)     { s.cachedSize = sz }
func (s *fyneSpacer) Position() fyne.Position { return s.position }
func (s *fyneSpacer) Move(pos fyne.Position)  { s.position = pos }
func (s *fyneSpacer) Visible() bool           { return !s.visible }
func (s *fyneSpacer) Show()                   { s.visible = true }
func (s *fyneSpacer) Hide()                   { s.visible = false }
func (s *fyneSpacer) Refresh()                {}
