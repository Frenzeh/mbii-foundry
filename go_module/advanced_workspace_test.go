package main

import (
	"encoding/json"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestAdvancedWorkspaceSwapsTabPresentationWithoutApplyingDraft(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("advanced presentation")
	t.Cleanup(window.Close)

	a := &App{
		fyneApp:      ui,
		mainWindow:   window,
		sourceDrafts: NewSourceDraftStore(),
		editors:      make(map[*container.TabItem]Editor),
		config:       AppConfig{EditorMode: editorModeGuided},
	}
	a.docTabs = container.NewDocTabs()
	editor := NewSABEditor(a)
	guided := editor.GetContent()
	tab := container.NewTabItem("Saber", guided)
	a.editors[tab] = editor
	a.docTabs.Append(tab)
	a.docTabs.Select(tab)

	originalModel := editor.CurrentSource()
	draft := SourceDraft{Text: "broken {", ParseErr: "retained language error", EditMode: true}
	a.sourceDrafts.Set(editor, draft)
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	a.advancedWorkspace = workspace
	workspace.SetActiveEditor(editor)
	doc := workspace.documents[editor]
	if doc == nil {
		t.Fatal("advanced view was not created for the active document")
	}
	if doc.code.Text() != draft.Text {
		t.Fatalf("advanced view did not bind retained raw draft: %q", doc.code.Text())
	}
	if !doc.analyzed || sourceAnalysisError(doc.analysis) == "" {
		t.Fatalf("initial raw draft was not analyzed before reporting status: %+v", doc.analysis.Diagnostics)
	}
	stored, ok := a.sourceDrafts.Get(editor)
	if !ok || stored != draft {
		t.Fatalf("hidden Advanced view rewrote the Guided-owned draft validation: %+v ok=%v", stored, ok)
	}
	if strings.Contains(doc.diagnosticBtn.Text, "refresh on edit") {
		t.Fatalf("initial diagnostics falsely deferred analysis: %q", doc.diagnosticBtn.Text)
	}
	if !strings.Contains(doc.cursorStatus.Text, "/ 8") {
		t.Fatalf("byte status assessed stale Guided source instead of raw draft: %q", doc.cursorStatus.Text)
	}
	if doc.code.OnSaveRequested == nil || doc.code.OnUndoFallback == nil || doc.code.OnRedoFallback == nil {
		t.Fatal("advanced code surface is not bound to guarded save/history routing")
	}
	if doc.code.OnUndoFallback() {
		t.Fatal("pending raw draft fell through to Guided session undo")
	}

	workspace.SetMode(editorModeAdvanced)
	if tab.Content != doc.content || a.docTabs.Selected() != tab {
		t.Fatal("advanced mode replaced tab identity instead of swapping its presentation")
	}
	workspace.SetMode(editorModeGuided)
	if tab.Content != guided || a.docTabs.Selected() != tab {
		t.Fatal("guided mode did not restore the original tab presentation and selection")
	}
	if got, ok := a.sourceDrafts.Get(editor); !ok || got != stored {
		t.Fatalf("presentation switch mutated retained draft: %+v ok=%v", got, ok)
	}
	if editor.CurrentSource() != originalModel {
		t.Fatal("presentation switch applied raw source to the Guided model")
	}

	// Advanced owns ParseErr only while it is the visible editor.
	a.config.EditorMode = editorModeAdvanced
	workspace.syncDocumentText(doc)
	reconciled, ok := a.sourceDrafts.Get(editor)
	if !ok || reconciled.Text != draft.Text || !reconciled.EditMode || reconciled.ParseErr != sourceAnalysisError(doc.analysis) {
		t.Fatalf("Advanced analysis did not own the visible draft validation: %+v ok=%v", reconciled, ok)
	}
}

func TestAdvancedWorkspaceDisposesClosedAssetTarget(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("closed target")
	t.Cleanup(window.Close)
	a := &App{
		fyneApp: ui, mainWindow: window,
		sourceDrafts: NewSourceDraftStore(), editors: make(map[*container.TabItem]Editor),
	}
	a.docTabs = container.NewDocTabs()
	editor := NewSABEditor(a)
	tab := container.NewTabItem("Saber", editor.GetContent())
	a.editors[tab] = editor
	a.docTabs.Append(tab)
	a.docTabs.Select(tab)
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	a.advancedWorkspace = workspace
	workspace.SetActiveEditor(editor)
	doc := workspace.documents[editor]

	text := "test_saber\n{\n\tsaberModel \"models/weapons2/saber/test/model.glm\"\n}\n"
	doc.code.SetText(text)
	caret := strings.Index(text, "models/weapons2") + len("models/weapons2")
	doc.code.SelectRange(caret, caret)
	target, ok := captureSourceAssetTarget(doc.code, SourceFormatSAB)
	if !ok {
		t.Fatal("real asset target was not captured before close")
	}

	workspace.RemoveEditor(editor)
	if !doc.code.IsReadOnly() {
		t.Fatal("closed document code surface remained writable")
	}
	if doc.code.OnChanged != nil || doc.code.OnCursorChanged != nil ||
		doc.code.OnRequestCompletions != nil || doc.code.OnSearchRequested != nil ||
		doc.code.OnAssetRequested != nil || doc.code.OnSaveRequested != nil ||
		doc.code.OnUndoFallback != nil || doc.code.OnRedoFallback != nil ||
		doc.code.OnUnhandledShortcut != nil {
		t.Fatal("closed document retained an active callback")
	}
	if target.apply("models/weapons2/saber/replacement/model.glm", true) {
		t.Fatal("pending picker applied to a closed document")
	}
	if doc.code.Text() != text || a.sourceDrafts.Has(editor) {
		t.Fatal("closed picker target mutated text or recreated an inaccessible draft")
	}
}

func TestAdvancedCodeForwardsUnhandledApplicationShortcuts(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("shortcut forwarding")
	t.Cleanup(window.Close)
	a := &App{
		fyneApp: ui, mainWindow: window,
		sourceDrafts: NewSourceDraftStore(), editors: make(map[*container.TabItem]Editor),
		config: AppConfig{EditorMode: editorModeAdvanced},
	}
	a.docTabs = container.NewDocTabs()
	first, second := NewSABEditor(a), NewVEHEditor(a)
	firstTab := container.NewTabItem("First", first.GetContent())
	secondTab := container.NewTabItem("Second", second.GetContent())
	a.editors[firstTab], a.editors[secondTab] = first, second
	a.docTabs.Append(firstTab)
	a.docTabs.Append(secondTab)
	a.docTabs.Select(firstTab)
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	a.advancedWorkspace = workspace
	workspace.SetActiveEditor(first)
	doc := workspace.documents[first]
	if doc.code.OnUnhandledShortcut == nil {
		t.Fatal("advanced code surface has no application shortcut host")
	}
	doc.code.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.Key2, Modifier: fyne.KeyModifierShortcutDefault})
	if a.docTabs.Selected() != secondTab {
		t.Fatal("focused code surface swallowed the application tab shortcut")
	}
}

func TestGuidedPendingDraftBannerKeepsStatusAboveActions(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("guided banner")
	t.Cleanup(window.Close)
	a := &App{
		fyneApp: ui, mainWindow: window,
		sourceDrafts: NewSourceDraftStore(), editors: make(map[*container.TabItem]Editor),
	}
	a.docTabs = container.NewDocTabs()
	editor := NewSABEditor(a)
	a.sourceDrafts.Set(editor, SourceDraft{Text: "broken {", ParseErr: "unclosed block", EditMode: true})
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	workspace.SetActiveEditor(editor)
	workspace.refreshGuidedBanner()

	workspace.guidedBanner.Resize(fyne.NewSize(420, 100))
	content, ok := workspace.guidedBanner.Objects[0].(*fyne.Container)
	if !ok || len(content.Objects) != 2 {
		t.Fatalf("guided banner does not expose separate status and action rows: %#v", workspace.guidedBanner.Objects)
	}
	status, actions := content.Objects[0], content.Objects[1]
	if actions.Position().Y < status.Position().Y+status.Size().Height {
		t.Fatalf("guided banner actions overlap status: status=%v/%v actions=%v/%v", status.Position(), status.Size(), actions.Position(), actions.Size())
	}
	if !workspace.guidedLabel.Visible() || workspace.guidedLabel.Text == "" {
		t.Fatal("guided pending-draft status was hidden to avoid overlap")
	}
}

func TestAdvancedWorkspaceRetainsPerDocumentBufferHistory(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("advanced history")
	t.Cleanup(window.Close)

	a := &App{
		fyneApp:      ui,
		mainWindow:   window,
		sourceDrafts: NewSourceDraftStore(),
		editors:      make(map[*container.TabItem]Editor),
		config:       AppConfig{EditorMode: editorModeAdvanced},
	}
	a.docTabs = container.NewDocTabs()
	first := NewSABEditor(a)
	second := NewVEHEditor(a)
	firstTab := container.NewTabItem("First", first.GetContent())
	secondTab := container.NewTabItem("Second", second.GetContent())
	a.editors[firstTab], a.editors[secondTab] = first, second
	a.docTabs.Append(firstTab)
	a.docTabs.Append(secondTab)

	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	a.advancedWorkspace = workspace
	workspace.SetMode(editorModeAdvanced)
	firstDoc := workspace.documents[first]
	secondDoc := workspace.documents[second]
	if firstDoc == nil || secondDoc == nil {
		t.Fatal("advanced mode did not allocate independent document views")
	}
	firstDoc.code.ReplaceRange(len(firstDoc.code.Text()), len(firstDoc.code.Text()), "\n// first draft")
	firstText := firstDoc.code.Text()
	if !firstDoc.code.CanUndo() {
		t.Fatal("first document did not retain local raw-text history")
	}

	a.docTabs.Select(secondTab)
	workspace.SetActiveEditor(second)
	secondDoc.code.ReplaceRange(len(secondDoc.code.Text()), len(secondDoc.code.Text()), "\n// second draft")
	a.docTabs.Select(firstTab)
	workspace.SetActiveEditor(first)
	workspace.SetMode(editorModeGuided)
	workspace.SetMode(editorModeAdvanced)
	if workspace.documents[first] != firstDoc || firstDoc.code.Text() != firstText || !firstDoc.code.CanUndo() {
		t.Fatal("tab/mode changes discarded the first document's buffer or undo history")
	}
	if firstDraft, ok := a.sourceDrafts.Get(first); !ok || firstDraft.Text != firstText {
		t.Fatalf("first document draft was not isolated: %+v ok=%v", firstDraft, ok)
	}
	if secondDraft, ok := a.sourceDrafts.Get(second); !ok || !strings.Contains(secondDraft.Text, "second draft") {
		t.Fatalf("second document draft was not isolated: %+v ok=%v", secondDraft, ok)
	}
}

func TestSourcePanelDeferredValidationPreservesAdvancedDiagnostic(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	a := &App{fyneApp: ui, mainWindow: ui.NewWindow("deferred source"), sourceDrafts: NewSourceDraftStore()}
	t.Cleanup(a.mainWindow.Close)
	editor := NewSABEditor(a)
	draft := SourceDraft{Text: "malformed {", ParseErr: "advanced diagnostic sentinel", EditMode: true}
	a.sourceDrafts.Set(editor, draft)
	panel := NewSourcePanel(a)
	t.Cleanup(panel.Close)
	panel.SetDeferredValidation(true)
	panel.SetActiveEditor(editor)

	if panel.lastParseErr != draft.ParseErr {
		t.Fatalf("hidden Guided panel reparsed Advanced draft: %q", panel.lastParseErr)
	}
	if got, ok := a.sourceDrafts.Get(editor); !ok || got != draft {
		t.Fatalf("hidden Guided panel rewrote Advanced draft: %+v ok=%v", got, ok)
	}
}

func TestAdvancedApplyConsumesOnlyExactValidDraft(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	a := &App{
		fyneApp: ui, mainWindow: ui.NewWindow("advanced apply"),
		sourceDrafts: NewSourceDraftStore(), editors: make(map[*container.TabItem]Editor),
		statusLabel: widget.NewLabel("test"),
	}
	t.Cleanup(a.mainWindow.Close)
	editor := NewSABEditor(a)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	a.docTabs = container.NewDocTabs()
	tab := container.NewTabItem("Saber", editor.GetContent())
	a.editors[tab] = editor
	a.docTabs.Append(tab)
	a.docTabs.Select(tab)
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	a.advancedWorkspace = workspace
	workspace.SetActiveEditor(editor)
	doc := workspace.documents[editor]
	draftText := editor.CurrentSource() + "\n// applied from Advanced\n"
	analysis := workspace.analyze(doc, draftText)
	if parseErr := sourceAnalysisError(analysis); parseErr != "" {
		t.Fatalf("valid fixture produced Advanced errors: %s", parseErr)
	}
	draft := SourceDraft{Text: draftText, EditMode: true}
	a.sourceDrafts.Set(editor, draft)
	workspace.applyDraft(doc)
	if _, ok := a.sourceDrafts.Get(editor); ok {
		t.Fatal("successful explicit Apply did not consume the exact draft")
	}
	if !strings.Contains(editor.CurrentSource(), "applied from Advanced") {
		t.Fatal("explicit Apply did not update the Guided working model")
	}
}

func TestAdvancedChromeMinimumsStayWithinViewport(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("bounded chrome")
	t.Cleanup(window.Close)
	a := &App{
		fyneApp: ui, mainWindow: window,
		sourceDrafts: NewSourceDraftStore(),
		editors:      make(map[*container.TabItem]Editor),
	}
	a.docTabs = container.NewDocTabs()
	editor := NewSABEditor(a)
	tab := container.NewTabItem("Saber", editor.GetContent())
	a.editors[tab] = editor
	a.docTabs.Append(tab)
	a.docTabs.Select(tab)
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	workspace.SetActiveEditor(editor)
	panel := NewSourcePanel(a)
	t.Cleanup(panel.Close)

	const viewportWidth = float32(760)
	for name, object := range map[string]fyne.CanvasObject{
		"toolbar":          a.createToolbar(),
		"guided source":    panel.GetContent(),
		"advanced content": workspace.documents[editor].content,
	} {
		if width := object.MinSize().Width; width > viewportWidth {
			t.Fatalf("%s minimum width %.1f exceeds %.1fpx viewport", name, width, viewportWidth)
		}
	}
}

func TestConfiguredSourcePaletteRejectsInvalidOverrides(t *testing.T) {
	if _, err := configuredSourcePalette("Foundry Dark", map[string]string{"caret": "not-a-colour"}); err == nil {
		t.Fatal("invalid custom syntax colour was silently accepted")
	}
	palette, err := configuredSourcePalette("Monokai", map[string]string{"caret": "#01020304"})
	if err != nil {
		t.Fatal(err)
	}
	if palette.Caret.R != 1 || palette.Caret.G != 2 || palette.Caret.B != 3 || palette.Caret.A != 4 {
		t.Fatalf("custom caret colour not applied: %+v", palette.Caret)
	}
}

func TestAdvancedPreferencesPersistModeAndSyntaxColours(t *testing.T) {
	want := AppConfig{
		EditorMode:   editorModeAdvanced,
		SyntaxTheme:  "Monokai",
		SyntaxColors: map[string]string{"background": "#010203", "caret": "#AABBCCDD"},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got AppConfig
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.EditorMode != want.EditorMode || got.SyntaxTheme != want.SyntaxTheme ||
		got.SyntaxColors["background"] != want.SyntaxColors["background"] ||
		got.SyntaxColors["caret"] != want.SyntaxColors["caret"] {
		t.Fatalf("advanced preferences did not round-trip: %+v", got)
	}
	if normalizeEditorMode("unexpected") != editorModeGuided {
		t.Fatal("unknown persisted editor mode did not fail safe to Guided")
	}
}
