package main

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// AttributeToggleWidget is a custom widget for selecting attribute levels
type AttributeToggleWidget struct {
	widget.BaseWidget

	ID          string
	Name        string
	Category    string
	MaxLevel    int
	CurrentVal  int
	Description string

	OnChange func(int)
	OnInfo   func(string, string) // Key, Context — called on row hover
	// OnInfoLeave fires when the mouse leaves the row's hoverable
	// controls (info button, level buttons). Paired with OnInfo so
	// the info-panel's sticky-context behavior can revert to the
	// last-interacted entry rather than freezing on the attribute
	// you happen to have passed over last.
	OnInfoLeave func()

	// UI Components
	label     *widget.Label
	buttons   []*HoverButton
	infoBtn   *widget.Button
	container *fyne.Container
}

func NewAttributeToggleWidget(attr AttributeDef, currentVal int, onChange func(int), onInfo func(string, string), icon fyne.Resource) *AttributeToggleWidget {
	w := &AttributeToggleWidget{
		ID:          attr.ID,
		Name:        attr.Name,
		Category:    attr.Category,
		MaxLevel:    attr.MaxLevel,
		CurrentVal:  currentVal,
		Description: attr.Description,
		OnChange:    onChange,
	}

	w.ExtendBaseWidget(w)
	w.createUI(onInfo, icon)
	return w
}

// SetOnInfoLeave wires the MouseOut callback post-construction so
// callers that didn't have the clear-hover func at build time can
// still opt in. Keeps the widget's primary constructor small.
func (w *AttributeToggleWidget) SetOnInfoLeave(f func()) {
	w.OnInfoLeave = f
	// Re-apply to already-built level buttons — refreshButtons
	// doesn't rewire, so we patch each button's onHoverOut directly.
	for _, btn := range w.buttons {
		if btn != nil {
			btn.onHoverOut = f
		}
	}
}

func (w *AttributeToggleWidget) createUI(onInfo func(string, string), iconRes fyne.Resource) {
	w.label = widget.NewLabel(w.Name)
	w.label.TextStyle = fyne.TextStyle{Bold: true}

	w.infoBtn = widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		if onInfo != nil {
			onInfo(w.ID, "")
		}
	})
	w.infoBtn.Importance = widget.LowImportance // Less intrusive

	// Icon — canvas.Image (via NewRasterIconFromResource) renders the
	// full extracted PNG at 24×24. widget.Icon would downscale to the
	// theme icon size (~20px) and leave the resource mostly unseen,
	// which is what made attribute icons look "missing" earlier.
	var iconObj fyne.CanvasObject
	if iconRes != nil {
		iconObj = NewRasterIconFromResource(iconRes, 24, 24)
	} else {
		iconObj = layout.NewSpacer()
	}

	// Create toggle buttons
	w.buttons = make([]*HoverButton, w.MaxLevel+1)

	// Level 0 (Off)
	w.buttons[0] = w.createLevelButton(0, "Off", onInfo)

	// Levels 1..Max
	for i := 1; i <= w.MaxLevel; i++ {
		w.buttons[i] = w.createLevelButton(i, strconv.Itoa(i), onInfo)
	}

	btnBox := container.NewHBox()
	for _, btn := range w.buttons {
		btnBox.Add(btn)
	}

	// Color Coding
	var catColor color.Color
	switch w.Category {
	case "Force":
		catColor = color.RGBA{0, 191, 255, 255} // Deep Sky Blue
	case "Saber":
		catColor = color.RGBA{255, 69, 0, 255} // Orange Red
	case "Weapons":
		catColor = color.RGBA{255, 215, 0, 255} // Gold
	case "Class Specific":
		catColor = color.RGBA{50, 205, 50, 255} // Lime Green
	default:
		catColor = color.RGBA{128, 128, 128, 255} // Grey
	}

	rect := canvas.NewRectangle(catColor)
	rect.SetMinSize(fyne.NewSize(5, 0)) // 5px wide strip

	// Layout: [Strip] [Info] [Icon] [Label] -- Spacer -- Buttons (Right)
	leftContainer := container.NewHBox(rect, w.infoBtn, iconObj, w.label)

	w.container = container.NewBorder(nil, nil,
		leftContainer,      // Left
		btnBox,             // Right
		layout.NewSpacer(), // Center (filler)
	)

	w.refreshButtons()
}

func (w *AttributeToggleWidget) createLevelButton(level int, text string, onInfo func(string, string)) *HoverButton {
	hover := func() {
		if onInfo != nil {
			context := ""
			if level > 0 {
				context = "Level " + strconv.Itoa(level)
			}
			onInfo(w.ID, context)
		}
	}

	btn := NewHoverButton(text, func() {
		w.CurrentVal = level
		w.refreshButtons()
		if w.OnChange != nil {
			w.OnChange(level)
		}
	}, hover, func() {
		// MouseOut — tell the info panel to revert to sticky.
		// Closure captures the widget; OnInfoLeave may be set later
		// via SetOnInfoLeave so read it lazily each time.
		if w.OnInfoLeave != nil {
			w.OnInfoLeave()
		}
	})
	return btn
}

func (w *AttributeToggleWidget) refreshButtons() {
	for i, btn := range w.buttons {
		if i == w.CurrentVal {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}
}

func (w *AttributeToggleWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(w.container)
}

// Helper container that aligns content to the right
type rightAlignedLayout struct{}

func (d *rightAlignedLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for _, o := range objects {
		childSize := o.MinSize()
		w += childSize.Width
		if childSize.Height > h {
			h = childSize.Height
		}
	}
	return fyne.NewSize(w, h)
}

func (d *rightAlignedLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	pos := fyne.NewPos(containerSize.Width, 0)
	for i := len(objects) - 1; i >= 0; i-- {
		o := objects[i]
		size := o.MinSize()
		pos = pos.Subtract(fyne.NewPos(size.Width, 0))
		o.Move(pos)
		o.Resize(fyne.NewSize(size.Width, containerSize.Height))
	}
}

func NewRightAligned(content ...fyne.CanvasObject) *fyne.Container {
	return container.New(&rightAlignedLayout{}, content...)
}
