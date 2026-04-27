package main

import (
	"bytes"
	"image/png"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type WelcomeScreen struct {
	app *App
}

func NewWelcomeScreen(app *App) *WelcomeScreen {
	return &WelcomeScreen{app: app}
}

// Welcome screen — brutalist-leaning typography paired with a more
// content-dense layout. Earlier iterations bled the title to the edge
// of an otherwise empty canvas; without content underneath, the bleed
// read as unfinished. The screen now flows:
//
//	MBII FOUNDRY (display)
//	MOVIE BATTLES II CONTENT EDITOR (caption)
//	[accent rule]
//	CREATE column       RECENT column
//	  [tile cards]        [file buttons OR onboarding tips]
//	[accent rule]
//	GET STARTED strip (keyboard shortcuts + pointers)
//	Version footer
//
// Padding is modest on all sides so the type still pushes toward the
// edges, but every horizontal band has something to look at.
func (w *WelcomeScreen) GetContent() fyne.CanvasObject {
	// Hero: MBII logo on the left, "FOUNDRY" word to its right. Logo
	// communicates the "MBII" half of the brand visually so the type
	// treatment no longer needs to carry it. Source PNG is 305×189.
	//
	// Decode the PNG upfront into an image.Image — NewImageFromReader
	// and NewImageFromResource were both claiming layout space but
	// rendering blank on Fyne v2.7.1 / macOS. NewImageFromImage takes
	// an already-decoded image and gives us a reliably drawable asset.
	logoHeight := SizeDisplay * 1.8 // ~97px tall — roomy enough to read clearly
	logoWidth := logoHeight * (305.0 / 189.0)
	// Decode via png.Decode directly, NOT image.Decode. The generic
	// image.Decode walks registered formats; one of our deps
	// (github.com/ftrvxmtrx/tga) registers TGA with an EMPTY magic
	// string which matches any input, so image.Decode was picking
	// TGA first, parsing PNG bytes as TGA, and failing with
	// "tga: invalid format". Direct png.Decode bypasses that.
	var logo fyne.CanvasObject = layout.NewSpacer()
	if decoded, err := png.Decode(bytes.NewReader(embedLogoMBII)); err == nil {
		img := canvas.NewImageFromImage(decoded)
		img.FillMode = canvas.ImageFillContain
		img.ScaleMode = canvas.ImageScaleSmooth
		img.SetMinSize(fyne.NewSize(logoWidth, logoHeight))
		// GridWrap forces an exact cell size so HBox allocates and
		// the canvas.Image gets a concrete Resize call — MinSize alone
		// was honored for layout math but not always for the draw pass.
		logo = container.New(layout.NewGridWrapLayout(fyne.NewSize(logoWidth, logoHeight)), img)
	} else {
		LogError("Welcome logo PNG decode failed: %v (bytes=%d)", err, len(embedLogoMBII))
	}

	title := canvas.NewText("FOUNDRY", theme.ForegroundColor())
	title.Alignment = fyne.TextAlignLeading
	title.TextSize = SizeDisplay
	title.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := canvas.NewText("MOVIE BATTLES II CONTENT EDITOR", theme.PlaceHolderColor())
	subtitle.Alignment = fyne.TextAlignLeading
	subtitle.TextSize = SizeSmall
	subtitle.TextStyle = fyne.TextStyle{Bold: true}

	// Type stack to the right of the logo: FOUNDRY on top, subtitle
	// directly beneath it so the subtitle indents to match the "F" of
	// FOUNDRY rather than hanging off the left margin under the logo.
	titleStack := container.NewVBox(title, subtitle)

	// Tight horizontal gap between logo and type (SpaceXS = 4px) so the
	// two read as one composite hero mark.
	heroRow := container.NewHBox(logo, Gap(SpaceXS), titleStack)

	// Accent rule factory. Uses AccentRule widget so each rule
	// repaints when the theme changes, rather than keeping whatever
	// color the welcome screen was built with.
	rule := func() fyne.CanvasObject { return NewAccentRule() }

	createLabel := sectionCaption("CREATE")
	recentLabel := sectionCaption("RECENT")

	// Action cards — stacked vertically inside the CREATE column so
	// each card gets full column width and the user doesn't have to
	// scan a 2×2 grid. Stacking also solves the minSizeWrapper bug
	// that made buttons invisible inside GridWithColumns.
	newChar := NewTileCard("New Character", ".mbch", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Character", NewMBCHEditor(w.app))
	})
	newSaber := NewTileCard("New Saber", ".sab", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Saber", NewSABEditor(w.app))
	})
	newVeh := NewTileCard("New Vehicle", ".veh", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Vehicle", NewVEHEditor(w.app))
	})
	newSiege := NewTileCard("New Siege", ".siege", theme.ContentAddIcon(), func() {
		w.app.createNewFile("Siege", NewSiegeEditor(w.app))
	})
	openCard := NewTileCard("Open Existing…", "mbch · sab · veh · siege", theme.FolderOpenIcon(), func() {
		w.app.openFile()
	})

	createCol := container.NewVBox(
		createLabel,
		Gap(SpaceXS),
		newChar,
		Gap(SpaceXS),
		newSaber,
		Gap(SpaceXS),
		newVeh,
		Gap(SpaceXS),
		newSiege,
		Gap(SpaceSM),
		openCard,
	)

	recentCol := container.NewVBox(recentLabel, Gap(SpaceXS), w.buildRecentColumn())

	// Two-column body. Equal widths; the sparse column gets onboarding
	// content so neither side feels under-used.
	body := container.New(layout.NewGridLayoutWithColumns(2), createCol, recentCol)

	// Footer strip — always pinned to the bottom of the welcome area.
	// Pairs with the top rule to bracket the content, and gives the
	// bottom of the screen a visual baseline instead of trailing off
	// into empty dark space on taller windows. The update callout
	// (when a newer release exists) lives inside the footer's right
	// column so it sits snug next to the version string rather than
	// as a separate banner up top.
	footer := w.buildFooter()
	footerBlock := container.NewVBox(rule(), Gap(SpaceSM), footer)

	top := container.NewVBox(
		Gap(SpaceSM),
		heroRow,
		Gap(SpaceMD),
		rule(),
		Gap(SpaceLG),
		body,
	)

	// Border lays top at its MinSize at the top, footer at its MinSize
	// at the bottom, leaving the middle as a breathing gap that grows
	// with the window — so the GET STARTED strip stays bottom-aligned.
	return container.NewPadded(container.NewBorder(top, footerBlock, nil, nil, layout.NewSpacer()))
}

// sectionCaption renders the small-caps-style section header used by
// each column. SizeSmall keeps it in Jost (≥11pt); Bold picks up the
// heavier weight.
func sectionCaption(text string) *canvas.Text {
	t := canvas.NewText(text, theme.PlaceHolderColor())
	t.TextSize = SizeSmall
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// buildRecentColumn returns either the list of recent file buttons or
// an onboarding block if there are no recents. The onboarding block
// mirrors the CREATE column's visual weight so an empty Recents still
// looks intentional.
func (w *WelcomeScreen) buildRecentColumn() fyne.CanvasObject {
	recentFiles := w.app.fileManager.GetRecentFiles()
	if len(recentFiles) > 0 {
		items := container.NewVBox()
		for _, rf := range recentFiles {
			path := rf.Path
			btn := widget.NewButtonWithIcon(filepath.Base(path), theme.FileIcon(), func() {
				// openFileFromPath defers the heavy work to the next
				// UI tick internally — the click handler returns
				// immediately so macOS doesn't show a beach ball.
				w.app.openFileFromPath(path)
			})
			btn.Importance = widget.LowImportance
			btn.Alignment = widget.ButtonAlignLeading
			items.Add(btn)
		}
		return items
	}

	emptyHeadline := canvas.NewText("NO RECENT FILES", theme.ForegroundColor())
	emptyHeadline.TextSize = SizeSubtitle
	emptyHeadline.TextStyle = fyne.TextStyle{Bold: true}

	emptyHint := canvas.NewText("Pick a card on the left or open a file.", theme.PlaceHolderColor())
	emptyHint.TextSize = SizeSmall

	tipHeader := canvas.NewText("WHILE YOU'RE HERE", theme.PlaceHolderColor())
	tipHeader.TextSize = SizeSmall
	tipHeader.TextStyle = fyne.TextStyle{Bold: true}

	tip := func(body string) *canvas.Text {
		t := canvas.NewText(body, theme.ForegroundColor())
		t.TextSize = SizeSmall
		t.TextStyle = fyne.TextStyle{Monospace: true}
		return t
	}

	return container.NewVBox(
		emptyHeadline,
		Gap(SpaceXS),
		emptyHint,
		Gap(SpaceLG),
		tipHeader,
		Gap(SpaceXS),
		tip("FILES     browse the assets on your disk"),
		tip("LIBRARY   look up every enum and flag"),
		tip("MODPACKS  package a folder into a .pk3"),
		Gap(SpaceSM),
		tip("Double-click any asset to open it."),
		tip("Drag the panel dividers to resize."),
	)
}

// buildFooter renders the bottom strip of the welcome screen — a
// shortcut list on the left, a brand/version block on the right.
// When an update is available, a compact callout slots in above the
// version string so users see the "new version available" hint in
// the same place they look for the current version — no separate
// banner up top fighting the hero for attention.
func (w *WelcomeScreen) buildFooter() fyne.CanvasObject {
	shortcutHeader := canvas.NewText("GET STARTED", theme.PlaceHolderColor())
	shortcutHeader.TextSize = SizeSmall
	shortcutHeader.TextStyle = fyne.TextStyle{Bold: true}

	mono := func(body string) *canvas.Text {
		t := canvas.NewText(body, theme.ForegroundColor())
		t.TextSize = SizeSmall
		t.TextStyle = fyne.TextStyle{Monospace: true}
		return t
	}

	shortcuts := container.NewVBox(
		shortcutHeader,
		Gap(SpaceXS),
		mono("CMD+N   create a new file"),
		mono("CMD+O   open an existing file"),
		mono("CMD+S   save the current file"),
		mono("?       help / show keyboard shortcuts"),
	)

	return container.New(layout.NewGridLayoutWithColumns(2), shortcuts, w.buildBrandColumn())
}

// buildBrandColumn renders the footer's right column: branding line,
// version (pulled live from AppVersion so it never drifts from the
// build), and — when applicable — the update callout. Stacked right-
// aligned so everything flows to the edge and reads as a cohesive
// mark.
func (w *WelcomeScreen) buildBrandColumn() fyne.CanvasObject {
	brandLine := canvas.NewText("MBII FOUNDRY", theme.PlaceHolderColor())
	brandLine.TextSize = SizeSmall
	brandLine.TextStyle = fyne.TextStyle{Bold: true}
	brandLine.Alignment = fyne.TextAlignTrailing

	versionTxt := canvas.NewText("v"+AppVersion, theme.ForegroundColor())
	versionTxt.TextSize = SizeSubtitle
	versionTxt.TextStyle = fyne.TextStyle{Bold: true}
	versionTxt.Alignment = fyne.TextAlignTrailing

	children := []fyne.CanvasObject{
		layout.NewSpacer(), // push the brand block to the bottom edge
		brandLine,
		versionTxt,
	}

	// Compact update callout, only rendered when UpdateChecker has a
	// newer release cached. Hidden by default so the footer stays a
	// clean brand mark for people who are already up to date.
	if w.app != nil && w.app.updateChecker != nil {
		if callout := NewUpdateCallout(w.app, w.app.updateChecker.Latest()); callout != nil {
			// Rule + spacer above the callout to separate it from the
			// version, keeping both readable instead of blending into
			// one amorphous bottom blob.
			children = append(children,
				Gap(SpaceXS),
				callout.GetContent(),
			)
		}
	}

	return container.NewVBox(children...)
}
