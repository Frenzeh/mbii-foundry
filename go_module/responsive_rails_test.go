package main

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func assertRailObjectContained(t *testing.T, root, object fyne.CanvasObject) {
	t.Helper()
	position, ok := objectPositionWithin(root, object)
	if !ok || !object.Visible() {
		t.Fatalf("%T is not present and visible in %T", object, root)
	}
	size, bounds := object.Size(), root.Size()
	if size.Width <= 0 || size.Height <= 0 || position.X < 0 || position.Y < 0 ||
		position.X+size.Width > bounds.Width+1 || position.Y+size.Height > bounds.Height+1 {
		t.Fatalf("%T escapes %T: position=%v size=%v bounds=%v", object, root, position, size, bounds)
	}
}

func TestResponsiveSidebarKeepsActivitiesContainedAndSelected(t *testing.T) {
	useEditorTestApp(t)
	items := []*ActivityItem{
		{ID: "files", Label: "Files", Icon: theme.FolderIcon()},
		{ID: "library", Label: "Library", Icon: theme.ListIcon()},
		{ID: "modpacks", Label: "Modpacks", Icon: theme.StorageIcon()},
		{ID: "bulkedit", Label: "Bulk Edit", Icon: theme.ContentPasteIcon()},
	}
	selections := 0
	header := NewSidebarHeader(items, func(*ActivityItem) { selections++ }, nil)
	// Saved selection is applied before the native renderer exists.
	header.SetActive("library")
	window := fyne.CurrentApp().NewWindow("responsive sidebar")
	window.SetPadded(false)
	window.SetContent(header)
	window.Show()
	defer window.Close()

	for _, width := range []float32{260, 450, 320, 260} {
		window.Resize(fyne.NewSize(width, 100))
		window.Canvas().Capture()
		if window.Canvas().Size().Width > width+1 {
			t.Fatalf("sidebar forces a %.0fpx viewport wider: %v", width, window.Canvas().Size())
		}
		for _, item := range items {
			pill := header.pills[item.ID]
			assertRailObjectContained(t, header, pill)
			assertRailObjectContained(t, pill, pill.icon)
			if pill.active != (item.ID == "library") {
				t.Fatalf("resize lost the visible selection for %q", item.ID)
			}
		}
		assertRailObjectContained(t, header, header.collapseBtn)
		last := header.pills["bulkedit"]
		lastPosition, _ := objectPositionWithin(header, last)
		collapsePosition, _ := objectPositionWithin(header, header.collapseBtn)
		if lastPosition.X+last.Size().Width > collapsePosition.X {
			t.Fatal("collapse control overlaps the last activity")
		}
	}
	if selections != 1 || header.ActiveID() != "library" {
		t.Fatal("resizing changed the selected activity or repeated its callback")
	}
}

func TestResponsiveInfoHeaderKeepsLongIdentityAndActionsInsideRail(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	panel := NewInfoPanel()
	panel.updateHeaderChips("MB_ATT_ARC_RIFLE_GRENADELAUNCHER_SECONDARY_AMMUNITION", "Attribute · Class Specific")
	panel.title.SetText("Secondary ammunition — glyphs, descenders (gyjp) and punctuation")
	window := fyne.CurrentApp().NewWindow("responsive reference")
	window.SetPadded(false)
	window.SetContent(panel.GetContent())
	window.Show()
	defer window.Close()

	for _, width := range []float32{450, 260, 320, 450} {
		window.Resize(fyne.NewSize(width, 720))
		window.Canvas().Capture()
		if window.Canvas().Size().Width > width+1 {
			t.Fatalf("reference header pins a %.0fpx rail wider", width)
		}
		for _, text := range []fyne.CanvasObject{panel.categoryChip, panel.idChip, panel.title} {
			assertRailObjectContained(t, window.Content(), text)
		}
		var checkActions func(fyne.CanvasObject)
		checkActions = func(object fyne.CanvasObject) {
			if !object.Visible() {
				return
			}
			if button, ok := object.(*TooltipButton); ok && button.Visible() {
				assertRailObjectContained(t, window.Content(), button)
			}
			for _, child := range childObjects(object) {
				checkActions(child)
			}
		}
		checkActions(panel.GetContent())
	}
}

func TestResponsiveBufferGaugeReflowsAfterAssessmentChanges(t *testing.T) {
	useEditorTestApp(t)
	gauge := NewBufferGaugeWidget()
	character := parsers.NewMBCHCharacter()
	window := fyne.CurrentApp().NewWindow("responsive buffer assessment")
	window.SetPadded(false)
	window.SetContent(gauge.GetContent())
	window.Show()
	defer window.Close()

	for _, width := range []float32{1200, 360, 600} {
		window.Resize(fyne.NewSize(width, 240))
		for _, name := range []string{"glyphs_gyjp", "unterminated\""} {
			character.Name = name
			gauge.Update(character)
			window.Canvas().Capture()
			if window.Canvas().Size().Width > width+1 {
				t.Fatalf("buffer status widens a %.0fpx viewport", width)
			}
			assertRailObjectContained(t, window.Content(), gauge.label)
			assertRailObjectContained(t, window.Content(), gauge.badge)
		}
	}
}

func TestResponsiveCollapsedRailKeyboardReopensSavedSplit(t *testing.T) {
	app := newAuditLifecycleApp(t)
	app.sidebarSplit.SetOffset(0.63)
	app.sourceSplit.SetOffset(0.74)
	app.mainWindow.Resize(fyne.NewSize(1200, 720))
	app.mainWindow.Canvas().Capture()
	app.toggleSidebar()
	app.mainWindow.Canvas().Capture()

	object := auditAssetFind(app.mainWindow.Content(), func(object fyne.CanvasObject) bool {
		_, ok := object.(*CollapsedRail)
		return ok
	})
	rail, ok := object.(*CollapsedRail)
	if !ok {
		t.Fatal("collapsing the sidebar removed its reopen control")
	}
	assertRailObjectContained(t, app.mainWindow.Content(), rail)
	focusable := auditFocusable(t, rail)
	resting := auditStroke(rail.bg)
	app.mainWindow.Canvas().Focus(focusable)
	if got := auditStroke(rail.bg); got == resting || got[3] == 0 {
		t.Fatal("collapsed rail has no visible keyboard focus")
	}
	focusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	focusable.TypedRune(' ')
	if app.sidebarVisible {
		t.Fatal("unrelated keyboard input reopened the sidebar")
	}
	focusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	app.mainWindow.Resize(fyne.NewSize(1440, 900))
	app.mainWindow.Canvas().Capture()
	if !app.sidebarVisible || app.sidebarSplit == nil || app.sourceSplit == nil {
		t.Fatal("keyboard activation did not restore the expanded shell")
	}
	if math.Abs(app.sidebarSplit.Offset-0.63) > 0.000001 || math.Abs(app.sourceSplit.Offset-0.74) > 0.000001 {
		t.Fatalf("collapse/reopen replaced saved split ratios: sidebar=%v source=%v", app.sidebarSplit.Offset, app.sourceSplit.Offset)
	}
	assertRailObjectContained(t, app.mainWindow.Content(), app.activityBar)
	assertRailObjectContained(t, app.mainWindow.Content(), app.docTabs)

	app.toggleSourcePanel()
	app.mainWindow.Canvas().Capture()
	object = auditAssetFind(app.mainWindow.Content(), func(object fyne.CanvasObject) bool {
		_, ok := object.(*CollapsedRail)
		return ok
	})
	sourceRail, ok := object.(*CollapsedRail)
	if !ok {
		t.Fatal("collapsing Source removed its reopen control")
	}
	assertRailObjectContained(t, app.mainWindow.Content(), sourceRail)
	sourceFocusable := auditFocusable(t, sourceRail)
	sourceFocusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	// A platform may also emit text input for the same key press. It must
	// not invoke the detached control again and collapse the restored pane.
	sourceFocusable.TypedRune(' ')
	app.mainWindow.Canvas().Capture()
	if !app.config.SourcePanelVisible || app.sourceSplit == nil ||
		math.Abs(app.sourceSplit.Offset-0.74) > 0.000001 || math.Abs(app.sidebarSplit.Offset-0.63) > 0.000001 {
		t.Fatal("Space did not restore Source with both saved split ratios")
	}
}

func TestResponsiveRailConfigurationRetainsWideSavedSidebar(t *testing.T) {
	auditCredentialHome(t)
	useEditorTestApp(t)
	directory, err := AppConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	auditWriteCredentialFixture(t, filepath.Join(directory, "config.json"), []byte(`{"sidebar_offset":0.67,"sidebar_visible":false,"source_panel_offset":0.72,"source_panel_visible":false}`))
	app := auditLoadCredentialApp(t, fyne.CurrentApp())
	if app.config.SidebarOffset != float32(0.67) || app.config.SourcePanelOffset != float32(0.72) ||
		app.config.SidebarVisible || app.config.SourcePanelVisible {
		t.Fatalf("loading configuration replaced the user's saved rail state: %+v", app.config)
	}
}

func TestResponsiveStatusFeedbackDoesNotCompeteWithBufferMetrics(t *testing.T) {
	app := newAuditLifecycleApp(t)
	app.bufferGauge.Update(parsers.NewMBCHCharacter())
	app.bufferGauge.GetContent().Show()
	app.updateStatus("Opened " + strings.Repeat("long_character_identifier_", 12) + ".mbch")
	app.mainWindow.Resize(fyne.NewSize(1200, 720))
	app.mainWindow.Canvas().Capture()
	if app.mainWindow.Canvas().Size().Width > 1201 {
		t.Fatal("status feedback widened the application")
	}
	assertRailObjectContained(t, app.mainWindow.Content(), app.statusLabel)
	assertRailObjectContained(t, app.mainWindow.Content(), app.bufferGauge.label)
	statusPosition, _ := objectPositionWithin(app.mainWindow.Content(), app.statusLabel)
	gaugePosition, _ := objectPositionWithin(app.mainWindow.Content(), app.bufferGauge.label)
	if statusPosition.Y+app.statusLabel.Size().Height > gaugePosition.Y {
		t.Fatal("operation feedback overlaps the buffer assessment")
	}
}

func TestTooltipActivationDismissesVisibleAndQueuedHints(t *testing.T) {
	for _, activation := range []struct {
		name string
		run  func(*TooltipButton)
	}{
		{"pointer", func(button *TooltipButton) { test.Tap(button) }},
		{"keyboard", func(button *TooltipButton) {
			button.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
			button.TypedRune(' ')
		}},
	} {
		for _, state := range []string{"pending", "visible"} {
			t.Run(activation.name+"/"+state, func(t *testing.T) {
				useEditorTestApp(t)
				window := fyne.CurrentApp().NewWindow("tooltip activation")
				var actionPopup *widget.PopUp
				activations := 0
				button := NewTooltipButton("Open", nil, func() {
					activations++
					actionPopup = widget.NewPopUp(widget.NewLabel("Action content"), window.Canvas())
					actionPopup.Show()
				}, "Delayed action hint")
				window.SetContent(button)
				window.Resize(fyne.NewSize(320, 200))
				window.Show()
				defer window.Close()
				window.Canvas().Capture()

				// Drive the queued UI callback explicitly instead of sleeping
				// for a hover timer or leaving a goroutine behind the test.
				button.hovering = true
				if state == "visible" {
					button.showTooltip()
				}
				hint := button.popUp
				activation.run(button)
				button.showTooltip()
				if activations != 1 || actionPopup == nil || !actionPopup.Visible() || !containsObject(window.Canvas().Overlays().Top(), actionPopup) {
					t.Fatal("a delayed tooltip obscured the activated action")
				}
				if hint != nil && hint.Visible() {
					t.Fatal("the previous tooltip remained visible behind the action")
				}
			})
		}
	}
}
