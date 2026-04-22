package main

import (
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// TooltipButton is a button that shows a tooltip on hover.
//
// Uses a 400ms hover delay before showing (IDE-standard) so the
// tooltip doesn't flicker when the mouse just transits across an icon
// toolbar. Tracks hover state explicitly so the tooltip cancels if
// the user moves off during the delay, and so repeated MouseIn calls
// from Fyne (e.g., during layout events) don't stack popups.
type TooltipButton struct {
	widget.Button
	tooltipText string
	popUp       *widget.PopUp
	hovering    bool
}

const tooltipHoverDelay = 400 * time.Millisecond

// NewTooltipButton creates a new button with a tooltip
func NewTooltipButton(text string, icon fyne.Resource, action func(), tooltip string) *TooltipButton {
	b := &TooltipButton{tooltipText: tooltip}
	b.Text = text
	b.Icon = icon
	b.OnTapped = action
	b.ExtendBaseWidget(b)
	return b
}

func (b *TooltipButton) MouseIn(e *desktop.MouseEvent) {
	if b.tooltipText == "" {
		return
	}
	b.hovering = true
	// If a tooltip is already showing (user hovering same button),
	// don't re-create it.
	if b.popUp != nil {
		return
	}
	// Schedule the popup after a short delay. If the user moves off
	// during the delay, hovering will flip false and we abort.
	go func() {
		time.Sleep(tooltipHoverDelay)
		fyne.Do(func() {
			if !b.hovering || b.popUp != nil {
				return
			}
			c := fyne.CurrentApp().Driver().CanvasForObject(b)
			if c == nil {
				return
			}
			label := widget.NewLabel(b.tooltipText)
			b.popUp = widget.NewPopUp(label, c)
			b.popUp.ShowAtRelativePosition(fyne.NewPos(0, b.Size().Height), b)
		})
	}()
}

func (b *TooltipButton) MouseOut() {
	b.hovering = false
	if b.popUp != nil {
		b.popUp.Hide()
		b.popUp = nil
	}
}

func (b *TooltipButton) MouseMoved(*desktop.MouseEvent) {}

// HoverLabel is a label that detects mouse entry
type HoverLabel struct {
	widget.Label
	onHover func()
}

func NewHoverLabel(text string, onHover func()) *HoverLabel {
	l := &HoverLabel{onHover: onHover}
	l.ExtendBaseWidget(l)
	l.SetText(text)
	return l
}

func (h *HoverLabel) MouseIn(*desktop.MouseEvent) {
	if h.onHover != nil {
		h.onHover()
	}
}

func (h *HoverLabel) MouseOut()                      {}
func (h *HoverLabel) MouseMoved(*desktop.MouseEvent) {}

// MultiSelectWidget allows selecting multiple options from a list.
type MultiSelectWidget struct {
	widget.BaseWidget
	options      []string
	selected     map[string]bool
	displayLabel *widget.Label
	button       *widget.Button
	onChanged    func(string) // Callback with comma-separated selected values
	onHover      func(string) // Optional hover callback
}

// NewMultiSelectWidget creates a new MultiSelectWidget.
func NewMultiSelectWidget(allOptions []string, initialValue string, onChanged func(string), onHover func(string)) *MultiSelectWidget {
	ms := &MultiSelectWidget{
		options:   allOptions,
		selected:  make(map[string]bool),
		onChanged: onChanged,
		onHover:   onHover,
	}
	ms.ExtendBaseWidget(ms)

	// Parse initial value
	if initialValue != "" {
		for _, val := range strings.Split(initialValue, "|") {
			ms.selected[strings.TrimSpace(val)] = true
		}
	}

	ms.displayLabel = widget.NewLabel(ms.getDisplayText())
	ms.displayLabel.Wrapping = fyne.TextWrapBreak
	ms.button = widget.NewButton("Change...", func() {
		ms.showSelectionDialog()
	})

	return ms
}

// CreateRenderer returns a new WidgetRenderer for this widget.
func (ms *MultiSelectWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewBorder(
		nil, nil, nil, ms.button, ms.displayLabel,
	))
}

// SetSelected sets the selected items based on a pipe-separated string.
func (ms *MultiSelectWidget) SetSelected(value string) {
	ms.selected = make(map[string]bool)
	if value != "" {
		for _, val := range strings.Split(value, "|") {
			ms.selected[strings.TrimSpace(val)] = true
		}
	}
	ms.displayLabel.SetText(ms.getDisplayText())
	ms.Refresh()
}

// GetSelected returns the current selections as a pipe-separated string.
func (ms *MultiSelectWidget) GetSelected() string {
	var vals []string
	for opt, isSelected := range ms.selected {
		if isSelected {
			vals = append(vals, opt)
		}
	}
	sort.Strings(vals) // Keep output consistent
	return strings.Join(vals, "|")
}

func (ms *MultiSelectWidget) getDisplayText() string {
	var vals []string
	for opt, isSelected := range ms.selected {
		if isSelected {
			vals = append(vals, opt)
		}
	}
	if len(vals) == 0 {
		return "None"
	}
	sort.Strings(vals)
	return strings.Join(vals, ", ")
}

func (ms *MultiSelectWidget) showSelectionDialog() {
	currentSelections := make(map[string]bool)
	for k, v := range ms.selected {
		currentSelections[k] = v // Copy map
	}

	// Manual implementation for filtering
	containerBox := container.NewVBox()

	// Helper to render the list
	renderList := func(filter string) {
		containerBox.Objects = nil
		filter = strings.ToLower(filter)

		// Sort options for stability
		sortedOptions := make([]string, len(ms.options))
		copy(sortedOptions, ms.options)
		sort.Strings(sortedOptions)

		for _, opt := range sortedOptions {
			if filter == "" || strings.Contains(strings.ToLower(opt), filter) {
				// Capture variable
				o := opt
				chk := widget.NewCheck("", func(b bool) { // Empty label
					currentSelections[o] = b
				})
				chk.Checked = currentSelections[o]

				// Use HoverLabel for text
				lbl := NewHoverLabel(o, func() {
					if ms.onHover != nil {
						ms.onHover(o)
					}
				})

				containerBox.Add(container.NewHBox(chk, lbl))
			}
		}
		containerBox.Refresh()
	}

	renderList("") // Initial render

	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder("Search...")
	searchEntry.OnChanged = func(s string) {
		renderList(s)
	}

	scroll := container.NewVScroll(containerBox)
	scroll.SetMinSize(fyne.NewSize(300, 400)) // Max height for dialog

	content := container.NewBorder(searchEntry, nil, nil, nil, scroll)

	dialog.ShowCustomConfirm("Select Options", "OK", "Cancel", content, func(ok bool) {
		if ok {
			ms.selected = currentSelections // Apply changes
			ms.displayLabel.SetText(ms.getDisplayText())
			if ms.onChanged != nil {
				ms.onChanged(ms.GetSelected())
			}
			ms.Refresh()
		}
	}, fyne.CurrentApp().Driver().AllWindows()[0])
}

// HoverButton is a button that triggers a callback on mouse hover
type HoverButton struct {
	widget.Button
	onHover    func()
	onHoverOut func()
}

func NewHoverButton(text string, tapped func(), onHover func(), onHoverOut func()) *HoverButton {
	b := &HoverButton{onHover: onHover, onHoverOut: onHoverOut}
	b.Text = text
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *HoverButton) MouseIn(e *desktop.MouseEvent) {
	b.Button.MouseIn(e)
	if b.onHover != nil {
		b.onHover()
	}
}

func (b *HoverButton) MouseOut() {
	b.Button.MouseOut()
	if b.onHoverOut != nil {
		b.onHoverOut()
	}
}
