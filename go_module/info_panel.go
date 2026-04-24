package main

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type InfoPanel struct {
	container *fyne.Container

	// --- Header zone --------------------------------------------------
	// "Hero" strip at top of the Context tab. Loose Swiss-ish grid:
	// category chip pinned left, monospace ID pinned right, bold title
	// below; tinted rectangle behind the whole block so it reads as a
	// distinct identity band regardless of scroll position.
	headerBG        *canvas.Rectangle // faint accent-tinted fill
	headerFrame     *canvas.Rectangle // 1px accent border for a solid brutalist edge
	categoryChip    *canvas.Text      // small caps, tinted
	idChip          *canvas.Text      // monospace, muted
	title           *widget.Label     // large, bold
	// Accent marker + rule below the header — the small square + thin
	// line is the sci-fi note. Both use accent color.
	headerMarker *canvas.Rectangle
	headerRule   *canvas.Rectangle

	content *widget.RichText // markdown body
	search  *widget.Entry
	tabs    *container.AppTabs

	// Library is a per-type Accordion — Attributes / Weapons / Classes
	// / Class Flags / Saber Styles / Glossary. Splitting by type cleans
	// up the old mega-list that jumbled everything into one 500-row
	// scroll where "Pistol" the weapon and "Pistol" the attribute sat
	// right next to each other with no indication of which was which.
	library       *widget.Accordion
	libraryGroups []*libraryGroup

	holocronClient *HolocronClient

	// onPopOut, if set, opens this panel's content in a new window
	// (dual-monitor workflow). The pop-out button on the hero band
	// invokes it; the App wires this to a method that creates a fresh
	// InfoPanel mirror so the new window updates live as the user
	// edits in the main window.
	onPopOut func()

	// Sticky state = what the user last INTERACTED with (clicked,
	// focused, edited). Hover state is transient — appears while
	// the mouse is over a target, reverts to sticky on mouse-out.
	// The user's mental model is: "the panel shows what I'm
	// currently working on, unless I'm peeking at something else."
	stickyKey     string
	stickyContext string
	// showingHover tells ClearHover whether there's anything to revert
	// from — lets us avoid re-rendering sticky when the panel is
	// already showing sticky.
	showingHover bool
}

// libraryGroup bundles the per-type state for a single Library
// accordion section — its slice of keys, the List widget that renders
// them, and the AccordionItem that wraps the list. Refresh rebuilds
// the slice in place and refreshes both the list and the parent item
// title (so counts stay in sync with the filter).
//
// When subGroups is non-nil the group renders an inner Accordion of
// per-category sub-sections instead of a single flat List. Used by
// the Attributes section since the ~200-entry list reads better as
// a grouped tree than a flat scroll.
type libraryGroup struct {
	title     string
	keys      []string
	source    func(filter string) []string
	list      *widget.List
	item      *widget.AccordionItem
	subGroups []*librarySubGroup
	subHost   *widget.Accordion
}

type librarySubGroup struct {
	title  string
	keys   []string
	source func(filter string) []string
	list   *widget.List
	item   *widget.AccordionItem
}

// categoryTagFor builds the small-caps chip text for the info panel
// header. Attributes get a "ATTRIBUTE · FORCE" style subtag so the
// reader sees both the enum kind and its bucket (Force / Weapons /
// Saber / Advanced / etc.) without having to infer from the ID.
func categoryTagFor(kind, bucket string) string {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	bucket = strings.ToUpper(strings.TrimSpace(bucket))
	if bucket == "" || bucket == "GENERAL" {
		return kind
	}
	return kind + " · " + bucket
}

// updateHeaderChips sets the hero band's ID chip + category chip.
// Called from ShowInfo after the def is resolved; welcome / not-found
// paths pass category="REFERENCE" and id="" so the band reads as a
// minimal title-only block rather than showing a blank "ATTRIBUTE · "
// chip above the welcome copy.
func (ip *InfoPanel) updateHeaderChips(id, category string) {
	if ip.idChip != nil {
		ip.idChip.Text = id
		ip.idChip.Refresh()
	}
	if ip.categoryChip != nil {
		up := strings.ToUpper(category)
		if up == "REFERENCE" {
			// Welcome/not-found — drop the chip entirely so the title
			// reads cleanly without a label that contradicts itself.
			up = ""
		}
		ip.categoryChip.Text = up
		ip.categoryChip.Refresh()
	}
}

// isHiddenLibraryKey reports whether a Definitions-map key names an
// ID that is in the hidden set. Used to filter the Library's legacy
// catch-all so private-overlay markdown doesn't leak into the glossary
// (e.g. a dev-installed MB_ATT_FP_STASIS.md shouldn't appear in the
// Library just because it was loaded from private/definitions/).
func isHiddenLibraryKey(key string) bool {
	// Keys come in two flavors: bare filename ("MB_ATT_FP_STASIS") and
	// relative path ("attributes/MB_ATT_FP_STASIS"). Normalize by
	// taking the basename.
	base := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		base = key[i+1:]
	}
	if hiddenAttributeIDs[base] {
		return true
	}
	if hiddenWeaponIDs[base] {
		return true
	}
	if hiddenClassIDs[base] {
		return true
	}
	// Force-power .md filenames are FP_* (no MB_ATT_ prefix); map back
	// to the attribute form for the lookup.
	if strings.HasPrefix(base, "FP_") && hiddenAttributeIDs["MB_ATT_"+base] {
		return true
	}
	// Weapon markdowns keyed by WP_* — already caught by hiddenWeaponIDs.
	return false
}

func NewInfoPanel() *InfoPanel {
	ip := &InfoPanel{}
	ip.createUI()
	return ip
}

func (ip *InfoPanel) SetHolocronClient(client *HolocronClient) {
	ip.holocronClient = client
}

// SetOnPopOut wires the pop-out button on the hero band. When the
// caller wants pop-out support (e.g. the main App), they pass a
// callback that opens a new window with a mirror panel; otherwise
// the button is still rendered but does nothing useful — App always
// wires it, so this only no-ops in test contexts.
func (ip *InfoPanel) SetOnPopOut(cb func()) {
	ip.onPopOut = cb
}

func (ip *InfoPanel) createUI() {
	// Title — large, bold, wrapping. Sits inside the header hero below.
	ip.title = widget.NewLabelWithStyle("Information", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	ip.title.Wrapping = fyne.TextWrapWord

	// Category chip — small-caps accent-colored label above the title.
	// Tells the reader what KIND of thing they're looking at (attribute,
	// weapon, force power, class, etc.) without needing the body to say so.
	ip.categoryChip = canvas.NewText("", CurrentThemeColor)
	ip.categoryChip.TextSize = SizeSmall
	ip.categoryChip.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	// ID chip — monospace enum id pinned right. Cues the reader into the
	// machine-facing name so they know what to type in source.
	ip.idChip = canvas.NewText("", theme.PlaceHolderColor())
	ip.idChip.TextSize = SizeSmall
	ip.idChip.TextStyle = fyne.TextStyle{Monospace: true}
	ip.idChip.Alignment = fyne.TextAlignTrailing

	// Hero background — faint accent tint, rounded corners to echo
	// the MBII launcher's panel style. CornerRadius > 0 + filled
	// rectangle + an inset stroke (drawn as a separate rect inside a
	// Padded container) give the offset-stroke look the launcher uses
	// for its boxes and buttons.
	ip.headerBG = canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 22))
	ip.headerBG.CornerRadius = 6
	ip.headerFrame = canvas.NewRectangle(color.Transparent)
	ip.headerFrame.StrokeColor = tintWithAlpha(CurrentThemeColor, 110)
	ip.headerFrame.StrokeWidth = 1
	ip.headerFrame.CornerRadius = 5

	// Accent marker + rule — small filled square left, thin rule right.
	ip.headerMarker = canvas.NewRectangle(CurrentThemeColor)
	ip.headerMarker.SetMinSize(fyne.NewSize(8, 8))
	ip.headerRule = canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 120))
	ip.headerRule.SetMinSize(fyne.NewSize(0, 1))

	welcomeMsg := `
### Info Panel

This panel provides real-time documentation and context for the field you're editing.

**How to use:**

*   **Hover** over any attribute or weapon in the editor to see its description here.
*   **Switch** to the "Library" tab above to browse all available topics.
`
	ip.content = widget.NewRichTextFromMarkdown(welcomeMsg)
	// Wrap at word boundaries — content reflows to whatever width
	// the sidebar is currently at. The bi-directional Scroll below
	// absorbs any MinSize contribution so long tokens (MB_ATT_ARC_
	// RIFLE_GRENADELAUNCHER, URLs) don't pin the HSplit divider.
	ip.content.Wrapping = fyne.TextWrapWord

	ip.search = NewInputEntry()
	ip.search.SetPlaceHolder("Search help...")
	ip.search.OnChanged = ip.filterList

	ip.library = widget.NewAccordion()
	ip.library.MultiOpen = true
	ip.buildLibraryGroups()
	ip.refreshKeys("")

	// ── Hero band ─────────────────────────────────────────────────
	// Launcher-style rounded panel: tinted fill at full extent, stroke
	// inset by 3px via a Padded wrapper so the 1px accent border sits
	// inside the bg rather than flush with its edge. That inset is the
	// "offset stroke" look the MBII launcher boxes use.
	popOutBtn := widget.NewButtonWithIcon("", theme.ComputerIcon(), func() {
		if ip.onPopOut != nil {
			ip.onPopOut()
		}
	})
	popOutBtn.Importance = widget.LowImportance
	rightChip := container.NewHBox(ip.idChip, popOutBtn)
	heroRow := container.NewBorder(nil, nil,
		ip.categoryChip,
		rightChip,
		nil,
	)
	heroInner := container.NewPadded(container.NewPadded(container.NewVBox(
		heroRow,
		ip.title,
	)))
	// Double-wrap the frame: Padded insets the frame rectangle 4px
	// from the bg's edges so the stroke reads as its own ring instead
	// of sharing the bg's silhouette.
	framePadded := container.NewPadded(ip.headerFrame)
	hero := container.NewStack(ip.headerBG, framePadded, heroInner)

	// Double-offset rule — two parallel accent lines of different
	// lengths + a filled square marker pinned left. The first (longer)
	// rule hangs off the left edge with the marker; the second
	// (shorter, dimmer) starts indented 48px and lines up underneath,
	// giving a mild "technical diagram" air without being noisy.
	markerBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(8, 8)), ip.headerMarker)
	primaryRule := container.NewBorder(nil, nil, markerBox, nil, ip.headerRule)
	secondaryLine := canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 55))
	secondaryLine.SetMinSize(fyne.NewSize(0, 1))
	secondaryIndent := canvas.NewRectangle(color.Transparent)
	secondaryIndent.SetMinSize(fyne.NewSize(48, 1))
	secondaryRule := container.NewBorder(nil, nil, secondaryIndent, nil, secondaryLine)
	spacerBetweenRules := canvas.NewRectangle(color.Transparent)
	spacerBetweenRules.SetMinSize(fyne.NewSize(0, 3))
	rule := container.NewVBox(primaryRule, spacerBetweenRules, secondaryRule)

	spacerTop := canvas.NewRectangle(color.Transparent)
	spacerTop.SetMinSize(fyne.NewSize(0, 10))
	spacerBottom := canvas.NewRectangle(color.Transparent)
	spacerBottom.SetMinSize(fyne.NewSize(0, 14))

	details := container.NewVBox(
		hero,
		spacerTop,
		rule,
		spacerBottom,
		container.NewPadded(ip.content),
	)
	detailsScroll := container.NewScroll(details)
	detailsScroll.SetMinSize(fyne.NewSize(120, 0))

	listHeader := widget.NewLabelWithStyle("Reference Library", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	libraryScroll := container.NewVScroll(ip.library)
	listContainer := container.NewBorder(container.NewVBox(listHeader, ip.search), nil, nil, nil, libraryScroll)

	ip.tabs = container.NewAppTabs(
		container.NewTabItem("Context", detailsScroll),
		container.NewTabItem("Library", listContainer),
	)

	ip.container = container.NewMax(ip.tabs)
}

// buildLibraryGroups wires up the per-type accordion sections once.
// Called from createUI; subsequent updates just refresh each group's
// key slice + list. Each group's source closure owns its own data
// lookup — keeps the filter logic declarative per section instead of
// stuffing 6 types of enumeration into one method.
func (ip *InfoPanel) buildLibraryGroups() {
	match := func(s, filter string) bool {
		return filter == "" || strings.Contains(strings.ToLower(s), filter)
	}

	// Attributes sub-groups — mirror the category order used in the
	// editor's attribute grid so the Library's categorization reads
	// the same way the author sees attributes elsewhere in the app.
	attrCategoryOrder := []string{
		"Weapons", "Force", "Saber", "Class Specific",
		"Supply", "Regen", "Multipliers", "Advanced", "General",
	}
	var attrSubs []*librarySubGroup
	for _, cat := range attrCategoryOrder {
		cat := cat // closure capture
		attrSubs = append(attrSubs, &librarySubGroup{
			title: cat,
			source: func(filter string) []string {
				var out []string
				for _, a := range GetAttributes() {
					if a.Category != cat {
						continue
					}
					if match(a.Name, filter) || match(a.ID, filter) {
						out = append(out, a.Name)
					}
				}
				sort.Strings(out)
				return out
			},
		})
	}

	ip.libraryGroups = []*libraryGroup{
		{
			title:     "Attributes",
			subGroups: attrSubs,
		},
		{
			title: "Weapons",
			source: func(filter string) []string {
				var out []string
				for _, w := range GetWeapons() {
					if match(w.Name, filter) || match(w.ID, filter) {
						out = append(out, w.Name)
					}
				}
				sort.Strings(out)
				return out
			},
		},
		{
			title: "Classes",
			source: func(filter string) []string {
				var out []string
				for _, c := range GetClasses() {
					if match(c.Name, filter) || match(c.ID, filter) {
						out = append(out, c.Name)
					}
				}
				sort.Strings(out)
				return out
			},
		},
		{
			title: "Class Flags",
			source: func(filter string) []string {
				var out []string
				for _, f := range GetClassFlags() {
					if match(f.Name, filter) || match(f.ID, filter) {
						out = append(out, f.Name)
					}
				}
				sort.Strings(out)
				return out
			},
		},
		{
			title: "Saber Styles",
			source: func(filter string) []string {
				var out []string
				for _, s := range GetSaberStyles() {
					if match(s.Name, filter) || match(s.ID, filter) {
						out = append(out, s.Name)
					}
				}
				sort.Strings(out)
				return out
			},
		},
		{
			title: "Glossary",
			source: func(filter string) []string {
				var out []string
				for _, g := range GetGlossary() {
					if match(g.Name, filter) || match(g.ID, filter) {
						out = append(out, g.Name)
					}
				}
				// Legacy Definitions map — raw .md filenames that don't
				// belong to the typed getters. Dumped into Glossary as a
				// catch-all so they stay reachable without crowding the
				// top-level sections. Filter out any key that names a
				// hidden ID so private-overlay markdown doesn't surface
				// here on dev machines where the overlay is installed.
				DefinitionsLock.RLock()
				for k := range Definitions {
					if isHiddenLibraryKey(k) {
						continue
					}
					if match(k, filter) {
						out = append(out, k)
					}
				}
				DefinitionsLock.RUnlock()
				sort.Strings(out)
				return out
			},
		},
	}

	for _, g := range ip.libraryGroups {
		group := g // closure capture
		if len(group.subGroups) > 0 {
			// Sub-accordion layout — e.g. Attributes by category.
			group.subHost = widget.NewAccordion()
			group.subHost.MultiOpen = true
			for _, sg := range group.subGroups {
				sub := sg
				sub.list = widget.NewList(
					func() int { return len(sub.keys) },
					func() fyne.CanvasObject { return widget.NewLabel("Topic") },
					func(id widget.ListItemID, obj fyne.CanvasObject) {
						if id < len(sub.keys) {
							obj.(*widget.Label).SetText(sub.keys[id])
						}
					},
				)
				sub.list.OnSelected = func(id widget.ListItemID) {
					if id < len(sub.keys) {
						ip.ShowSticky(sub.keys[id], "")
						ip.tabs.SelectIndex(0)
					}
					sub.list.UnselectAll()
				}
				scroll := container.NewVScroll(sub.list)
				scroll.SetMinSize(fyne.NewSize(0, 200))
				sub.item = widget.NewAccordionItem(sub.title, scroll)
				group.subHost.Append(sub.item)
			}
			group.item = widget.NewAccordionItem(group.title, group.subHost)
			ip.library.Append(group.item)
			continue
		}
		group.list = widget.NewList(
			func() int { return len(group.keys) },
			func() fyne.CanvasObject { return widget.NewLabel("Topic") },
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				if id < len(group.keys) {
					obj.(*widget.Label).SetText(group.keys[id])
				}
			},
		)
		group.list.OnSelected = func(id widget.ListItemID) {
			if id < len(group.keys) {
				ip.ShowSticky(group.keys[id], "")
				ip.tabs.SelectIndex(0)
			}
			group.list.UnselectAll()
		}
		// Each list needs a concrete height so it renders inside the
		// accordion; List with no MinSize collapses to 0px. 240 fits
		// ~10 rows — enough to scan without dominating the sidebar.
		scroll := container.NewVScroll(group.list)
		scroll.SetMinSize(fyne.NewSize(0, 240))
		group.item = widget.NewAccordionItem(group.title, scroll)
		ip.library.Append(group.item)
	}
}

func (ip *InfoPanel) refreshKeys(filter string) {
	filter = strings.ToLower(filter)
	filtering := filter != ""
	for _, g := range ip.libraryGroups {
		if len(g.subGroups) > 0 {
			total := 0
			for _, sg := range g.subGroups {
				sg.keys = sg.source(filter)
				sg.item.Title = fmt.Sprintf("%s (%d)", sg.title, len(sg.keys))
				if filtering && len(sg.keys) > 0 {
					sg.item.Open = true
				}
				sg.list.Refresh()
				total += len(sg.keys)
			}
			g.item.Title = fmt.Sprintf("%s (%d)", g.title, total)
			if filtering && total > 0 {
				g.item.Open = true
			}
			g.subHost.Refresh()
			continue
		}
		g.keys = g.source(filter)
		g.item.Title = fmt.Sprintf("%s (%d)", g.title, len(g.keys))
		// Auto-expand any section that has matches under an active
		// filter — surfaces results without forcing a click. When the
		// filter clears, leave collapse state alone (don't re-close
		// sections the user opened manually).
		if filtering && len(g.keys) > 0 {
			g.item.Open = true
		}
		g.list.Refresh()
	}
	ip.library.Refresh()
}

func (ip *InfoPanel) filterList(text string) {
	ip.refreshKeys(text)
}

func (ip *InfoPanel) GetContent() fyne.CanvasObject {
	return ip.container
}

// ShowSticky is called when the user has INTERACTED with a field
// (clicked, focused, edited). Saves the key as the panel's sticky
// view and renders it. Subsequent hover-outs revert here.
func (ip *InfoPanel) ShowSticky(key, context string) {
	if key == "" {
		return
	}
	ip.stickyKey = key
	ip.stickyContext = context
	ip.showingHover = false
	ip.ShowInfo(key, context)
}

// ShowHover is called on mouseover of a hoverable target. Renders
// the hover content without mutating sticky state. ClearHover will
// restore whatever sticky was showing.
func (ip *InfoPanel) ShowHover(key, context string) {
	if key == "" {
		return
	}
	// Don't clobber the sticky render if the hover is actually the
	// same key — avoids a flicker when the user mouses off and back
	// onto the same row.
	if ip.stickyKey == key && ip.stickyContext == context && !ip.showingHover {
		return
	}
	ip.showingHover = true
	ip.ShowInfo(key, context)
}

// ClearHover reverts the panel to its last-interacted (sticky)
// state. Called on mouse-out of hover targets. Noop when nothing
// sticky has been set (e.g. app just launched, user hasn't touched
// a field yet) — the panel just stays on whatever hover was
// showing until the user interacts.
func (ip *InfoPanel) ClearHover() {
	if !ip.showingHover {
		return
	}
	ip.showingHover = false
	if ip.stickyKey == "" {
		return
	}
	ip.ShowInfo(ip.stickyKey, ip.stickyContext)
}

func (ip *InfoPanel) ShowInfo(key, context string) {
	LogInfo("InfoPanel: ShowInfo called for key='%s'", key)

	// Tab auto-switch was here and caused layout jitter on every
	// hover: switching the active tab recalcs AppTabs layout, which
	// cascaded into HSplit min-size updates and made the sidebar
	// rail visibly shift whenever the user moused over a new field.
	// Left behind for the Library list's OnSelected to invoke
	// explicitly — that's the only flow that NEEDS a tab switch
	// (user clicked a key in the Library and expects to see its
	// content). Hovers just mutate text.

	var def string
	var found bool

	// Before anything else, check the markdown file for this key. If the
	// .md has more than a stub's worth of content, it's the canonical
	// source of truth — JSON `overview`/`description` fields lose to it.
	// This lets non-coders edit definitions/*.md via GitHub's web UI and
	// see their changes show up in the app, instead of getting shadowed
	// by stale or thin JSON content.
	preferMarkdown := func(preferredKey string) (string, bool) {
		const stubBytes = 200
		const stubMarker = "*Stub — a human needs to document this.*"
		md, ok := GetDefinition(preferredKey)
		if !ok || md == "" {
			return "", false
		}
		if strings.Contains(md, stubMarker) {
			return "", false
		}
		if len(md) < stubBytes {
			return "", false
		}
		return md, true
	}

	// Helper to format rich doc
	formatRich := func(desc, overview string, tips []string, levels map[string]LevelDoc, stats map[string]string, tags []string) string {
		var sb strings.Builder

		// Check for specific Level context
		if strings.HasPrefix(context, "Level ") {
			lvlStr := strings.TrimPrefix(context, "Level ")
			if lvlInfo, ok := levels[lvlStr]; ok {
				sb.WriteString(fmt.Sprintf("# %s\n", lvlInfo.Name))
				sb.WriteString(lvlInfo.Effect + "\n\n")
				if lvlInfo.FPCost > 0 {
					sb.WriteString(fmt.Sprintf("**FP Cost:** %d\n", lvlInfo.FPCost))
				}
				if lvlInfo.Tip != "" {
					sb.WriteString(fmt.Sprintf("> *%s*\n", lvlInfo.Tip))
				}
				sb.WriteString("\n---\n\n")
			}
		}

		if overview != "" {
			sb.WriteString(overview + "\n\n")
		} else {
			sb.WriteString(desc + "\n\n")
		}

		// New: Stats section
		if len(stats) > 0 {
			sb.WriteString("### Stats\n")
			for k, v := range stats {
				sb.WriteString(fmt.Sprintf("* **%s:** %s\n", k, v))
			}
			sb.WriteString("\n")
		}

		// Show all levels if no specific one selected, or just summary?
		if !strings.HasPrefix(context, "Level ") && len(levels) > 0 {
			sb.WriteString("### Levels\n")
			var lvls []int
			for k := range levels {
				if v, err := strconv.Atoi(k); err == nil {
					lvls = append(lvls, v)
				}
			}
			sort.Ints(lvls)
			for _, l := range lvls {
				lStr := strconv.Itoa(l)
				info := levels[lStr]
				sb.WriteString(fmt.Sprintf("* **%s**: %s", info.Name, info.Effect))
				if info.FPCost > 0 {
					sb.WriteString(fmt.Sprintf(" (**FP Cost:** %d)", info.FPCost))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		if len(tips) > 0 {
			sb.WriteString("### Tips\n")
			for _, t := range tips {
				sb.WriteString("* " + t + "\n")
			}
		}

		if len(tags) > 0 {
			sb.WriteString("\n---\n")
			for _, t := range tags {
				sb.WriteString(fmt.Sprintf("`#%s` ", t))
			}
			sb.WriteString("\n")
		}

		return sb.String()
	}

	// Treat an empty/placeholder JSON description as "not found" so the
	// markdown fallback runs. Rich .md files like
	// definitions/attributes/MB_ATT_BESKAR.md have real prose that would
	// otherwise lose out to a blank JSON row.
	hasContent := func(desc, overview string) bool {
		return strings.TrimSpace(desc) != "" || strings.TrimSpace(overview) != ""
	}

	// 1. JSON Data Lookup (Priority) — but markdown wins when richer.
	var resolvedID, resolvedCategory string
	for _, attr := range GetAttributes() {
		if attr.ID == key || attr.Name == key {
			resolvedID = attr.ID
			resolvedCategory = categoryTagFor("ATTRIBUTE", attr.Category)
			if md, ok := preferMarkdown(attr.ID); ok {
				def = md
				key = attr.Name
				found = true
			} else if hasContent(attr.Description, attr.Overview) {
				def = formatRich(attr.Description, attr.Overview, attr.Tips, attr.Levels, nil, attr.Tags)
				key = attr.Name
				found = true
			}
			break
		}
	}
	if !found {
		for _, w := range GetWeapons() {
			if w.ID == key || w.Name == key {
				resolvedID = w.ID
				resolvedCategory = "WEAPON"
				if md, ok := preferMarkdown(w.ID); ok {
					def = md
					key = w.Name
					found = true
				} else if hasContent(w.Description, w.Overview) {
					def = formatRich(w.Description, w.Overview, w.Tips, nil, w.Stats, w.Tags)
					key = w.Name
					found = true
				}
				break
			}
		}
	}
	if !found {
		for _, c := range GetClasses() {
			if c.ID == key || c.Name == key {
				resolvedID = c.ID
				resolvedCategory = "CLASS"
				if md, ok := preferMarkdown(c.ID); ok {
					def = md
					key = c.Name
					found = true
				} else if strings.TrimSpace(c.Description) != "" {
					def = c.Description
					key = c.Name
					found = true
				}
				break
			}
		}
	}
	if !found {
		for _, f := range GetClassFlags() {
			if f.ID == key || f.Name == key {
				resolvedID = f.ID
				resolvedCategory = "CLASS FLAG"
				if md, ok := preferMarkdown(f.ID); ok {
					def = md
					key = f.Name
					found = true
				} else if hasContent(f.Description, f.Overview) {
					def = formatRich(f.Description, f.Overview, nil, nil, nil, nil)
					key = f.Name
					found = true
				}
				break
			}
		}
	}
	if !found {
		for _, s := range GetSaberStyles() {
			if s.ID == key || s.Name == key {
				resolvedID = s.ID
				resolvedCategory = "SABER STYLE"
				if md, ok := preferMarkdown(s.ID); ok {
					def = md
					key = s.Name
					found = true
				} else if hasContent(s.Description, s.Overview) {
					def = formatRich(s.Description, s.Overview, nil, nil, nil, nil)
					key = s.Name
					found = true
				}
				break
			}
		}
	}
	if !found {
		for _, g := range GetGlossary() {
			if g.ID == key || g.Name == key {
				resolvedID = g.ID
				resolvedCategory = "GLOSSARY"
				if md, ok := preferMarkdown(g.ID); ok {
					def = md
					key = g.Name
					found = true
				} else if hasContent(g.Description, g.Overview) {
					def = formatRich(g.Description, g.Overview, nil, nil, nil, nil)
					key = g.Name
					found = true
				}
				break
			}
		}
	}

	// 1c. Heuristic Lookup (FP_ -> MB_ATT_FP_)
	if !found && strings.HasPrefix(key, "FP_") {
		altKey := "MB_ATT_" + key
		for _, attr := range GetAttributes() {
			if attr.ID == altKey {
				resolvedID = attr.ID
				if hasContent(attr.Description, attr.Overview) {
					def = formatRich(attr.Description, attr.Overview, attr.Tips, attr.Levels, nil, attr.Tags)
					key = attr.Name + " (" + key + ")"
					found = true
				}
				break
			}
		}
	}
	// 2. Legacy Local Lookup (Markdown Fallback)
	// If JSON matched but had empty prose, resolvedID holds the enum ID so
	// we can look up the corresponding definitions/*.md directly instead
	// of falling back to fuzzy name matching.
	if !found {
		var legacyDef string
		var legacyFound bool
		if resolvedID != "" {
			legacyDef, legacyFound = GetDefinition(resolvedID)
		}
		if !legacyFound {
			legacyDef, legacyFound = GetDefinition(key)
		}
		if !legacyFound {
			DefinitionsLock.RLock()
			for k, v := range Definitions {
				if strings.HasSuffix(k, "/"+key) || k == key || strings.HasSuffix(k, "/"+strings.ToLower(key)) {
					legacyDef = v
					key = k
					legacyFound = true
					break
				}
			}
			DefinitionsLock.RUnlock()
		}
		if legacyFound {
			def = legacyDef
			found = true
		}
	}

	// Construct markdown for RichText
	md := ""
	if !found {
		ip.title.SetText(key)
		md = "_No local documentation available._"
		resolvedCategory = "REFERENCE"
	} else {
		ip.title.SetText(key)
		if context != "" {
			md = "**" + context + "**\n\n" + def
		} else {
			md = def
		}
	}
	ip.updateHeaderChips(resolvedID, resolvedCategory)

	ip.content.ParseMarkdown(md)

	// Dev-only: async query the local Holocron server for extra context.
	// Client is nil for regular users (see holocron_client.go); this
	// block is dead code for non-maintainer builds.
	if ip.holocronClient != nil && ip.holocronClient.Available {
		go func(query string) {
			ip.content.ParseMarkdown(md + "\n\n_[querying dev backend...]_")

			summary, err := ip.holocronClient.Ask(query)
			if err == nil && summary != "" {
				newContent := md + "\n\n**DEV INSIGHT**\n" + summary
				ip.content.ParseMarkdown(newContent)
			} else {
				ip.content.ParseMarkdown(md) // Revert
			}
		}(key)
	}
}
