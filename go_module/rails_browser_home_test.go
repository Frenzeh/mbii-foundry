package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func browserHomeTooltipButtons(root fyne.CanvasObject) []*TooltipButton {
	if button, ok := root.(*TooltipButton); ok {
		return []*TooltipButton{button}
	}
	var buttons []*TooltipButton
	for _, child := range childObjects(root) {
		buttons = append(buttons, browserHomeTooltipButtons(child)...)
	}
	return buttons
}

func browserHomeAssertContained(t *testing.T, root, object fyne.CanvasObject) {
	t.Helper()
	position, found := objectPositionWithin(root, object)
	size := object.Size()
	bounds := root.Size()
	if !found || !object.Visible() || size.Width <= 0 || size.Height <= 0 ||
		position.X < -1 || position.Y < -1 ||
		position.X+size.Width > bounds.Width+1 || position.Y+size.Height > bounds.Height+1 {
		t.Fatalf("%T escapes %T: found=%v position=%v size=%v bounds=%v", object, root, found, position, size, bounds)
	}
}

func browserHomeAssertTextContained(t *testing.T, root fyne.CanvasObject) {
	t.Helper()
	var visit func(fyne.CanvasObject)
	visit = func(object fyne.CanvasObject) {
		if !object.Visible() {
			return
		}
		if text, ok := object.(*canvas.Text); ok && text.Text != "" {
			browserHomeAssertContained(t, root, text)
			if text.MinSize().Width > text.Size().Width+1 {
				t.Fatalf("text glyphs exceed their allocation: min=%v size=%v", text.MinSize(), text.Size())
			}
		}
		for _, child := range childObjects(object) {
			visit(child)
		}
	}
	visit(root)
}

func TestAssetBrowserChromeContainmentAndActions(t *testing.T) {
	previousDensity := CurrentDensityScale
	t.Cleanup(func() { CurrentDensityScale = previousDensity })
	for _, density := range []float32{0.8, 1, 1.25} {
		t.Run(fmt.Sprintf("density_%g", density), func(t *testing.T) {
			CurrentDensityScale = density
			ui := test.NewApp()
			ui.Settings().SetTheme(&FoundryTheme{})
			defer ui.Quit()
			browser := &AssetBrowser{
				assets: make(map[string]*AssetEntry), virtualOnly: true,
				viewMode: ViewModeGrid, sortMode: SortNameAsc, iconSize: 100,
			}
			browser.createUI()
			browser.loadGrid(&AssetEntry{IsDir: true})
			// Exercise a long selected source without navigating to a real path.
			browser.sourceSelect.Selected = strings.Repeat("source-directory/", 12)
			browser.sourceSelect.Refresh()
			buttons := browserHomeTooltipButtons(browser.container)
			var locationCopy, statusCopy, viewToggle *TooltipButton
			for _, button := range buttons {
				if button.Icon.Name() == theme.ContentCopyIcon().Name() {
					if containsObject(browser.topBar, button) {
						locationCopy = button
					} else {
						statusCopy = button
					}
				}
				if button.Icon.Name() == theme.ListIcon().Name() {
					viewToggle = button
				}
			}
			if locationCopy == nil || statusCopy == nil || viewToggle == nil {
				t.Fatal("browser copy or view actions are unavailable")
			}
			controls := []fyne.CanvasObject{
				browser.sourceSelect, browser.quickNavSelect, browser.searchEntry,
				browser.sortSelect, browser.zoomSlider, browser.breadcrumbLabel, browser.statusLabel,
			}
			for _, button := range buttons {
				controls = append(controls, button)
			}
			window := ui.NewWindow("browser chrome containment")
			window.SetPadded(false)
			window.SetContent(browser.GetContent())
			window.Show()
			defer window.Close()
			for _, width := range []float32{450, 260, 320, 450} {
				window.Resize(fyne.NewSize(width, 720))
				browser.breadcrumbLabel.SetText(strings.Repeat("asset/path/", 16) + fmt.Sprint(width))
				browser.statusLabel.SetText(strings.Repeat("A directory could not be read — ", 10) + fmt.Sprint(width))
				window.Canvas().Capture()
				if browser.container.MinSize().Width > width+1 || browser.container.Size().Width > width+1 {
					t.Fatalf("browser forces rail wider than %g: min=%v size=%v", width, browser.container.MinSize(), browser.container.Size())
				}
				for i, control := range controls {
					browserHomeAssertContained(t, browser.container, control)
					position, _ := objectPositionWithin(browser.container, control)
					for _, other := range controls[i+1:] {
						otherPosition, _ := objectPositionWithin(browser.container, other)
						if position.X < otherPosition.X+other.Size().Width-1 && otherPosition.X < position.X+control.Size().Width-1 &&
							position.Y < otherPosition.Y+other.Size().Height-1 && otherPosition.Y < position.Y+control.Size().Height-1 {
							t.Fatalf("browser controls overlap at width %g: %T and %T", width, control, other)
						}
					}
				}
				for _, selectWidget := range []*widget.Select{browser.sourceSelect, browser.quickNavSelect, browser.sortSelect} {
					for _, object := range childObjects(selectWidget) {
						if _, ok := object.(*widget.Icon); ok {
							browserHomeAssertContained(t, selectWidget, object)
						}
					}
				}
				for _, action := range []struct {
					button *TooltipButton
					label  *widget.Label
				}{{locationCopy, browser.breadcrumbLabel}, {statusCopy, browser.statusLabel}} {
					ui.Clipboard().SetContent("")
					test.Tap(action.button)
					if ui.Clipboard().Content() != action.label.Text {
						t.Fatal("copy action did not preserve the current untruncated label")
					}
				}
				window.Canvas().Focus(browser.searchEntry)
				query := strings.Repeat("long-search-token-", 20)
				browser.searchEntry.SetText("")
				test.Type(browser.searchEntry, query)
				if browser.searchEntry.Text != query || window.Canvas().Focused() != browser.searchEntry {
					t.Fatal("narrow browser search lost input or keyboard focus")
				}
				test.Tap(viewToggle)
				if browser.viewMode != ViewModeList || viewToggle.Icon.Name() != theme.GridIcon().Name() {
					t.Fatal("view toggle did not enter list mode")
				}
				test.Tap(viewToggle)
				browser.sortSelect.SetSelected(string(SortSizeDesc))
				browser.zoomSlider.SetValue(220)
				browser.ShowTopBar(false)
				if browser.topBar.Visible() {
					t.Fatal("browser chrome did not hide")
				}
				browser.ShowTopBar(true)
				if !browser.topBar.Visible() || browser.viewMode != ViewModeGrid || browser.sortMode != SortSizeDesc || browser.iconSize != 220 {
					t.Fatal("browser controls lost state after restoring chrome")
				}
			}
		})
	}
}

func TestWelcomeContentReflowsWithoutHorizontalOverflow(t *testing.T) {
	for _, recent := range []bool{false, true} {
		t.Run(fmt.Sprintf("recent_%t", recent), func(t *testing.T) {
			ui := test.NewApp()
			ui.Settings().SetTheme(&FoundryTheme{})
			defer ui.Quit()
			manager := &FileManager{}
			if recent {
				manager.recentFiles = []RecentFile{{Path: filepath.Join("assets", strings.Repeat("long-character-name-", 12)+".mbch")}}
			}
			content := NewWelcomeScreen(&App{fileManager: manager}).GetContent()
			window := ui.NewWindow("Home containment")
			window.SetPadded(false)
			window.SetContent(content)
			window.Show()
			defer window.Close()
			for _, width := range []float32{900, 360, 600, 900} {
				window.Resize(fyne.NewSize(width, 720))
				window.Canvas().Capture()
				if content.Size().Width > width+1 {
					t.Fatalf("Home expanded past its pane: size=%v width=%g", content.Size(), width)
				}
				columns := 0
				var visit func(fyne.CanvasObject)
				visit = func(object fyne.CanvasObject) {
					if !object.Visible() {
						return
					}
					if group, ok := object.(*fyne.Container); ok {
						if _, ok := group.Layout.(*welcomeColumnsLayout); ok {
							columns++
							for _, column := range group.Objects {
								browserHomeAssertContained(t, group, column)
							}
							first, second := group.Objects[0], group.Objects[1]
							if width < 900 && second.Position().Y < first.Position().Y+first.Size().Height {
								t.Fatal("Home columns did not stack when narrowed")
							}
							if width == 900 && second.Position().X < first.Position().X+first.Size().Width {
								t.Fatal("Home columns did not return side by side")
							}
						}
					}
					wrappedHeading := false
					if heading, ok := object.(*container.ThemeOverride); ok {
						_, wrappedHeading = heading.Theme.(welcomeHeadingTheme)
					}
					_, recentLink := object.(*widget.Hyperlink)
					if wrappedHeading || recentLink {
						position, found := objectPositionWithin(content, object)
						if !found || position.X < -1 || position.X+object.Size().Width > content.Size().Width+1 {
							t.Fatalf("Home text escaped pane horizontally: %T at %v size=%v", object, position, object.Size())
						}
						browserHomeAssertTextContained(t, object)
					}
					for _, child := range childObjects(object) {
						visit(child)
					}
				}
				visit(content)
				if columns == 0 {
					t.Fatal("Home column layout is missing")
				}
			}
		})
	}
}

func TestWelcomeHeadingWrapsWithoutChangingApplicationTypography(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	defer ui.Quit()
	bodySize := ui.Settings().Theme().Size(theme.SizeNameText)
	heading := welcomeHeading(strings.Repeat("Glyphs, punctuation — readable prose. ", 4),
		SizeTitle, theme.ColorNameForeground, fyne.TextAlignLeading)
	window := ui.NewWindow("wrapped Home heading")
	window.SetPadded(false)
	window.SetContent(heading)
	window.Show()
	defer window.Close()
	window.Resize(fyne.NewSize(900, 720))
	window.Canvas().Capture()
	wideHeight := heading.MinSize().Height
	window.Resize(fyne.NewSize(360, 720))
	window.Canvas().Capture()
	narrowHeight := heading.MinSize().Height
	if heading.Size().Width > 361 || narrowHeight <= wideHeight {
		t.Fatalf("heading did not reflow into its narrow allocation: size=%v wideHeight=%g narrowHeight=%g",
			heading.Size(), wideHeight, narrowHeight)
	}
	browserHomeAssertTextContained(t, heading)
	window.Resize(fyne.NewSize(900, 720))
	window.Canvas().Capture()
	if heading.MinSize().Height >= narrowHeight {
		t.Fatal("heading retained narrow wrapping after widening")
	}
	if ui.Settings().Theme().Size(theme.SizeNameText) != bodySize {
		t.Fatal("Home heading changed application-wide typography")
	}
}
