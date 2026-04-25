package main

// EmptyStateTile — a TilePanel-shaped reusable empty-state component.
// Replaces blank silent panels (filter-zero in attribute/weapon grids,
// "no docs available" in info panel, "select a file" placeholder in
// source panel) with a consistent identity block.
//
// Design intent: when content is absent, the user should still see
// the surface telling them WHAT'S MISSING and what to do about it.
// This component renders headline + body + optional action button
// inside a TilePanel shell so empty states don't look like render
// failures.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// NewEmptyStateTile builds the standard empty-state surface.
// headline: short "what state is this" label (kept short, all-caps
// reads brutalist + matches the section-caption pattern).
// hint: 1-2 sentence body explaining what the user can do.
// actionLabel + action: optional button. Pass "" + nil to omit.
func NewEmptyStateTile(headline, hint, actionLabel string, action func()) fyne.CanvasObject {
	headlineText := canvas.NewText(headline, theme.PlaceHolderColor())
	headlineText.TextSize = SizeSmall
	headlineText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	headlineText.Alignment = fyne.TextAlignCenter

	hintLabel := widget.NewLabelWithStyle(hint, fyne.TextAlignCenter, fyne.TextStyle{})
	hintLabel.Wrapping = fyne.TextWrapWord

	stack := container.NewVBox(headlineText, hintLabel)

	if actionLabel != "" && action != nil {
		btn := widget.NewButton(actionLabel, action)
		btn.Importance = widget.LowImportance
		stack.Add(container.NewCenter(btn))
	}

	tile := NewTilePanel(stack, TileOpts{Padded: true})
	// Wrap once more in Padded so the tile floats inside its parent
	// rather than touching the parent's edges — empty states are
	// breathing-room moments, not dense data displays.
	return container.NewPadded(tile)
}
