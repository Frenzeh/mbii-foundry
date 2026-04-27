package main

// Icon Inventory window — a definitive "what icons does Foundry
// actually have?" debug surface. Renders every PNG embedded in
// `assets/icons/{weapons,attributes,classes,force}` in a scrollable
// grid, alongside every embedded `assets/boxicons/*.svg` glyph.
//
// Why: when the user complains "icons aren't showing", we don't
// always know whether the renderer is broken, the alias mapping is
// wrong, or the file simply isn't where we think it is. This window
// shows the ground truth — if a tile renders here, the asset is
// present and the canvas pipeline works; if every UI surface still
// shows blank, the alias resolution is at fault, not the rendering.
//
// Each tile prints the basename underneath so the user can match
// against `attributeIconAliases` / `weaponIconAliases` / boxicon
// keyword needles to figure out why a specific row isn't picking up
// its icon.

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// showIconInventory pops a window cataloguing every icon Foundry
// has embedded. Useful for verifying the asset pipeline end-to-end:
// if the tiles here render but the editor's attribute / weapon /
// class rows don't, the issue is alias resolution, not rendering.
func (a *App) showIconInventory() {
	w := a.fyneApp.NewWindow("Icon Inventory")
	w.Resize(fyne.NewSize(900, 700))

	categories := []struct {
		title string
		dir   string
	}{
		{"Weapons (gfx/hud/w_icon_*)", "weapons"},
		{"Attributes (gfx/menus/alpha/* + i_icon_*)", "attributes"},
		{"Force Powers (gfx/mp/*)", "force"},
		{"Classes (gfx/mp/c_icon_*)", "classes"},
	}

	tabs := container.NewAppTabs()

	totalEmbedded := 0
	for _, cat := range categories {
		entries, err := embedIcons.ReadDir("assets/icons/" + cat.dir)
		if err != nil {
			tabs.Append(container.NewTabItem(cat.title,
				widget.NewLabel(fmt.Sprintf("error reading dir: %v", err))))
			continue
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".png") {
				names = append(names, strings.TrimSuffix(e.Name(), ".png"))
			}
		}
		sort.Strings(names)
		totalEmbedded += len(names)

		grid := container.NewGridWrap(fyne.NewSize(150, 130))
		for _, basename := range names {
			grid.Add(buildIconInventoryTile(basename, cat.dir))
		}
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("%s — %d files", cat.title, len(names)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		body := container.NewBorder(header, nil, nil, nil,
			container.NewVScroll(grid))
		tabs.Append(container.NewTabItem(fmt.Sprintf("%s (%d)", cat.dir, len(names)), body))
	}

	// Boxicons tab — the SVG fallback set used when no MBII HUD
	// art exists for an attribute. Different render path (SVG via
	// fyne.NewStaticResource, no LoadGameIcon involved) so showing
	// these confirms the boxicon fallback works independently.
	{
		entries, _ := embedBoxicons.ReadDir("assets/boxicons")
		var names []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".svg") {
				names = append(names, strings.TrimSuffix(e.Name(), ".svg"))
			}
		}
		sort.Strings(names)
		grid := container.NewGridWrap(fyne.NewSize(150, 130))
		for _, basename := range names {
			grid.Add(buildBoxiconInventoryTile(basename))
		}
		header := widget.NewLabelWithStyle(
			fmt.Sprintf("Boxicons — %d files", len(names)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		body := container.NewBorder(header, nil, nil, nil,
			container.NewVScroll(grid))
		tabs.Append(container.NewTabItem(fmt.Sprintf("boxicons (%d)", len(names)), body))
	}

	intro := widget.NewLabelWithStyle(
		fmt.Sprintf("Foundry has %d embedded HUD icons + boxicon fallbacks. "+
			"If a tile renders here, the asset pipeline works. "+
			"If a row in the editor stays blank, it's the alias mapping that's wrong, not rendering.",
			totalEmbedded),
		fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	intro.Wrapping = fyne.TextWrapWord

	w.SetContent(container.NewBorder(
		container.NewPadded(intro), nil, nil, nil,
		tabs,
	))
	w.Show()
}

// buildIconInventoryTile renders one embedded HUD icon as a tile.
//
// Render strategy (this iteration): bypass fyne.StaticResource +
// canvas.NewImageFromResource entirely. We've confirmed LoadGameIcon
// returns a valid image.Image, but routing it through PNG bytes →
// StaticResource → canvas.NewImageFromResource produces an invisible
// rendering. Instead, hand the already-decoded image.Image directly
// to canvas.NewImageFromImage — that path skips Fyne's internal
// resource-format detection.
//
// Status codes (corner) tell us at a glance which path each tile
// took. Width/height of the decoded image are printed too, so
// "rendered to 0×0" is visible.
//
//   ✓ wxh — image.Image decoded, sized w×h
//   M     — embedIcons.ReadFile failed (PNG isn't in the binary)
//   E0    — file embedded but zero bytes
//   D     — decode failed (corrupt PNG)
func buildIconInventoryTile(basename, dir string) fyne.CanvasObject {
	path := "assets/icons/" + dir + "/" + basename + ".png"
	var (
		img    fyne.CanvasObject
		status string
		bg     color.Color
	)

	data, err := embedIcons.ReadFile(path)
	switch {
	case err != nil:
		status = "M"
		bg = color.NRGBA{R: 80, G: 30, B: 80, A: 200}
		img = container.NewGridWrap(fyne.NewSize(64, 64),
			widget.NewIcon(theme.QuestionIcon()))
	case len(data) == 0:
		status = "E0"
		bg = color.NRGBA{R: 80, G: 30, B: 30, A: 200}
		img = container.NewGridWrap(fyne.NewSize(64, 64),
			widget.NewIcon(theme.QuestionIcon()))
	default:
		status = "✓"
		res := fyne.NewStaticResource(basename+".png", data)
		ci := canvas.NewImageFromResource(res)
		ci.FillMode = canvas.ImageFillContain
		ci.ScaleMode = canvas.ImageScaleSmooth
		ci.SetMinSize(fyne.NewSize(64, 64))
		img = ci
	}

	label := canvas.NewText(basename, theme.ForegroundColor())
	label.TextSize = 10
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Monospace: true}

	statusLbl := canvas.NewText(status, theme.PlaceHolderColor())
	statusLbl.TextSize = 9
	statusLbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	footer := container.NewBorder(nil, nil, statusLbl, nil, label)
	tile := container.NewBorder(
		nil,                      // top
		footer,                   // bottom
		nil, nil,                 // left/right
		container.NewCenter(img), // center
	)
	if bg != nil {
		bgRect := canvas.NewRectangle(bg)
		return container.NewStack(bgRect, tile)
	}
	return tile
}

// buildBoxiconInventoryTile renders one boxicon SVG as a tile so we
// can visually confirm the SVG fallback set is loading.
func buildBoxiconInventoryTile(basename string) fyne.CanvasObject {
	var img fyne.CanvasObject = widget.NewIcon(theme.QuestionIcon())
	if res := loadBoxiconResource(basename); res != nil {
		img = NewRasterIconFromResource(res, 48, 48)
	}
	label := canvas.NewText(basename, theme.ForegroundColor())
	label.TextSize = 10
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Monospace: true}

	return container.NewVBox(
		container.NewCenter(img),
		label,
	)
}
