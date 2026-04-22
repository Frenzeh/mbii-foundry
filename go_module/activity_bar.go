package main

// SidebarHeader — horizontal activity-switcher that lives at the top
// of the left sidebar. Each activity is an icon+label "pill":
//
//   * active pill:   full opacity, filled background, accent color
//   * inactive pill: dimmed label, transparent background
//
// A collapse/expand toggle sits pinned to the right edge of the header.
// When the sidebar is expanded, the toggle uses a "push-to-left" icon;
// collapsed, it uses "pull-from-left". The toolbar surfaces the same
// toggle (for when the sidebar is fully hidden and there's no header
// to click), keeping the control findable in every state.
//
// This replaces the old vertical icon strip (VS Code activity bar
// style). That pattern was unintuitive — users didn't know the icons
// swapped the sidebar content. Putting the switcher in the sidebar's
// own header makes the relationship obvious: the pill you select is
// the content you see.

import (
	"image/color"

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
	// Thin gaps between pills so they read as distinct items rather
	// than a solid tab bar.
	tabRow := container.New(layout.NewHBoxLayout(), pillObjects...)

	collapseItem := &ActivityItem{
		ID:      "__collapse",
		Icon:    PanelCollapseLeftIcon(),
		Tooltip: "Hide sidebar",
	}
	s.collapseBtn = newActivityPill(collapseItem, s, true)

	// Collapse toggle pinned to the far LEFT of the header — the
	// sidebar itself is on the left of the window, so a left-edge
	// "push this out" button reads more directly than one on the
	// right. Pills sit immediately to its right.
	row := container.NewBorder(nil, nil, s.collapseBtn, nil, tabRow)

	// Bottom rule — thin accent line under the whole header so it
	// reads as its own band. AccentRule so it repaints on theme swap.
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

	bg    *canvas.Rectangle
	icon  *widget.Icon
	label *canvas.Text
}

func newActivityPill(item *ActivityItem, parent *SidebarHeader, isToggle bool) *activityPill {
	p := &activityPill{item: item, parent: parent, isToggle: isToggle}
	p.ExtendBaseWidget(p)
	return p
}

func (p *activityPill) CreateRenderer() fyne.WidgetRenderer {
	p.bg = canvas.NewRectangle(color.Transparent)
	p.icon = widget.NewIcon(p.item.Icon)

	var content fyne.CanvasObject
	if p.isToggle {
		content = container.NewPadded(container.NewCenter(p.icon))
	} else {
		p.label = canvas.NewText(p.item.Label, theme.PlaceHolderColor())
		p.label.TextSize = SizeSmall
		p.label.TextStyle = fyne.TextStyle{Bold: true}
		row := container.NewHBox(p.icon, p.label)
		content = container.NewPadded(row)
	}

	p.applyStyle()
	return widget.NewSimpleRenderer(container.NewStack(p.bg, content))
}

func (p *activityPill) MinSize() fyne.Size {
	if p.isToggle {
		return fyne.NewSize(34, 32)
	}
	return fyne.NewSize(96, 32)
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
		p.bg.FillColor = tintWithAlpha(CurrentThemeColor, 140)
	case p.hovering:
		p.bg.FillColor = tintWithAlpha(CurrentThemeColor, 50)
	default:
		// Inactive resting: a whisper of fill so the pill is still
		// findable when the user isn't hovering — feels more like
		// "a disabled tab" than "invisible affordance."
		p.bg.FillColor = color.NRGBA{R: 255, G: 255, B: 255, A: 8}
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

func (p *activityPill) MouseIn(*desktop.MouseEvent) {
	p.hovering = true
	p.applyStyle()
}
func (p *activityPill) MouseOut() {
	p.hovering = false
	p.applyStyle()
}
func (p *activityPill) MouseMoved(*desktop.MouseEvent) {}

func (p *activityPill) Cursor() desktop.Cursor { return desktop.PointerCursor }
