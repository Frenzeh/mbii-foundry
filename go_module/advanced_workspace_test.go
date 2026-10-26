package main

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

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
	message := strings.Repeat("é", 119) + "🙂界 " + strings.Repeat("diagnostic path/λ/", 20)
	draft := SourceDraft{Text: "malformed {", ParseErr: message, EditMode: true}
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
	if !utf8.ValidString(panel.validationMsg.Text) {
		t.Fatalf("diagnostic preview splits a Unicode rune: %q", panel.validationMsg.Text)
	}
	if !strings.Contains(panel.validationMsg.Text, "…") || !panel.validationDetails.Visible() {
		t.Fatal("bounded diagnostic preview has no full-detail action")
	}
	a.mainWindow.SetContent(panel.GetContent())
	a.mainWindow.Resize(fyne.NewSize(400, 720))
	test.Tap(panel.validationDetails)
	detail := findLabelByPrefix(a.mainWindow.Canvas().Overlays().Top(), message)
	if detail == nil || detail.Text != message {
		t.Fatal("complete Unicode diagnostic is not accessible from the rail")
	}
}

func TestSourceDiagnosticPreviewBoundsUnicodeAndMultilineErrors(t *testing.T) {
	for _, count := range []int{sourceDiagnosticPreviewRunes - 1, sourceDiagnosticPreviewRunes, sourceDiagnosticPreviewRunes + 1} {
		message := strings.Repeat("界", count)
		preview := sourceDiagnosticPreview(message)
		want := message
		if count > sourceDiagnosticPreviewRunes {
			want = strings.Repeat("界", sourceDiagnosticPreviewRunes) + "…"
		}
		if !utf8.ValidString(preview) || preview != want {
			t.Fatalf("Unicode preview boundary failed for %d runes: %q", count, preview)
		}
	}
	multiline := sourceDiagnosticPreview(strings.Repeat("界\r\n\t", sourceDiagnosticPreviewRunes))
	if strings.ContainsAny(multiline, "\r\n\t") || utf8.RuneCountInString(multiline) > sourceDiagnosticPreviewRunes+1 {
		t.Fatalf("multiline diagnostic escapes bounded rail preview: %q", multiline)
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

func TestSourceChromeReflowsAcrossRailWidths(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("source reflow")
	t.Cleanup(window.Close)
	a := &App{
		fyneApp: ui, mainWindow: window, sourceDrafts: NewSourceDraftStore(),
		editors: make(map[*container.TabItem]Editor),
	}
	a.docTabs = container.NewDocTabs()
	editor := NewSABEditor(a)
	a.sourceDrafts.Set(editor, SourceDraft{
		Text: "malformed {", ParseErr: strings.Repeat("界/long-diagnostic-path/", 20), EditMode: true,
	})
	workspace := NewAdvancedWorkspace(a)
	t.Cleanup(workspace.Close)
	workspace.SetActiveEditor(editor)
	doc := workspace.documents[editor]
	panel := NewSourcePanel(a)
	t.Cleanup(panel.Close)
	panel.SetDeferredValidation(true)
	panel.SetActiveEditor(editor)

	for _, replace := range []bool{false, true} {
		workspace.showSearch(doc, replace)
		for _, root := range []fyne.CanvasObject{panel.GetContent(), doc.content, workspace.GuidedBanner()} {
			for _, width := range []float32{760, 260, 400, 260, 760} {
				root.Resize(fyne.NewSize(width, 720))
				test.LaidOutObjects(root)
				root.Refresh()
				test.LaidOutObjects(root)
				assertSourceChromeContained(t, root)
				if root.MinSize().Width > width {
					t.Fatalf("chrome retains a wider minimum after rail resize: width=%.0f min=%v", width, root.MinSize())
				}
			}
		}
	}
	if !panel.applyBtn.Visible() || !panel.revertBtn.Visible() || !doc.applyBtn.Visible() || !doc.findEntry.Visible() || !doc.replaceEntry.Visible() {
		t.Fatal("reflow hid source editing or search actions")
	}
}

func assertSourceChromeContained(t *testing.T, root fyne.CanvasObject) {
	t.Helper()
	// Scroll content deliberately exceeds its viewport; chrome must not need
	// scrolling to reach actions. Native Entry/TextGrid internals scroll too.
	if _, ok := root.(*container.Scroll); ok {
		return
	}
	parent, ok := root.(*fyne.Container)
	if !ok {
		return
	}
	for _, child := range parent.Objects {
		if !child.Visible() {
			continue
		}
		position, size := child.Position(), child.Size()
		if position.X < 0 || position.Y < 0 || position.X+size.Width > root.Size().Width+1 || position.Y+size.Height > root.Size().Height+1 {
			t.Fatalf("%T exceeds chrome bounds: position=%v size=%v parent=%v", child, position, size, root.Size())
		}
		if _, scroll := child.(*container.Scroll); !scroll && size.Width+1 < child.MinSize().Width {
			t.Fatalf("%T clips its measured content: size=%v minimum=%v", child, size, child.MinSize())
		}
		assertSourceChromeContained(t, child)
	}
}

func TestAdvancedDiagnosticsWrapAndNavigateAfterWindowResize(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	t.Cleanup(ui.Quit)
	window := ui.NewWindow("source diagnostics")
	t.Cleanup(window.Close)
	code := NewSourceCodeEditor()
	code.SetText("prefix 界🙂 suffix")
	window.SetContent(code)
	window.Resize(fyne.NewSize(760, 720))
	message := "Unicode diagnostic: " + strings.Repeat("long/path/界🙂/", 30)
	diagnostic := SourceDiagnostic{
		Range: SourceRange{Start: len("prefix "), End: len("prefix 界🙂")},
		Line:  1, Column: 8, Severity: SourceError, Message: message,
	}
	doc := &advancedDocumentView{code: code, analysis: SourceAnalysis{Diagnostics: []SourceDiagnostic{diagnostic}}}
	workspace := &AdvancedWorkspace{app: &App{fyneApp: ui, mainWindow: window}}
	workspace.showDiagnostics(doc)
	for _, width := range []float32{760, 400, 760, 400} {
		window.Resize(fyne.NewSize(width, 720))
		overlay := window.Canvas().Overlays().Top()
		// Recreating Fyne's overlay renderer treats its centered position as manual placement.
		overlay.Refresh()
		label := findLabelByPrefix(overlay, message)
		if label == nil || label.Text != message {
			t.Fatal("diagnostic description was lost or truncated")
		}
		position, ok := objectPositionWithin(overlay, label)
		position = position.Add(overlay.Position())
		if !ok || position.X < 0 || position.X+label.Size().Width > window.Canvas().Size().Width+1 {
			t.Fatalf("diagnostic escapes resized window: position=%v size=%v viewport=%v", position, label.Size(), window.Canvas().Size())
		}
		if label.Size().Height < label.MinSize().Height {
			t.Fatal("diagnostic's wrapped lines are vertically clipped")
		}
	}
	selectButton := findButtonByText(window.Canvas().Overlays().Top(), "ERROR — line 1, col 8")
	if selectButton == nil {
		t.Fatal("diagnostic location is not actionable")
	}
	test.Tap(selectButton)
	if start, end := code.Selection(); start != diagnostic.Range.Start || end != diagnostic.Range.End {
		t.Fatalf("diagnostic selected incorrect Unicode byte range: [%d,%d)", start, end)
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
