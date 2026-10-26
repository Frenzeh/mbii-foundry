package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func newAuditLifecycleApp(t *testing.T) *App {
	t.Helper()
	usePublicCatalogs(t)
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), configDir)
	app.configPath = filepath.Join(configDir, "config.json")
	app.config.SidebarVisible = true
	app.config.SourcePanelVisible = true
	app.config.SidebarOffset = 0.30
	app.config.SourcePanelOffset = 0.70
	app.setupUI()
	app.setupShortcuts()
	app.mainWindow.Resize(fyne.NewSize(1200, 800))
	app.mainWindow.Show()
	t.Cleanup(func() {
		app.sourcePanel.Close()
		for _, mirror := range app.sourcePanelMirrors {
			mirror.Close()
		}
	})
	if app.mainWindow.Canvas().Overlays().Top() != nil {
		t.Fatal("lifecycle fixture opened with an unexpected dialog")
	}
	return app
}

func auditLifecycleSaber(t *testing.T, app *App) *SABEditor {
	t.Helper()
	editor := NewSABEditor(app)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	return editor
}

func auditLifecycleEntries(t *testing.T, app *App) []*RecoveryEntry {
	t.Helper()
	entries, err := app.recovery.List()
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func auditLifecycleCloseMenu(t *testing.T, app *App) {
	t.Helper()
	for _, menu := range app.mainWindow.MainMenu().Items {
		for _, item := range menu.Items {
			if item.Label == "Close Tab" && item.Action != nil {
				item.Action()
				return
			}
		}
	}
	t.Fatal("Close Tab menu action is unavailable")
}

func auditLifecycleQuitItem(t *testing.T, app *App) *fyne.MenuItem {
	t.Helper()
	for _, menu := range app.mainWindow.MainMenu().Items {
		for _, item := range menu.Items {
			if item.IsQuit {
				if item.Action == nil {
					t.Fatal("Quit has no guarded action")
				}
				return item
			}
		}
	}
	t.Fatal("application menu has no guarded Quit action")
	return nil
}

func TestAuditLifecycleCloseInterceptAndMenuOwnVisibleTabs(t *testing.T) {
	app := newAuditLifecycleApp(t)
	mirror := NewSourcePanel(app)
	app.sourcePanelMirrors = append(app.sourcePanelMirrors, mirror)
	first := auditLifecycleSaber(t, app)
	app.createNewFile("first", first)
	firstTab := app.docTabs.Selected()
	draft := SourceDraft{Text: "pending source\n{", ParseErr: "unbalanced braces", EditMode: true}
	app.sourceDrafts.Set(first, draft)
	app.captureRecoveryEntry(first)
	if len(auditLifecycleEntries(t, app)) != 1 {
		t.Fatal("source-only work was not captured before close")
	}
	second := auditLifecycleSaber(t, app)
	app.createNewFile("second", second)
	secondTab := app.docTabs.Selected()

	if app.docTabs.CloseIntercept == nil {
		t.Fatal("tab close must be intercepted before Fyne removes the document")
	}
	app.docTabs.CloseIntercept(firstTab)
	if len(app.docTabs.Items) != 2 || app.editors[firstTab] != first {
		t.Fatal("close request removed a visible tab before confirmation")
	}
	test.Tap(saveDialogButton(t, app.mainWindow.Canvas().Overlays().Top(), "No"))
	if got, ok := app.sourceDrafts.Get(first); !ok || got != draft || len(auditLifecycleEntries(t, app)) != 1 {
		t.Fatal("cancelled close discarded the draft or its recovery snapshot")
	}
	app.docTabs.CloseIntercept(firstTab)
	test.Tap(saveDialogButton(t, app.mainWindow.Canvas().Overlays().Top(), "Yes"))
	if len(app.docTabs.Items) != 1 || app.docTabs.Items[0] != secondTab || len(app.editors) != 1 {
		t.Fatal("confirmed close did not remove exactly the requested visible tab and registration")
	}
	if app.sourceDrafts.Has(first) || len(auditLifecycleEntries(t, app)) != 0 {
		t.Fatal("confirmed discard retained a draft or recovery snapshot")
	}
	if app.sourcePanel.editorRef != second || mirror.editorRef != second {
		t.Fatal("closing an inactive tab cleared the selected document's source panels")
	}

	third := auditLifecycleSaber(t, app)
	app.createNewFile("third", third)
	auditLifecycleCloseMenu(t, app)
	if len(app.docTabs.Items) != 1 || app.editors[app.docTabs.Selected()] != second {
		t.Fatal("menu close left a visible unregistered tab or selected the wrong survivor")
	}
	if app.sourcePanel.editorRef != second || mirror.editorRef != second {
		t.Fatal("menu close did not rebind source panels to the surviving selection")
	}
	auditLifecycleCloseMenu(t, app)
	if len(app.editors) != 0 || len(app.docTabs.Items) != 1 || app.docTabs.Items[0].Text != "Home" {
		t.Fatal("closing the last document did not replace it with Home")
	}
	if app.sourcePanel.editorRef != nil || mirror.editorRef != nil {
		t.Fatal("Home retained a closed editor in a source panel")
	}
}

func TestAuditLifecycleReattachKeepsLiveDirtyTitle(t *testing.T) {
	app := newAuditLifecycleApp(t)
	editor := auditLifecycleSaber(t, app)
	app.createNewFile("reattach", editor)
	app.popOutCurrentTab()
	window := tornOutWindow(t, app)
	findTooltipButton(t, window.Content(), "Reattach tab").OnTapped()
	tab := app.docTabs.Selected()
	if app.editors[tab] != editor || len(app.detachedEditors) != 0 {
		t.Fatal("reattach did not return the same editor to the live tab registry")
	}
	editor.nameEntry.SetText("edited_after_reattach")
	editor.session.flushPending()
	if !strings.HasPrefix(tab.Text, "* ") || !editor.IsDirty() {
		t.Fatalf("reattached editor's dirty callback updated a dead tab: title=%q", tab.Text)
	}
	if app.sourcePanel.editorRef != editor {
		t.Fatal("reattach did not bind the selected editor's source")
	}
}

func TestAuditLifecycleIdenticalSaveAsAdoptsDestination(t *testing.T) {
	app := newAuditLifecycleApp(t)
	editor := auditLifecycleSaber(t, app)
	originalPath := editor.GetCurrentPath()
	original, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	app.createNewFile("save-as", editor)
	tab := app.docTabs.Selected()
	destination := filepath.Join(t.TempDir(), "identical-copy.sab")
	candidate, err := editor.PrepareSave(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte(candidate), 0600); err != nil {
		t.Fatal(err)
	}
	completed := false
	var saveErr error
	finish := func(err error) { completed, saveErr = true, err }
	app.saveWithReviewCompletion(editor, tab, destination, finish)
	if overlay := app.mainWindow.Canvas().Overlays().Top(); overlay != nil {
		test.Tap(saveDialogButton(t, overlay, "Save"))
	}
	if !completed || saveErr != nil {
		t.Fatalf("identical Save As did not complete: completed=%v err=%v", completed, saveErr)
	}
	if editor.GetCurrentPath() != destination || editor.Session().Path != destination || editor.Session().OriginalBytes != candidate {
		t.Fatal("identical Save As returned success without adopting its destination and baseline")
	}
	if editor.IsDirty() || tab.Text != filepath.Base(destination) {
		t.Fatal("identical Save As did not update the visible clean document identity")
	}
	if recent := app.fileManager.GetRecentFiles(); len(recent) == 0 || recent[0].Path != destination {
		t.Fatal("Save As did not record its adopted destination in recent files")
	}

	editor.nameEntry.SetText("after_save_as")
	editor.session.flushPending()
	completed = false
	app.saveWithReviewCompletion(editor, tab, editor.GetCurrentPath(), finish)
	test.Tap(saveDialogButton(t, app.mainWindow.Canvas().Overlays().Top(), "Save"))
	if !completed || saveErr != nil {
		t.Fatalf("subsequent save failed: completed=%v err=%v", completed, saveErr)
	}
	unchanged, err := os.ReadFile(originalPath)
	if err != nil || string(unchanged) != string(original) {
		t.Fatalf("subsequent save changed the old original: err=%v", err)
	}
	published, err := os.ReadFile(destination)
	if err != nil || !strings.Contains(string(published), "after_save_as") {
		t.Fatalf("subsequent save did not target the adopted destination: err=%v", err)
	}
}

func TestAuditLifecycleFreshSourceOnlyRecoveryPreservesAllFormats(t *testing.T) {
	usePublicCatalogs(t)
	for _, format := range []string{".mbch", ".sab", ".veh", ".siege"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
			editor := newEditorForFormat(app, format).(SessionEditor)
			working := editor.CurrentSource()
			if editor.IsDirty() {
				t.Fatal("fresh form is already dirty before a source-only edit")
			}
			draft := SourceDraft{Text: "unapplied " + format + "\n{", ParseErr: "unbalanced braces", EditMode: true}
			app.sourceDrafts.Set(editor, draft)
			app.captureRecoveryEntry(editor)
			entries := auditLifecycleEntries(t, app)
			if len(entries) != 1 || !entries[0].HasDraft {
				t.Fatal("fresh source-only work was not persisted")
			}
			if err := app.restoreRecoveredEntryError(entries[0]); err != nil {
				t.Fatalf("fresh form blocked recovery of retained source work: %v", err)
			}
			if len(app.editors) != 1 {
				t.Fatal("recovery did not open exactly one document")
			}
			for _, recovered := range app.editors {
				sessionEditor := recovered.(SessionEditor)
				if got, ok := app.sourceDrafts.Get(recovered); !ok || got != draft {
					t.Fatalf("recovery consumed or changed the raw draft: %+v present=%v", got, ok)
				}
				if sessionEditor.CurrentSource() != working {
					t.Fatal("recovery changed fields or invented an identifier in the fresh working form")
				}
				if recovered.GetCurrentPath() != "" || sessionEditor.Session().HasOriginal {
					t.Fatal("fresh source-only recovery acquired an original-file destination")
				}
				if format == ".sab" || format == ".veh" {
					if err := sessionEditor.ApplySourceText(working); err == nil {
						t.Fatal("internal fresh-snapshot restoration weakened user Apply identifier validation")
					}
				}
			}
		})
	}
}

func TestAuditLifecycleAutosaveRetiresCleanAttachedAndDetachedRecords(t *testing.T) {
	for _, detached := range []bool{false, true} {
		name := "attached"
		if detached {
			name = "detached"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			app := newRecoveryTestApp(t, filepath.Join(root, "recovery"), filepath.Join(root, "config"))
			editor := auditLifecycleSaber(t, app)
			if detached {
				app.detachedEditors = map[Editor]string{editor: "Detached Saber"}
			} else {
				tab := container.NewTabItem("Saber", editor.GetContent())
				app.editors[tab] = editor
				app.docTabs.Append(tab)
			}
			app.sourceDrafts.Set(editor, SourceDraft{Text: "discarded draft\n{", EditMode: true})
			app.autosaveRecoveryPass()
			if len(auditLifecycleEntries(t, app)) != 1 {
				t.Fatal("source draft was not snapshotted")
			}
			app.sourceDrafts.Delete(editor)
			app.autosaveRecoveryPass()
			if len(auditLifecycleEntries(t, app)) != 0 {
				t.Fatal("discarding the only source draft left stale recoverable work")
			}

			editor.nameEntry.SetText("undone_form_edit")
			editor.session.flushPending()
			app.autosaveRecoveryPass()
			if len(auditLifecycleEntries(t, app)) != 1 {
				t.Fatal("form edit was not snapshotted")
			}
			if !editor.Undo() || editor.IsDirty() {
				t.Fatal("undo did not return the form to its saved state")
			}
			app.autosaveRecoveryPass()
			if len(auditLifecycleEntries(t, app)) != 0 {
				t.Fatal("undoing to the saved form left a stale recovery record")
			}
		})
	}
}

func TestAuditLifecycleQuitReportsCheckedRecoveryAndRefreshesDrafts(t *testing.T) {
	for _, mode := range []string{"available", "disabled", "write-failure"} {
		t.Run(mode, func(t *testing.T) {
			app := newAuditLifecycleApp(t)
			switch mode {
			case "disabled":
				app.recovery = NewRecoveryStore("")
			case "write-failure":
				blocker := filepath.Join(t.TempDir(), "not-a-directory")
				if err := os.WriteFile(blocker, []byte("blocked"), 0600); err != nil {
					t.Fatal(err)
				}
				app.recovery = NewRecoveryStore(filepath.Join(blocker, "recovery"))
			}
			attached := auditLifecycleSaber(t, app)
			app.createNewFile("attached", attached)
			detached := auditLifecycleSaber(t, app)
			app.detachedEditors = map[Editor]string{detached: "Detached Saber"}
			initial := SourceDraft{Text: "attached initial\n{", ParseErr: "unbalanced braces", EditMode: true}
			other := SourceDraft{Text: "detached initial\n{", ParseErr: "unbalanced braces", EditMode: true}
			app.sourceDrafts.Set(attached, initial)
			app.sourceDrafts.Set(detached, other)
			auditLifecycleQuitItem(t, app).Action()
			overlay := app.mainWindow.Canvas().Overlays().Top()
			message := findLabelByPrefix(overlay, "These tabs have unsaved changes:")
			if message == nil || !strings.Contains(message.Text, "Detached Saber") {
				t.Fatal("Quit did not guard attached and detached source-only work")
			}
			if mode == "available" {
				if !strings.Contains(message.Text, "was saved") || len(auditLifecycleEntries(t, app)) != 2 {
					t.Fatal("Quit claimed recovery without persisting both documents")
				}
			} else if !strings.Contains(message.Text, "could not be guaranteed") || strings.Contains(message.Text, "was saved") {
				t.Fatalf("Quit promised recovery despite unavailable persistence: %q", message.Text)
			}
			test.Tap(saveDialogButton(t, overlay, "No"))
			if got, ok := app.sourceDrafts.Get(attached); !ok || got != initial || len(app.editors) != 1 || len(app.detachedEditors) != 1 {
				t.Fatal("cancelling Quit lost source work or document ownership")
			}
			if mode != "available" {
				return
			}
			auditLifecycleQuitItem(t, app).Action()
			latest := SourceDraft{Text: "newer mirror draft\n{", ParseErr: "unbalanced braces", EditMode: true}
			app.sourceDrafts.Set(attached, latest)
			test.Tap(saveDialogButton(t, app.mainWindow.Canvas().Overlays().Top(), "Yes"))
			foundLatest := false
			for _, entry := range auditLifecycleEntries(t, app) {
				if entry.OriginalPath == attached.GetCurrentPath() {
					foundLatest = entry.HasDraft && entry.Draft == latest.Text && entry.DraftParseErr == latest.ParseErr && entry.DraftEditMode
				}
			}
			if !foundLatest {
				t.Fatal("confirmed Quit relied on a stale snapshot taken before a mirror draft changed")
			}
		})
	}
}

func TestAuditLifecycleQuitRechecksPersistenceBeforeExit(t *testing.T) {
	app := newAuditLifecycleApp(t)
	editor := auditLifecycleSaber(t, app)
	app.createNewFile("quit-recheck", editor)
	initial := SourceDraft{Text: "first draft\n{", EditMode: true}
	app.sourceDrafts.Set(editor, initial)
	auditLifecycleQuitItem(t, app).Action()
	if len(auditLifecycleEntries(t, app)) != 1 {
		t.Fatal("initial quit snapshot was not saved")
	}
	retained := app.recovery.dir + "-retained"
	if err := os.Rename(app.recovery.dir, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.recovery.dir, []byte("storage became unavailable"), 0600); err != nil {
		t.Fatal(err)
	}
	latest := SourceDraft{Text: "latest draft\n{", ParseErr: "unbalanced braces", EditMode: true}
	app.sourceDrafts.Set(editor, latest)
	test.Tap(saveDialogButton(t, app.mainWindow.Canvas().Overlays().Top(), "Yes"))
	overlay := app.mainWindow.Canvas().Overlays().Top()
	if findLabelByPrefix(overlay, "The latest work could not be saved for recovery:") == nil {
		t.Fatal("Quit silently relied on an earlier persistence success after the final snapshot failed")
	}
	test.Tap(saveDialogButton(t, overlay, "No"))
	if got, ok := app.sourceDrafts.Get(editor); !ok || got != latest || len(app.editors) != 1 {
		t.Fatal("cancelling unsafe exit lost the latest document state")
	}
	entries, err := NewRecoveryStore(retained).List()
	if err != nil || len(entries) != 1 || entries[0].Draft != initial.Text {
		t.Fatalf("failed final persistence damaged the earlier recovery snapshot: entries=%+v err=%v", entries, err)
	}
}

func TestAuditLifecycleExplicitDocsIgnoreHoverPreference(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	app := &App{infoPanel: NewInfoPanel(), config: AppConfig{HoverTooltipsDisabled: true}}
	mirror := NewInfoPanel()
	app.infoPanelMirrors = []*InfoPanel{mirror}
	app.showStickyContext("WP_BLASTER", "")
	if app.infoPanel.idChip.Text != "WP_BLASTER" || mirror.idChip.Text != "WP_BLASTER" {
		t.Fatal("disabling hover also disabled explicit documentation clicks")
	}
	app.showHoverContext("WP_SABER", "")
	if app.infoPanel.idChip.Text != "WP_BLASTER" || mirror.idChip.Text != "WP_BLASTER" {
		t.Fatal("disabled hover changed the explicit documentation selection")
	}
}

func TestAuditLifecycleBufferGaugeFollowsActiveDocument(t *testing.T) {
	app := newAuditLifecycleApp(t)
	first := NewMBCHEditor(app)
	first.nameEntry.SetText("first_character")
	first.session.flushPending()
	app.createNewFile("first", first)
	firstTab := app.docTabs.Selected()
	firstLabel := app.bufferGauge.label.Text
	if !app.bufferGauge.GetContent().Visible() || firstLabel != bufferGaugeLabel(CalculateBufferStatus(first.character)) {
		t.Fatal("selecting a character did not publish its buffer assessment")
	}
	second := NewMBCHEditor(app)
	second.nameEntry.SetText("second_character")
	second.descriptionEntry.SetText(strings.Repeat("distinct description ", 12))
	second.session.flushPending()
	app.createNewFile("second", second)
	secondTab := app.docTabs.Selected()
	if app.bufferGauge.label.Text == firstLabel {
		t.Fatal("selecting a different character retained the previous buffer assessment")
	}
	app.docTabs.Select(firstTab)
	if app.bufferGauge.label.Text != firstLabel {
		t.Fatal("switching back did not restore the first character's buffer assessment")
	}
	second.descriptionEntry.SetText(strings.Repeat("inactive edits ", 20))
	second.session.flushPending()
	if app.bufferGauge.label.Text != firstLabel {
		t.Fatal("an inactive character overwrote the active character's buffer gauge")
	}
	app.docTabs.Select(secondTab)
	if app.bufferGauge.label.Text != bufferGaugeLabel(CalculateBufferStatus(second.character)) {
		t.Fatal("selecting the edited character did not refresh its buffer gauge")
	}
	app.createNewFile("Saber", NewSABEditor(app))
	if app.bufferGauge.GetContent().Visible() {
		t.Fatal("a non-character document showed a stale character buffer assessment")
	}
}

func TestAuditLifecycleCloseRetainsOwnershipWhenRecoveryCleanupFails(t *testing.T) {
	app := newAuditLifecycleApp(t)
	editor := auditLifecycleSaber(t, app)
	app.createNewFile("cleanup-failure", editor)
	tab := app.docTabs.Selected()
	draft := SourceDraft{Text: "retained draft\n{", ParseErr: "unbalanced braces", EditMode: true}
	app.sourceDrafts.Set(editor, draft)
	app.captureRecoveryEntry(editor)
	entries := auditLifecycleEntries(t, app)
	if len(entries) != 1 {
		t.Fatal("close fixture did not persist its source draft")
	}
	path := filepath.Join(app.recovery.dir, entries[0].ID+".json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "retained"), []byte("block removal"), 0600); err != nil {
		t.Fatal(err)
	}
	app.closeTab(tab)
	confirmation := app.mainWindow.Canvas().Overlays().Top()
	test.Tap(saveDialogButton(t, confirmation, "Yes"))
	if len(app.docTabs.Items) != 1 || app.docTabs.Items[0] != tab || app.editors[tab] != editor {
		t.Fatal("failed recovery cleanup removed the visible document or its registration")
	}
	if got, ok := app.sourceDrafts.Get(editor); !ok || got != draft {
		t.Fatal("failed close discarded source work that still belongs to the open document")
	}
	feedback := app.mainWindow.Canvas().Overlays().Top()
	if feedback == nil || feedback == confirmation || !feedback.Visible() {
		t.Fatal("failed recovery cleanup did not replace confirmation with visible feedback")
	}
	test.Tap(saveDialogButton(t, feedback, "OK"))
	if app.mainWindow.Canvas().Overlays().Top() != nil {
		t.Fatal("acknowledging recovery failure left an unexpected dialog")
	}
	if len(app.docTabs.Items) != 1 || app.docTabs.Items[0] != tab || app.editors[tab] != editor {
		t.Fatal("acknowledging recovery failure dismissed or unregistered the open document")
	}
	if got, ok := app.sourceDrafts.Get(editor); !ok || got != draft {
		t.Fatal("acknowledging recovery failure discarded the open document's source draft")
	}
}
