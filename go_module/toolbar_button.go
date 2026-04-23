package main

// ToolbarButton — a tight, low-chrome icon (or icon+label) button
// designed for toolbar and rail contexts. Hover fill is a subtle
// accent tint matching the rest of the app (sidebar pills, tile
// cards), replacing Fyne's default grey Material hover which clashed
// with the dark/accent design language.
//
// Use this instead of widget.Button for any toolbar- or rail-style
// control. Keep widget.Button for form-grade buttons (Save/Cancel)
// where the Material fill is actually appropriate.

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ToolbarButton struct {
	widget.BaseWidget

	icon    fyne.Resource
	label   string
	tooltip string
	onTap   func()

	hovering bool
	disabled bool
	bg       *canvas.Rectangle
	iconW    *widget.Icon
	labelW   *canvas.Text
	popUp    *widget.PopUp
}

// Disable turns the button into a non-interactive, visually-faded
// state. Used by panels that need to lock the button out while a
// background action is running (e.g. the update banner during
// download).
func (b *ToolbarButton) Disable() {
	if b.disabled {
		return
	}
	b.disabled = true
	b.applyStyle()
	if b.iconW != nil {
		b.iconW.Refresh()
	}
	b.Refresh()
}

// Enable re-enables a previously disabled button.
func (b *ToolbarButton) Enable() {
	if !b.disabled {
		return
	}
	b.disabled = false
	b.applyStyle()
	if b.iconW != nil {
		b.iconW.Refresh()
	}
	b.Refresh()
}

// Disabled reports the current state.
func (b *ToolbarButton) Disabled() bool { return b.disabled }

func NewToolbarButton(label string, icon fyne.Resource, onTap func(), tooltip string) *ToolbarButton {
	b := &ToolbarButton{label: label, icon: icon, onTap: onTap, tooltip: tooltip}
	b.ExtendBaseWidget(b)
	return b
}

// SetIcon swaps the live icon — matches activityPill.setIcon pattern
// so callers can flip state without rebuilding the button.
func (b *ToolbarButton) SetIcon(icon fyne.Resource, tooltip string) {
	b.icon = icon
	b.tooltip = tooltip
	if b.iconW != nil {
		b.iconW.SetResource(icon)
	}
}

func (b *ToolbarButton) CreateRenderer() fyne.WidgetRenderer {
	b.bg = canvas.NewRectangle(color.Transparent)
	b.iconW = widget.NewIcon(b.icon)

	var content fyne.CanvasObject
	if b.label != "" {
		b.labelW = canvas.NewText(b.label, theme.ForegroundColor())
		b.labelW.TextSize = SizeSmall
		b.labelW.TextStyle = fyne.TextStyle{Bold: true}
		row := container.NewHBox(b.iconW, b.labelW)
		content = container.NewPadded(row)
	} else {
		content = container.NewPadded(b.iconW)
	}
	return widget.NewSimpleRenderer(container.NewStack(b.bg, content))
}

func (b *ToolbarButton) MinSize() fyne.Size {
	if b.label == "" {
		return fyne.NewSize(32, 32)
	}
	// Widen a bit for labeled buttons — base width + label budget.
	return fyne.NewSize(68, 32)
}

func (b *ToolbarButton) Tapped(*fyne.PointEvent) {
	if b.disabled {
		return
	}
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *ToolbarButton) MouseIn(*desktop.MouseEvent) {
	if b.disabled {
		return
	}
	b.hovering = true
	b.applyStyle()
	if b.tooltip == "" {
		return
	}
	go func() {
		time.Sleep(400 * time.Millisecond)
		fyne.Do(func() {
			if !b.hovering || b.popUp != nil {
				return
			}
			c := fyne.CurrentApp().Driver().CanvasForObject(b)
			if c == nil {
				return
			}
			lbl := widget.NewLabel(b.tooltip)
			b.popUp = widget.NewPopUp(lbl, c)
			b.popUp.ShowAtRelativePosition(fyne.NewPos(0, b.Size().Height), b)
		})
	}()
}

func (b *ToolbarButton) MouseOut() {
	b.hovering = false
	b.applyStyle()
	if b.popUp != nil {
		b.popUp.Hide()
		b.popUp = nil
	}
}

func (b *ToolbarButton) MouseMoved(*desktop.MouseEvent) {}

func (b *ToolbarButton) Cursor() desktop.Cursor {
	if b.disabled {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *ToolbarButton) applyStyle() {
	if b.bg == nil {
		return
	}
	if b.hovering {
		b.bg.FillColor = tintWithAlpha(CurrentThemeColor, 55)
	} else {
		b.bg.FillColor = color.Transparent
	}
	b.bg.Refresh()
}
