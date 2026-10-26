package main

import (
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
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

// SetTooltip swaps the hover-tooltip string. Used by toggle buttons
// that want to reflect the on/off state in the tooltip text.
func (b *TooltipButton) SetTooltip(s string) { b.tooltipText = s }

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
		fyne.Do(b.showTooltip)
	}()
}

func (b *TooltipButton) showTooltip() {
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
}

// Activation can remove the button while its delayed hover callback is queued.
// Dismiss both pending and visible hints before handing control to the action.
func (b *TooltipButton) Tapped(event *fyne.PointEvent) {
	b.MouseOut()
	b.Button.Tapped(event)
}

func (b *TooltipButton) TypedKey(event *fyne.KeyEvent) {
	if event.Name == fyne.KeySpace {
		b.MouseOut()
	}
	b.Button.TypedKey(event)
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

	searchEntry := NewInputEntry()
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

// Input-field constructors — force Monospace so FoundryTheme.Font
// returns Hack. Any field the user types into is a "portal" to
// editing raw game data; monospace makes path fragments, filenames,
// and numeric values easier to scan and line up. Use these everywhere
// instead of bare widget.NewEntry / NewMultiLineEntry / NewPasswordEntry.
func NewInputEntry() *widget.Entry {
	e := widget.NewEntry()
	e.TextStyle = fyne.TextStyle{Monospace: true}
	return e
}

// wheelRoute lets a slot entry retain the identity of its owning vertical
// viewport while that viewport is rebuilt or attached later.
type wheelRoute struct {
	target fyne.Scrollable
}

func (r *wheelRoute) Scrolled(event *fyne.ScrollEvent) {
	if r == nil || r.target == nil || event == nil {
		return
	}
	r.target.Scrolled(event)
}

// slotEntry keeps the native Entry event surface while adding a scroll-only
// overlay that separates vertical page gestures from horizontal text panning.
type slotEntry struct {
	widget.Entry
	wheel *wheelRoute
}

// NewSlotEntry keeps long point-buy names, descriptions, and paths editable
// inside a narrow slot without trapping vertical wheel gestures in the Entry's
// horizontal viewport.
func NewSlotEntry(wheel *wheelRoute) *slotEntry {
	e := &slotEntry{wheel: wheel}
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.Wrapping = fyne.TextWrapOff
	e.Scroll = container.ScrollHorizontalOnly
	e.ExtendBaseWidget(e)
	return e
}

func (e *slotEntry) CreateRenderer() fyne.WidgetRenderer {
	delegate := e.Entry.CreateRenderer()
	delegateObjects := delegate.Objects()
	var horizontal *container.Scroll
	for _, object := range delegateObjects {
		if scroll, ok := object.(*container.Scroll); ok {
			horizontal = scroll
			break
		}
	}
	overlay := newSlotEntryScrollOverlay(e.wheel, &wheelRoute{target: horizontal})
	objects := make([]fyne.CanvasObject, len(delegateObjects)+1)
	copy(objects, delegateObjects)
	objects[len(objects)-1] = overlay
	return &slotEntryRenderer{
		delegate: delegate,
		overlay:  overlay,
		objects:  objects,
	}
}

type slotEntryRenderer struct {
	delegate fyne.WidgetRenderer
	overlay  *slotEntryScrollOverlay
	objects  []fyne.CanvasObject
}

func (r *slotEntryRenderer) Destroy() {
	r.delegate.Destroy()
}

func (r *slotEntryRenderer) Layout(size fyne.Size) {
	r.delegate.Layout(size)
	r.overlay.Move(fyne.NewPos(0, 0))
	r.overlay.Resize(size)
}

func (r *slotEntryRenderer) MinSize() fyne.Size {
	return r.delegate.MinSize()
}

func (r *slotEntryRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *slotEntryRenderer) Refresh() {
	r.delegate.Refresh()
	r.overlay.Refresh()
}

// slotEntryScrollOverlay is intentionally only a Widget and Scrollable. It
// must not become a tap, focus, drag, mouse, or desktop-input target because
// all non-wheel behavior belongs to the embedded native Entry.
type slotEntryScrollOverlay struct {
	widget.BaseWidget
	vertical   *wheelRoute
	horizontal *wheelRoute
}

func newSlotEntryScrollOverlay(vertical, horizontal *wheelRoute) *slotEntryScrollOverlay {
	overlay := &slotEntryScrollOverlay{vertical: vertical, horizontal: horizontal}
	overlay.ExtendBaseWidget(overlay)
	return overlay
}

func (o *slotEntryScrollOverlay) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (o *slotEntryScrollOverlay) Scrolled(event *fyne.ScrollEvent) {
	if event == nil {
		return
	}
	dx, dy := event.Scrolled.DX, event.Scrolled.DY
	if dy != 0 && absoluteSlotWheelDelta(dy) >= absoluteSlotWheelDelta(dx) {
		forwarded := *event
		forwarded.Scrolled = fyne.Delta{DY: dy}
		o.vertical.Scrolled(&forwarded)
		return
	}
	if dx != 0 {
		forwarded := *event
		forwarded.Scrolled = fyne.Delta{DX: dx}
		o.horizontal.Scrolled(&forwarded)
	}
}

func absoluteSlotWheelDelta(delta float32) float32 {
	if delta < 0 {
		return -delta
	}
	return delta
}

// AttachPathSuggest wires PK3-aware inline autocomplete onto an existing
// Entry. As the user types, indexed paths matching the substring are
// shown as a SelectEntry-style dropdown of up to 50 suggestions.
// `accept` is an optional filter — return true to include a path
// (typically used to constrain to a suffix like ".skin" or a prefix
// like "models/players/"). Pass nil to suggest from every indexed path.
//
// This lives on the existing *widget.Entry so callers don't have to
// swap widget types (the Entry remains a regular Entry — keeps the
// surrounding ValidatedEntry/InputEntry plumbing intact). The trade-
// off vs converting to *widget.SelectEntry is that the dropdown only
// surfaces while focused and on text change, not via a click-arrow.
func AttachPathSuggest(entry *widget.Entry, vfs *VirtualFileSystem, accept func(string) bool) {
	if entry == nil || vfs == nil {
		return
	}
	prevOnChanged := entry.OnChanged
	var popup *widget.PopUpMenu

	hidePopup := func() {
		if popup != nil {
			popup.Hide()
			popup = nil
		}
	}

	showSuggestions := func(text string) {
		matches := vfs.Suggest(text, accept, 25)
		if len(matches) == 0 {
			hidePopup()
			return
		}
		canvas := fyne.CurrentApp().Driver().CanvasForObject(entry)
		if canvas == nil {
			return
		}
		items := make([]*fyne.MenuItem, len(matches))
		for i, m := range matches {
			path := m // capture
			items[i] = fyne.NewMenuItem(path, func() {
				// Setting text fires OnChanged. Suppress the popup
				// during that re-entry so the click-to-select doesn't
				// immediately spawn a new popup from the new text.
				prev := entry.OnChanged
				entry.OnChanged = nil
				entry.SetText(path)
				entry.OnChanged = prev
				hidePopup()
			})
		}
		hidePopup()
		popup = widget.NewPopUpMenu(fyne.NewMenu("", items...), canvas)
		// Position below the entry. Add the entry's height so the popup
		// doesn't overlap the field the user is typing into.
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(entry)
		pos.Y += entry.Size().Height
		popup.ShowAtPosition(pos)
	}

	entry.OnChanged = func(s string) {
		if prevOnChanged != nil {
			prevOnChanged(s)
		}
		if strings.TrimSpace(s) == "" {
			hidePopup()
			return
		}
		showSuggestions(s)
	}
}

// HasSuffixAny is a small helper for AttachPathSuggest's accept func —
// returns a closure that matches any of the listed suffixes
// case-insensitively. Lets callers write
// AttachPathSuggest(e, vfs, HasSuffixAny(".skin", ".jpg", ".png")).
func HasSuffixAny(suffixes ...string) func(string) bool {
	lower := make([]string, len(suffixes))
	for i, s := range suffixes {
		lower[i] = strings.ToLower(s)
	}
	return func(path string) bool {
		lp := strings.ToLower(path)
		for _, s := range lower {
			if strings.HasSuffix(lp, s) {
				return true
			}
		}
		return false
	}
}

func NewMultiLineInputEntry() *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.TextStyle = fyne.TextStyle{Monospace: true}
	return e
}

func NewPasswordInputEntry() *widget.Entry {
	e := widget.NewPasswordEntry()
	e.TextStyle = fyne.TextStyle{Monospace: true}
	return e
}

// NewFormSection groups a related editor form in the same restrained surface
// used throughout Foundry. It replaces Fyne's generic Card chrome so editors
// share spacing, typography, and accent behavior.
func NewFormSection(title, hint string, content fyne.CanvasObject) fyne.CanvasObject {
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.Wrapping = fyne.TextWrapWord
	objects := []fyne.CanvasObject{heading}
	if hint != "" {
		subtitle := widget.NewLabelWithStyle(hint, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
		subtitle.Wrapping = fyne.TextWrapWord
		objects = append(objects, subtitle)
	}
	objects = append(objects, content)
	return NewTilePanel(newFormColumns(1, objects...), TileOpts{
		FillAlpha:   9,
		StrokeAlpha: 38,
		Padded:      true,
	})
}

// newEditorContent measures a wrapping header at the current pane width before
// assigning the remaining height to its editor. A Border layout measures the
// header at its previous width, which can overlap tabs on the first narrow resize.
func newEditorContent(header, body fyne.CanvasObject) *fyne.Container {
	return container.New(&editorHeaderLayout{}, header, body)
}

type editorHeaderLayout struct{}

func (*editorHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	header, body := objects[0], objects[1]
	top := float32(0)
	if header.Visible() {
		header.Resize(fyne.NewSize(size.Width, header.MinSize().Height))
		height := header.MinSize().Height
		header.Resize(fyne.NewSize(size.Width, height))
		header.Move(fyne.NewPos(0, 0))
		top = height + theme.Padding()
	}
	body.Move(fyne.NewPos(0, top))
	body.Resize(fyne.NewSize(size.Width, fyne.Max(0, size.Height-top)))
}

func (*editorHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	header, body := objects[0], objects[1]
	min := body.MinSize()
	if header.Visible() {
		headerMin := header.MinSize()
		min.Width = fyne.Max(min.Width, headerMin.Width)
		min.Height += headerMin.Height + theme.Padding()
	}
	return min
}

// newFormColumns reflows related form groups without fixing their height.
// Unlike GridWrap, each row grows with wrapped labels and validation messages.
// The minimum is one column, allowing the editor rail to shrink again.
func newFormColumns(maxColumns int, objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&formColumnsLayout{maxColumns: maxColumns, columns: 1}, objects...)
}

type formColumnsLayout struct {
	maxColumns int
	columns    int
}

func (l *formColumnsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	padding := theme.Padding()
	columnMin := float32(280)
	visible := 0
	for _, object := range objects {
		if object.Visible() {
			columnMin = fyne.Max(columnMin, object.MinSize().Width)
			visible++
		}
	}
	columns := int((size.Width + padding) / (columnMin + padding))
	if columns > l.maxColumns {
		columns = l.maxColumns
	}
	if columns > visible {
		columns = visible
	}
	if columns < 1 {
		columns = 1
	}
	l.columns = columns
	width := fyne.Max(0, (size.Width-padding*float32(columns-1))/float32(columns))
	y := float32(0)
	for start := 0; start < len(objects); {
		end, count := start, 0
		height := float32(0)
		for end < len(objects) && count < columns {
			object := objects[end]
			end++
			if !object.Visible() {
				continue
			}
			// Width must reach the text renderer before measuring row height.
			object.Resize(fyne.NewSize(width, object.MinSize().Height))
			height = fyne.Max(height, object.MinSize().Height)
			count++
		}
		column := 0
		for _, object := range objects[start:end] {
			if !object.Visible() {
				continue
			}
			object.Resize(fyne.NewSize(width, height))
			object.Move(fyne.NewPos(float32(column)*(width+padding), y))
			column++
		}
		if count > 0 {
			y += height + padding
		}
		start = end
	}
}

func (l *formColumnsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var min fyne.Size
	rowHeight := float32(0)
	column, rows := 0, 0
	for _, object := range objects {
		if !object.Visible() {
			continue
		}
		childMin := object.MinSize()
		min.Width = fyne.Max(min.Width, childMin.Width)
		rowHeight = fyne.Max(rowHeight, childMin.Height)
		column++
		if column == l.columns {
			min.Height += rowHeight
			rows++
			column, rowHeight = 0, 0
		}
	}
	if column > 0 {
		min.Height += rowHeight
		rows++
	}
	if rows > 1 {
		min.Height += theme.Padding() * float32(rows-1)
	}
	return min
}
