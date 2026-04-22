package main

// Favorites = a persistent list of pinned folder paths. Works like the
// "pinned" sidebar entries in Windows Explorer / macOS Finder — users
// mark paths they revisit (GameData, TextAssets, their modpack dir,
// etc.) and the app surfaces them as one-click shortcuts in every
// path-entry widget, avoiding another trip through Fyne's folder
// picker.

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const maxFavoritePaths = 12

// PinFavorite adds a path to the favorites list (prepended, so most-
// recently-pinned is first). No-ops on empty paths; dedupes; trims
// trailing separators; caps the list at maxFavoritePaths.
func (a *App) PinFavorite(path string) {
	path = normalizeFavoritePath(path)
	if path == "" {
		return
	}
	// Remove any existing entry so the re-pin moves it to the front.
	out := make([]string, 0, len(a.config.FavoritePaths)+1)
	out = append(out, path)
	for _, p := range a.config.FavoritePaths {
		if normalizeFavoritePath(p) == path {
			continue
		}
		out = append(out, p)
		if len(out) >= maxFavoritePaths {
			break
		}
	}
	a.config.FavoritePaths = out
	a.saveConfig()
}

// UnpinFavorite removes a path from the favorites list.
func (a *App) UnpinFavorite(path string) {
	path = normalizeFavoritePath(path)
	if path == "" {
		return
	}
	out := a.config.FavoritePaths[:0]
	for _, p := range a.config.FavoritePaths {
		if normalizeFavoritePath(p) != path {
			out = append(out, p)
		}
	}
	a.config.FavoritePaths = out
	a.saveConfig()
}

// IsFavorite reports whether a path is currently pinned.
func (a *App) IsFavorite(path string) bool {
	path = normalizeFavoritePath(path)
	if path == "" {
		return false
	}
	for _, p := range a.config.FavoritePaths {
		if normalizeFavoritePath(p) == path {
			return true
		}
	}
	return false
}

func normalizeFavoritePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

// NewPathEntryWithFavorites builds a path-entry widget bundled with a
// pin toggle, a Favorites dropdown, and a Browse button. Drop-in
// replacement for bare `widget.NewEntry()` anywhere the app asks the
// user for a folder path.
//
// The `onBrowse` callback is invoked when the Browse button is
// pressed; the caller wires in the actual picker (typically
// dialog.NewFolderOpen with a sensible starting location).
func (a *App) NewPathEntryWithFavorites(entry *widget.Entry, onBrowse func()) fyne.CanvasObject {
	// Pin/unpin toggle — the icon flips between ★ and ☆ based on
	// whether the entry's current text is already pinned.
	pinBtn := widget.NewButtonWithIcon("", theme.ContentAddIcon(), nil)
	pinBtn.Importance = widget.LowImportance
	var refreshPin func()
	refreshPin = func() {
		if a.IsFavorite(entry.Text) {
			pinBtn.SetIcon(theme.ConfirmIcon())
			pinBtn.OnTapped = func() {
				a.UnpinFavorite(entry.Text)
				refreshPin()
			}
		} else {
			pinBtn.SetIcon(theme.ContentAddIcon())
			pinBtn.OnTapped = func() {
				a.PinFavorite(entry.Text)
				refreshPin()
			}
		}
	}
	refreshPin()

	// Wrap whatever the entry's existing OnChanged is so pin-state
	// updates follow every keystroke.
	prevOnChanged := entry.OnChanged
	entry.OnChanged = func(s string) {
		if prevOnChanged != nil {
			prevOnChanged(s)
		}
		refreshPin()
	}

	// Favorites dropdown — shows pinned paths, selecting one fills
	// the entry. Rebuilt each time the button is pressed so it stays
	// in sync with config.
	favoritesBtn := widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), nil)
	favoritesBtn.Importance = widget.LowImportance
	favoritesBtn.OnTapped = func() {
		if len(a.config.FavoritePaths) == 0 {
			// Offer a helpful hint instead of an empty menu.
			prompt := widget.NewLabel("No favorites yet. Pin a path with the ★ button to quick-select it later.")
			prompt.Wrapping = fyne.TextWrapWord
			popUp := widget.NewPopUp(prompt, a.mainWindow.Canvas())
			popUp.Resize(fyne.NewSize(320, 60))
			popUp.ShowAtPosition(favoritesBtn.Position().AddXY(0, favoritesBtn.Size().Height))
			return
		}
		items := make([]*fyne.MenuItem, 0, len(a.config.FavoritePaths))
		for _, p := range a.config.FavoritePaths {
			path := p // capture
			items = append(items, fyne.NewMenuItem(path, func() {
				entry.SetText(path)
			}))
		}
		menu := fyne.NewMenu("Favorites", items...)
		popUp := widget.NewPopUpMenu(menu, a.mainWindow.Canvas())
		popUp.ShowAtPosition(favoritesBtn.Position().AddXY(0, favoritesBtn.Size().Height))
	}

	browseBtn := NewTooltipButton("", theme.FolderOpenIcon(), onBrowse, "Browse for folder")
	browseBtn.Importance = widget.LowImportance

	buttons := container.NewHBox(favoritesBtn, pinBtn, browseBtn)
	return container.NewBorder(nil, nil, nil, buttons, entry)
}
