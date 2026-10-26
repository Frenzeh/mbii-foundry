package main

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

// Leave room for the native close button, padding and overflow control even
// in the minimum-width document pane. Measure bold: selected tabs use it.
const documentTabCaptionWidth float32 = 195
const mainWindowTitle = AppName + " - MBII Content Editor"

func documentTabCaption(title string) string {
	fits := func(text string) bool {
		return fyne.MeasureText(text, theme.TextSize(), fyne.TextStyle{Bold: true}).Width <= documentTabCaptionWidth
	}
	if title == "Home" || fits(title) {
		return title
	}
	marker := ""
	name := title
	if strings.HasPrefix(name, "* ") {
		marker, name = "* ", name[2:]
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	// An exceptionally long extension cannot be retained in full within the
	// bound. In that case elide the whole name, still retaining its tail.
	if !fits(marker + "…" + ext) {
		stem, ext = name, ""
	}
	runes := []rune(stem)
	caption := func(keep int) string {
		left := (keep + 1) / 2
		right := keep / 2
		return marker + string(runes[:left]) + "…" + string(runes[len(runes)-right:]) + ext
	}
	best := marker + "…" + ext
	low, high := 1, len(runes)-1
	for low <= high {
		keep := low + (high-low)/2
		candidate := caption(keep)
		if fits(candidate) {
			best = candidate
			low = keep + 1
		} else {
			high = keep - 1
		}
	}
	return best
}

func (a *App) fullTabTitle(tab *container.TabItem) string {
	if tab == nil {
		return ""
	}
	if title, ok := a.tabTitles[tab]; ok {
		return title
	}
	return tab.Text // Home and caller-created tabs have no document metadata.
}

func (a *App) setTabTitle(tab *container.TabItem, title string) {
	if tab == nil {
		return
	}
	if previous, ok := a.tabTitles[tab]; ok && previous == title {
		return
	}
	if a.tabTitles == nil {
		a.tabTitles = make(map[*container.TabItem]string)
	}
	a.tabTitles[tab] = title
	tab.Text = documentTabCaption(title)
	if a.docTabs != nil {
		a.docTabs.Refresh()
		if a.docTabs.Selected() == tab {
			a.updateSelectedWindowTitle()
		}
	}
}

func documentTitleWithDirty(title string, dirty bool) string {
	if strings.HasPrefix(title, "* ") == dirty {
		return title
	}
	if dirty {
		return "* " + title
	}
	return strings.TrimPrefix(title, "* ")
}

func (a *App) updateSelectedWindowTitle() {
	if a.mainWindow == nil || a.docTabs == nil {
		return
	}
	tab := a.docTabs.Selected()
	if tab == nil || a.editors[tab] == nil {
		if a.mainWindow.Title() != mainWindowTitle {
			a.mainWindow.SetTitle(mainWindowTitle)
		}
		return
	}
	title := a.fullTabTitle(tab)
	const suffix = " — " + AppName
	current, ok := strings.CutSuffix(a.mainWindow.Title(), suffix)
	if !ok || current != title {
		a.mainWindow.SetTitle(title + suffix)
	}
}
