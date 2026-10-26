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

// Home keeps its actions in one scrollable page and stacks columns when the
// editor pane is narrow, without changing the user's saved pane widths.
func (w *WelcomeScreen) GetContent() fyne.CanvasObject {
	// Hero: MBII logo on the left, "FOUNDRY" word to its right. Logo
	// communicates the "MBII" half of the brand visually so the type
	// treatment no longer needs to carry it. Source PNG is 305×189.
	//
	// Decode the PNG upfront into an image.Image — NewImageFromReader
	// and NewImageFromResource were both claiming layout space but
	// rendering blank on Fyne v2.7.1 / macOS. NewImageFromImage takes
	// an already-decoded image and gives us a reliably drawable asset.
	logoHeight := SizeTitle * 1.1
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
		// GridWrap forces an exact cell size so HBox allocates and
		// the canvas.Image gets a concrete Resize call — MinSize alone
		// was honored for layout math but not always for the draw pass.
		logo = container.New(layout.NewGridWrapLayout(fyne.NewSize(logoWidth, logoHeight)), img)
	} else {
		LogError("Welcome logo PNG decode failed: %v (bytes=%d)", err, len(embedLogoMBII))
	}

	title := welcomeHeading("FOUNDRY", SizeTitle, theme.ColorNameForeground, fyne.TextAlignLeading)

	subtitle := welcomeHint("MOVIE BATTLES II CONTENT EDITOR")
	subtitle.TextStyle = fyne.TextStyle{Bold: true}
	subtitle.SizeName = theme.SizeNameCaptionText

	// Type stack to the right of the logo: FOUNDRY on top, subtitle
	// directly beneath it so the subtitle indents to match the "F" of
	// FOUNDRY rather than hanging off the left margin under the logo.
	titleStack := container.NewVBox(title, subtitle)

	heroRow := container.NewBorder(nil, nil, logo, nil, titleStack)

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
		newSaber,
		newVeh,
		newSiege,
		Gap(SpaceXS),
		openCard,
	)

	recentCol := container.NewVBox(recentLabel, Gap(SpaceXS), w.buildRecentColumn())

	body := container.New(&welcomeColumnsLayout{}, createCol, recentCol)
	footerBlock := container.NewVBox(rule(), Gap(SpaceSM), w.buildFooter())
	page := container.NewVBox(
		heroRow,
		Gap(SpaceSM),
		rule(),
		Gap(SpaceSM),
		body,
		Gap(SpaceLG),
		footerBlock,
	)
	padded := container.New(layout.NewCustomPaddedLayout(SpaceLG, SpaceLG, SpaceLG, SpaceLG), page)
	return container.NewVScroll(padded)
}

// sectionCaption gives primary sections enough weight to anchor each column
// without competing with the page title.
func sectionCaption(text string) fyne.CanvasObject {
	return welcomeHeading(text, SizeBody, theme.ColorNameForeground, fyne.TextAlignLeading)
}

// buildRecentColumn returns recent-file links or a short onboarding block.
func (w *WelcomeScreen) buildRecentColumn() fyne.CanvasObject {
	recentFiles := w.app.fileManager.GetRecentFiles()
	if len(recentFiles) > 0 {
		items := container.NewVBox()
		for _, rf := range recentFiles {
			path := rf.Path
			link := widget.NewHyperlink(filepath.Base(path), nil)
			link.Wrapping = fyne.TextWrapWord
			link.OnTapped = func() {
				// openFileFromPath defers the heavy work to the next
				// UI tick internally — the click handler returns
				// immediately so macOS doesn't show a beach ball.
				w.app.openFileFromPath(path)
			}
			items.Add(link)
		}
		return items
	}

	emptyHeadline := welcomeHeading("NO RECENT FILES", SizeSubtitle, theme.ColorNameForeground, fyne.TextAlignLeading)

	emptyHint := welcomeHint("Create something new or open an existing file.")

	tipHeader := sectionCaption("WHILE YOU'RE HERE")

	return container.NewVBox(
		emptyHeadline,
		Gap(SpaceXS),
		emptyHint,
		Gap(SpaceLG),
		tipHeader,
		Gap(SpaceXS),
		welcomeHint("Files — browse the assets on your disk."),
		welcomeHint("Library — look up every enum and flag."),
		welcomeHint("Modpacks — package a folder into a .pk3."),
		Gap(SpaceSM),
		welcomeHint("Double-click any asset to open it."),
		welcomeHint("Drag the panel dividers to resize."),
	)
}

// buildFooter keeps shortcuts and version information readable at any pane width.
func (w *WelcomeScreen) buildFooter() fyne.CanvasObject {
	shortcutHeader := sectionCaption("GET STARTED")

	shortcuts := container.NewVBox(
		shortcutHeader,
		Gap(SpaceXS),
		welcomeHint("CMD+N — create a new file"),
		welcomeHint("CMD+O — open an existing file"),
		welcomeHint("CMD+S — save the current file"),
		welcomeHint("? — help / show keyboard shortcuts"),
	)

	return container.New(&welcomeColumnsLayout{}, shortcuts, w.buildBrandColumn())
}

// buildBrandColumn renders the footer's right column: branding line,
// version (pulled live from AppVersion so it never drifts from the
// build), and — when applicable — the update callout. Stacked right-
// aligned so everything flows to the edge and reads as a cohesive
// mark.
func (w *WelcomeScreen) buildBrandColumn() fyne.CanvasObject {
	brandLine := welcomeHeading("MBII FOUNDRY", SizeSmall, theme.ColorNamePlaceHolder, fyne.TextAlignTrailing)
	versionTxt := welcomeHeading("v"+AppVersion, SizeSubtitle, theme.ColorNameForeground, fyne.TextAlignTrailing)

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

// welcomeHeading keeps the existing canvas-text sizes while allowing Home's
// headings and branding to wrap with the pane. Only text size is overridden;
// fonts, colors, and density continue to come from the application theme.
func welcomeHeading(text string, size float32, colorName fyne.ThemeColorName, alignment fyne.TextAlign) fyne.CanvasObject {
	content := widget.NewRichText(&widget.TextSegment{
		Text: text,
		Style: widget.RichTextStyle{
			Alignment: alignment,
			ColorName: colorName,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	content.Wrapping = fyne.TextWrapWord
	return container.NewThemeOverride(content, welcomeHeadingTheme{
		Theme:    fyne.CurrentApp().Settings().Theme(),
		textSize: size,
	})
}

type welcomeHeadingTheme struct {
	fyne.Theme
	textSize float32
}

func (t welcomeHeadingTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return t.textSize
	}
	return t.Theme.Size(name)
}

func welcomeHint(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

// welcomeColumnsLayout uses equal columns only when each has room for readable
// copy. Its minimum width is one column, so Home never widens the editor split.
type welcomeColumnsLayout struct {
	width float32
}

func (l *welcomeColumnsLayout) stacked(objects []fyne.CanvasObject) bool {
	minWidth := float32(320)
	for _, object := range objects {
		minWidth = fyne.Max(minWidth, object.MinSize().Width)
	}
	return l.width < minWidth*2+SpaceLG
}

func (l *welcomeColumnsLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var size fyne.Size
	stacked := l.stacked(objects)
	for i, object := range objects {
		min := object.MinSize()
		size.Width = fyne.Max(size.Width, min.Width)
		if stacked {
			size.Height += min.Height
			if i > 0 {
				size.Height += SpaceLG
			}
		} else {
			size.Height = fyne.Max(size.Height, min.Height)
		}
	}
	return size
}

func (l *welcomeColumnsLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.width = size.Width
	stacked := l.stacked(objects)
	width := size.Width
	if !stacked {
		width = (width - SpaceLG) / 2
	}
	var position fyne.Position
	for _, object := range objects {
		// Establish the wrapping width before measuring this column's height.
		object.Resize(fyne.NewSize(width, object.MinSize().Height))
		height := object.MinSize().Height
		object.Resize(fyne.NewSize(width, height))
		object.Move(position)
		if stacked {
			position.Y += height + SpaceLG
		} else {
			position.X += width + SpaceLG
		}
	}
}
