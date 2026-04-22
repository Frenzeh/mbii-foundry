package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type InfoPanel struct {
	container *fyne.Container
	title     *widget.Label
	content   *widget.RichText // Changed to RichText for formatting
	search    *widget.Entry
	list      *widget.List
	tabs      *container.AppTabs

	keys []string

	holocronClient *HolocronClient
}

func NewInfoPanel() *InfoPanel {
	ip := &InfoPanel{}
	ip.createUI()
	return ip
}

func (ip *InfoPanel) SetHolocronClient(client *HolocronClient) {
	ip.holocronClient = client
}

func (ip *InfoPanel) createUI() {
	ip.title = widget.NewLabelWithStyle("Information", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	welcomeMsg := `
### Info Panel

This panel provides real-time documentation and context for the field you're editing.

**How to use:**
*   **Hover** over any attribute or weapon in the editor to see its description here.
*   **Switch** to the "Reference Library" tab to browse all available topics.
`
	ip.content = widget.NewRichTextFromMarkdown(welcomeMsg)

	ip.search = NewInputEntry()
	ip.search.SetPlaceHolder("Search help...")
	ip.search.OnChanged = ip.filterList

	ip.list = widget.NewList(
		func() int { return len(ip.keys) },
		func() fyne.CanvasObject { return widget.NewLabel("Topic") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(ip.keys[id])
		},
	)
	ip.list.OnSelected = func(id widget.ListItemID) {
		ip.ShowInfo(ip.keys[id], "")
		// Auto-switch to Context tab to show result
		ip.tabs.SelectIndex(0)
	}

	ip.refreshKeys("")

	// "Active Context" was an unhelpful header that just said "here's the
	// thing you're hovering" — redundant when the title below already shows
	// the item name. Dropped; the title + content speak for themselves.
	details := container.NewVBox(ip.title, widget.NewSeparator(), ip.content)
	detailsScroll := container.NewVScroll(details)

	listHeader := widget.NewLabelWithStyle("Reference Library", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	listContainer := container.NewBorder(container.NewVBox(listHeader, ip.search), nil, nil, nil, ip.list)

	ip.tabs = container.NewAppTabs(
		container.NewTabItem("Context", detailsScroll),
		container.NewTabItem("Library", listContainer),
	)

	ip.container = container.NewMax(ip.tabs)
}

func (ip *InfoPanel) refreshKeys(filter string) {
	DefinitionsLock.RLock()
	defer DefinitionsLock.RUnlock()

	ip.keys = []string{}
	filter = strings.ToLower(filter)

	for k := range Definitions {
		if filter == "" || strings.Contains(strings.ToLower(k), filter) {
			ip.keys = append(ip.keys, k)
		}
	}

	// Add JSON keys too if searchable
	for _, a := range GetAttributes() {
		k := a.Name
		if filter == "" || strings.Contains(strings.ToLower(k), filter) {
			ip.keys = append(ip.keys, k)
		}
	}
	for _, w := range GetWeapons() {
		k := w.Name
		if filter == "" || strings.Contains(strings.ToLower(k), filter) {
			ip.keys = append(ip.keys, k)
		}
	}

	// Add other definitions
	for _, c := range GetClasses() {
		if filter == "" || strings.Contains(strings.ToLower(c.Name), filter) {
			ip.keys = append(ip.keys, c.Name)
		}
	}
	for _, f := range GetClassFlags() {
		if filter == "" || strings.Contains(strings.ToLower(f.Name), filter) {
			ip.keys = append(ip.keys, f.Name)
		}
	}
	for _, s := range GetSaberStyles() {
		if filter == "" || strings.Contains(strings.ToLower(s.Name), filter) {
			ip.keys = append(ip.keys, s.Name)
		}
	}
	for _, g := range GetGlossary() {
		if filter == "" || strings.Contains(strings.ToLower(g.Name), filter) {
			ip.keys = append(ip.keys, g.Name)
		}
	}

	// Deduplicate and sort
	sort.Strings(ip.keys)
	ip.list.Refresh()
}

func (ip *InfoPanel) filterList(text string) {
	ip.refreshKeys(text)
}

func (ip *InfoPanel) GetContent() fyne.CanvasObject {
	return ip.container
}

func (ip *InfoPanel) ShowInfo(key, context string) {
	LogInfo("InfoPanel: ShowInfo called for key='%s'", key)

	// Auto-switch to Context tab so user sees the info
	ip.tabs.SelectIndex(0)

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
	var resolvedID string
	for _, attr := range GetAttributes() {
		if attr.ID == key || attr.Name == key {
			resolvedID = attr.ID
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
	} else {
		ip.title.SetText(key)
		if context != "" {
			md = "**" + context + "**\n\n" + def
		} else {
			md = def
		}
	}

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
