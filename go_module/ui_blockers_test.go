package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestSourcePanelPopoutCloseUnsubscribes(t *testing.T) {
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
	t.Cleanup(editor.Session().flushPending)
	application.sourcePanel = NewSourcePanel(application)
	application.sourcePanel.SetActiveEditor(editor)
	t.Cleanup(application.sourcePanel.Close)

	for i := range 8 {
		application.popOutSourcePanel()
		if len(application.sourcePanelMirrors) != 1 {
			t.Fatalf("iteration %d: source mirrors = %d, want 1", i, len(application.sourcePanelMirrors))
		}
		mirror := application.sourcePanelMirrors[0]
		t.Cleanup(mirror.Close)
		editor.nameEntry.SetText(fmt.Sprintf("source_live_%d", i))
		live := editor.CurrentSource()
		if mirror.currentText() != live || application.sourcePanel.currentText() != live {
			t.Fatalf("iteration %d: form edit did not update both source panels", i)
		}
		closedByteCount := mirror.byteCount.Text

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

		if len(application.sourcePanelMirrors) != 0 {
			t.Fatalf("iteration %d: closed mirror remains registered", i)
		}
		mirror.Close()                 // idempotent after the window's OnClosed path
		mirror.SetActiveEditor(editor) // a closed panel cannot reattach
		editor.nameEntry.SetText(fmt.Sprintf("source_after_close_%d", i))
		if application.sourcePanel.currentText() != editor.CurrentSource() {
			t.Fatalf("iteration %d: closing a mirror stopped primary updates", i)
		}
		if mirror.currentText() != live || mirror.byteCount.Text != closedByteCount {
			t.Fatalf("iteration %d: a closed mirror still received source updates", i)
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
	if browser.browserSplit.Visible() {
		t.Fatal("no-root state retained the empty tree divider")
	}
	if browser.emptySelectButton == nil || browser.emptySelectButton.OnTapped == nil ||
		browser.emptySelectButton.Importance != widget.HighImportance {
		t.Fatal("no-asset-root state lacks a prominent source selection action")
	}

	browser.loadGrid(&AssetEntry{Path: "models", IsDir: true, PK3Source: "VFS"})
	if browser.emptyState.Visible() || !browser.browserSplit.Visible() {
		t.Fatal("loaded empty folder did not restore the browser panes")
	}
	browser.browserSplit.SetOffset(0.45)
	browser.currentDir = nil
	browser.updateEmptyState()
	if !browser.emptyState.Visible() || browser.browserSplit.Visible() {
		t.Fatal("clearing the root did not restore the full-pane empty state")
	}

	browser.assets["models"] = &AssetEntry{Path: "models", IsDir: true, PK3Source: "archive.pk3"}
	browser.updateEmptyState()
	if browser.emptyState.Visible() || !browser.browserSplit.Visible() {
		t.Fatal("loaded archive without a selected folder did not restore the browser panes")
	}
	if browser.browserSplit.Offset != 0.45 {
		t.Fatal("empty-state transition reset the tree split offset")
	}
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
