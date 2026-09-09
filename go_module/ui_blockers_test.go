package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

func sourceListenerCount(editor *SABEditor) int {
	editor.sourceSubs.mu.Lock()
	defer editor.sourceSubs.mu.Unlock()
	return len(editor.sourceSubs.fns)
}

func TestSourcePanelPopoutCloseStopsAndUnsubscribes(t *testing.T) {
	useEditorTestApp(t)
	ui := fyne.CurrentApp()
	mainWindow := ui.NewWindow("main")
	mainWindow.SetContent(widget.NewLabel("main"))

	application := &App{
		fyneApp:      ui,
		mainWindow:   mainWindow,
		sourceDrafts: NewSourceDraftStore(),
	}
	editor := NewSABEditor(application)
	application.sourcePanel = NewSourcePanel(application)
	application.sourcePanel.SetActiveEditor(editor)
	t.Cleanup(application.sourcePanel.Close)

	baseline := sourceListenerCount(editor)
	if baseline != 1 {
		t.Fatalf("primary source panel listener count = %d, want 1", baseline)
	}

	for i := range 8 {
		application.popOutSourcePanel()
		if len(application.sourcePanelMirrors) != 1 {
			t.Fatalf("iteration %d: source mirrors = %d, want 1", i, len(application.sourcePanelMirrors))
		}
		mirror := application.sourcePanelMirrors[0]
		if got := sourceListenerCount(editor); got != baseline+1 {
			t.Fatalf("iteration %d: listener count while open = %d, want %d", i, got, baseline+1)
		}

		var popout fyne.Window
		for _, candidate := range ui.Driver().AllWindows() {
			if candidate.Title() == "Source — MBII Foundry" {
				popout = candidate
			}
		}
		if popout == nil {
			t.Fatalf("iteration %d: source popout window not found", i)
		}
		popout.Close()
		fyne.DoAndWait(func() {})

		if len(application.sourcePanelMirrors) != 0 {
			t.Fatalf("iteration %d: closed mirror remains registered", i)
		}
		if got := sourceListenerCount(editor); got != baseline {
			t.Fatalf("iteration %d: listener leaked after close: got %d, want %d", i, got, baseline)
		}
		if !mirror.closed.Load() {
			t.Fatalf("iteration %d: popout panel was not disposed", i)
		}
		select {
		case <-mirror.tickerDone:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d: source refresh ticker did not stop", i)
		}
		mirror.Close() // idempotent after the window's OnClosed path
		editor.sourceSubs.fire()
		fyne.DoAndWait(func() {})
		if got := sourceListenerCount(editor); got != baseline {
			t.Fatalf("iteration %d: closed panel callback re-subscribed: %d", i, got)
		}
		if application.sourcePanel.closed.Load() {
			t.Fatalf("iteration %d: closing a mirror disposed the primary panel", i)
		}
	}
}

func TestAssetBrowserQuickNavResolvesCanonicalDirectoryAndFile(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	playerDir := filepath.Join(root, "models", "players", "kyle")
	if err := os.MkdirAll(playerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(playerDir, "model.glm"), []byte("model"), 0o644); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(root, "models", "players", "readme.txt")
	if err := os.WriteFile(filePath, []byte("asset"), 0o644); err != nil {
		t.Fatal(err)
	}

	vfs := NewVirtualFileSystem("", root)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	browser := &AssetBrowser{
		assets:      make(map[string]*AssetEntry),
		rootEntries: []*AssetEntry{},
		viewMode:    ViewModeGrid,
		sortMode:    SortNameAsc,
		iconSize:    100,
		vfs:         vfs,
	}
	browser.createUI()

	selectedTreePath := ""
	originalOnSelected := browser.tree.OnSelected
	browser.tree.OnSelected = func(id widget.TreeNodeID) {
		selectedTreePath = id
		originalOnSelected(id)
	}

	browser.quickNavSelect.SetSelected("Player Models")
	if browser.currentPK3 != "VFS" || browser.currentDir == nil || browser.currentDir.Path != "models/players" {
		t.Fatalf("Quick Nav directory = %#v (source %q), want canonical VFS models/players", browser.currentDir, browser.currentPK3)
	}
	if selectedTreePath != "models/players" {
		t.Fatalf("tree selection = %q, want models/players", selectedTreePath)
	}
	if got := browser.breadcrumbLabel.Text; got != "VFS / models/players" {
		t.Fatalf("breadcrumb = %q", got)
	}
	if len(browser.currentDir.Children) != 2 {
		t.Fatalf("directory grid has %d children, want 2", len(browser.currentDir.Children))
	}

	browser.navigateToPath("MODELS/PLAYERS/README.TXT")
	selected := browser.GetSelectedAsset()
	if selected == nil || selected.Path != "models/players/readme.txt" {
		t.Fatalf("file Quick Nav selection = %#v", selected)
	}
	if browser.selectedItem == nil || !browser.selectedItem.selected {
		t.Fatal("file Quick Nav did not highlight the matching grid item")
	}

	previousDir := browser.currentDir
	browser.navigateToPath("models/players/missing")
	if browser.currentDir != previousDir {
		t.Fatal("not-found Quick Nav changed the current directory")
	}
	if !strings.Contains(browser.statusLabel.Text, "Path not found in VFS: models/players/missing") {
		t.Fatalf("not-found status = %q", browser.statusLabel.Text)
	}
}
func TestAssetBrowserEmptyStateAndClearQuickNav(t *testing.T) {
	useEditorTestApp(t)
	browser := &AssetBrowser{
		assets:      make(map[string]*AssetEntry),
		rootEntries: []*AssetEntry{},
		viewMode:    ViewModeGrid,
		sortMode:    SortNameAsc,
		iconSize:    100,
		vfs:         NewVirtualFileSystem("", ""),
	}
	browser.createUI()

	if browser.emptyState == nil || !browser.emptyState.Visible() {
		t.Fatal("initial Files pane did not show its no-asset-root state")
	}
	if browser.emptyHeadline == nil || browser.emptyHeadline.Text != "No asset root selected" {
		t.Fatal("no-asset-root guidance headline is missing")
	}
	if browser.emptySelectButton == nil || browser.emptySelectButton.Text != "Select Asset Root" ||
		browser.emptySelectButton.Importance != widget.HighImportance {
		t.Fatal("no-asset-root state lacks a prominent Select Asset Root action")
	}
	if browser.quickNavSelect.PlaceHolder != "Choose common path" {
		t.Fatalf("Quick Nav placeholder = %q", browser.quickNavSelect.PlaceHolder)
	}
	if findTooltipByText(browser.topBar, "Jump to a common asset directory in the selected virtual filesystem") == nil {
		t.Fatal("Go to asset path help tooltip is missing")
	}
	for _, tooltip := range []string{
		"Add the current folder to favorites",
		"Switch to list view",
		"Go to parent folder",
		"Go to home folder",
		"Refresh the current folder",
	} {
		if findTooltipByText(browser.topBar, tooltip) == nil {
			t.Fatalf("pane icon action lacks tooltip %q", tooltip)
		}
	}

	browser.loadGrid(&AssetEntry{Path: "models", IsDir: true, PK3Source: "VFS"})
	if browser.emptyState.Visible() {
		t.Fatal("no-source state remained visible after browser content loaded")
	}
}

func findTooltipByText(root fyne.CanvasObject, tooltip string) *TooltipButton {
	if root == nil {
		return nil
	}
	if button, ok := root.(*TooltipButton); ok && button.tooltipText == tooltip {
		return button
	}
	for _, child := range childObjects(root) {
		if button := findTooltipByText(child, tooltip); button != nil {
			return button
		}
	}
	return nil
}

func TestCharacterSummaryConstructsWithoutFyneImageError(t *testing.T) {
	useEditorTestApp(t)
	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&output)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	}()

	summary := NewCharacterSummaryWidget(&parsers.MBCHCharacter{
		Name:      "Kyle",
		MBClass:   "MB_CLASS_JEDI",
		MaxHealth: 100,
	}, nil, nil, nil)
	window := fyneTest.NewWindow(summary)
	defer window.Close()
	window.Resize(fyne.NewSize(480, 120))
	if captured := window.Canvas().Capture(); captured == nil || captured.Bounds().Empty() {
		t.Fatal("character summary did not render")
	}
	if strings.Contains(output.String(), "Failed to load image") || strings.Contains(output.String(), "param mismatch") {
		t.Fatalf("character summary emitted a Fyne image error:\n%s", output.String())
	}
}
