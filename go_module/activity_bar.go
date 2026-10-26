package main

// SidebarHeader — horizontal activity-switcher that lives at the top
// of the left sidebar. Each activity is an icon+label "pill":
//
//   * active pill:   full opacity, filled background, accent color
//   * inactive pill: dimmed label, transparent background
//
// A collapse/expand toggle sits pinned to the right edge of the header.
// When the sidebar is expanded, the toggle uses a "push-to-left" icon;
// collapsed, it uses "pull-from-left". A collapsed edge rail keeps the
// control findable when the sidebar header is hidden.
//
// This replaces the old vertical icon strip (VS Code activity bar
// style). That pattern was unintuitive — users didn't know the icons
// swapped the sidebar content. Putting the switcher in the sidebar's
// own header makes the relationship obvious: the pill you select is
// the content you see.

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ActivityItem describes one sidebar activity. Label is the text
// rendered next to the icon in the pill; Tooltip is the hover hint.
type ActivityItem struct {
	ID      string
	Label   string
	Tooltip string
	Icon    fyne.Resource
	Content fyne.CanvasObject
}

type SidebarHeader struct {
	widget.BaseWidget

	items     []*ActivityItem
	activeID  string
	onSelect  func(*ActivityItem)
	onToggle  func()
	collapsed bool

	pills       map[string]*activityPill
	collapseBtn *activityPill
}

func NewSidebarHeader(items []*ActivityItem, onSelect func(*ActivityItem), onToggle func()) *SidebarHeader {
	s := &SidebarHeader{
		items:    items,
		onSelect: onSelect,
		onToggle: onToggle,
		pills:    make(map[string]*activityPill),
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetActive highlights the activity with this ID and fires onSelect.
// Idempotent.
func (s *SidebarHeader) SetActive(id string) {
	if s.activeID == id {
		return
	}
	s.activeID = id
	for pillID, pill := range s.pills {
		pill.setActive(pillID == id)
	}
	if s.onSelect != nil {
		for _, it := range s.items {
			if it.ID == id {
				s.onSelect(it)
				return
			}
		}
	}
}

func (s *SidebarHeader) ActiveID() string { return s.activeID }

// SetCollapsed swaps the collapse toggle's icon + tooltip to reflect
// the current sidebar visibility. Called by the host after toggling.
func (s *SidebarHeader) SetCollapsed(collapsed bool) {
	s.collapsed = collapsed
	if s.collapseBtn == nil {
		return
	}
	if collapsed {
		s.collapseBtn.setIcon(PanelExpandLeftIcon(), "Show sidebar")
	} else {
		s.collapseBtn.setIcon(PanelCollapseLeftIcon(), "Hide sidebar")
	}
}

func (s *SidebarHeader) CreateRenderer() fyne.WidgetRenderer {
	pillObjects := make([]fyne.CanvasObject, 0, len(s.items))
	for _, it := range s.items {
		pill := newActivityPill(it, s, false)
		s.pills[it.ID] = pill
		pillObjects = append(pillObjects, pill)
	}
	// Every activity receives an equal share of the available sidebar
	// width. The previous HBox summed four fixed 96px pills and clipped
	// the final activity in the normal 280–340px sidebar.
	tabRow := container.New(layout.NewGridLayoutWithColumns(len(pillObjects)), pillObjects...)

	collapseItem := &ActivityItem{
		ID:      "__collapse",
		Icon:    PanelCollapseLeftIcon(),
		Tooltip: "Hide sidebar",
	}
	s.collapseBtn = newActivityPill(collapseItem, s, true)

	s.SetCollapsed(s.collapsed)
	row := container.NewBorder(nil, nil, nil, s.collapseBtn, tabRow)
	return widget.NewSimpleRenderer(container.NewVBox(row, NewAccentRule()))
}

// --- individual pill --------------------------------------------------

type activityPill struct {
	widget.BaseWidget

	item   *ActivityItem
	parent *SidebarHeader

	isToggle bool // true for the collapse/expand button (icon-only)

	active   bool
	hovering bool
	focused  bool

	bg    *canvas.Rectangle
	icon  *widget.Icon
	label *canvas.Text
	popUp *widget.PopUp
}

func newActivityPill(item *ActivityItem, parent *SidebarHeader, isToggle bool) *activityPill {
	p := &activityPill{
		item: item, parent: parent, isToggle: isToggle,
		active: !isToggle && parent.activeID == item.ID,
	}
	p.ExtendBaseWidget(p)
	return p
}

func (p *activityPill) CreateRenderer() fyne.WidgetRenderer {
	p.bg = canvas.NewRectangle(color.Transparent)
	p.bg.StrokeWidth = 2
	p.icon = widget.NewIcon(p.item.Icon)

	var content fyne.CanvasObject
	if p.isToggle {
		content = container.NewPadded(container.NewCenter(p.icon))
	} else {
		p.label = canvas.NewText(p.item.Label, theme.PlaceHolderColor())
		p.label.TextSize = SizeSmall
		p.label.TextStyle = fyne.TextStyle{Bold: true}
		content = container.New(&activityPillLayout{pill: p}, p.icon, p.label)
	}

	p.applyStyle()
	return widget.NewSimpleRenderer(container.NewStack(p.bg, content))
}

func (p *activityPill) MinSize() fyne.Size {
	// Icon-only pills use the same target as the collapse control. The
	// grid expands them evenly; hidden labels must not pin a narrow rail.
	return fyne.NewSize(36, 36)
}

type activityPillLayout struct {
	pill *activityPill
}

func (l *activityPillLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return l.pill.MinSize()
}

func (l *activityPillLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	p := l.pill
	iconSize, labelSize := p.icon.MinSize(), p.label.MinSize()
	padding := theme.Padding()
	// Switch the whole tab row to icons at normal sidebar widths.
	// Also guard longer labels rather than drawing beyond the pill.
	compact := size.Width < 112 || size.Width < iconSize.Width+labelSize.Width+3*padding
	p.icon.Resize(iconSize)
	if compact {
		p.label.Hide()
		p.icon.Move(fyne.NewPos((size.Width-iconSize.Width)/2, (size.Height-iconSize.Height)/2))
		return
	}
	p.label.Show()
	p.icon.Move(fyne.NewPos(padding, (size.Height-iconSize.Height)/2))
	p.label.Resize(labelSize)
	p.label.Move(fyne.NewPos(2*padding+iconSize.Width, (size.Height-labelSize.Height)/2))
}

func (p *activityPill) setIcon(icon fyne.Resource, tooltip string) {
	p.item.Icon = icon
	p.item.Tooltip = tooltip
	if p.icon != nil {
		p.icon.SetResource(icon)
	}
}

func (p *activityPill) setActive(active bool) {
	p.active = active
	p.applyStyle()
}

// applyStyle paints resting / hover / active states. The spec from
// the user: active pill has full opacity fill, inactive pills have
// slight opacity. Hover is a halfway state so there's immediate
// feedback on mousemove.
func (p *activityPill) applyStyle() {
	if p.bg == nil {
		return
	}
	switch {
	case p.isToggle:
		if p.hovering {
			p.bg.FillColor = tintWithAlpha(CurrentThemeColor, 45)
		} else {
			p.bg.FillColor = color.Transparent
		}
	case p.active:
		p.bg.FillColor = tintWithAlpha(CurrentThemeColor, 85)
	case p.hovering:
		p.bg.FillColor = tintWithAlpha(CurrentThemeColor, 38)
	default:
		p.bg.FillColor = color.Transparent
	}
	p.bg.StrokeColor = color.Transparent
	if p.focused {
		p.bg.StrokeColor = theme.FocusColor()
	}
	p.bg.Refresh()

	if p.label != nil {
		if p.active {
			p.label.Color = theme.ForegroundColor()
		} else {
			p.label.Color = theme.PlaceHolderColor()
		}
		p.label.Refresh()
	}
}

func (p *activityPill) Tapped(*fyne.PointEvent) {
	if p.isToggle {
		if p.parent.onToggle != nil {
			p.parent.onToggle()
		}
		return
	}
	p.parent.SetActive(p.item.ID)
}

func (p *activityPill) FocusGained() {
	p.focused = true
	p.applyStyle()
}

func (p *activityPill) FocusLost() {
	p.focused = false
	p.applyStyle()
}

func (p *activityPill) TypedRune(r rune) {
	if r == ' ' {
		p.Tapped(nil)
	}
}

func (p *activityPill) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		p.Tapped(nil)
	}
}

func (p *activityPill) MouseIn(*desktop.MouseEvent) {
	p.hovering = true
	p.applyStyle()
	if p.popUp != nil {
		return
	}
	go func() {
		time.Sleep(tooltipHoverDelay)
		fyne.Do(func() {
			if !p.hovering || p.popUp != nil {
				return
			}
			c := fyne.CurrentApp().Driver().CanvasForObject(p)
			if c == nil {
				return
			}
			text := p.item.Tooltip
			if p.item.Label != "" {
				text = p.item.Label + "\n" + text
			}
			p.popUp = widget.NewPopUp(widget.NewLabel(text), c)
			p.popUp.ShowAtRelativePosition(fyne.NewPos(0, p.Size().Height), p)
		})
	}()
}
func (p *activityPill) MouseOut() {
	p.hovering = false
	p.applyStyle()
	if p.popUp != nil {
		p.popUp.Hide()
		p.popUp = nil
	}
}
func (p *activityPill) MouseMoved(*desktop.MouseEvent) {}

func (p *activityPill) Cursor() desktop.Cursor { return desktop.PointerCursor }
