package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func assertDocumentCaptionBound(t *testing.T, caption string) {
	t.Helper()
	if !utf8.ValidString(caption) {
		t.Fatalf("caption is not valid UTF-8: %q", caption)
	}
	if width := fyne.MeasureText(caption, theme.TextSize(), fyne.TextStyle{Bold: true}).Width; width > documentTabCaptionWidth {
		t.Fatalf("selected caption width %g exceeds %g: %q", width, documentTabCaptionWidth, caption)
	}
}

func TestDocumentTabCaptionMeasuredUnicodeBound(t *testing.T) {
	useEditorTestApp(t)
	for _, name := range []string{
		strings.Repeat("WWW", 24) + ".sab",
		strings.Repeat("iii", 24) + ".sab",
		"日本語_" + strings.Repeat("銀河の剣é", 16) + ".mbch",
		"éclair_" + strings.Repeat("界🙂", 20) + ".siege",
		"archive_saber.sab  ·  " + strings.Repeat("長い名前", 15) + ".pk3",
	} {
		for _, dirty := range []bool{false, true} {
			full := documentTitleWithDirty(name, dirty)
			caption := documentTabCaption(full)
			assertDocumentCaptionBound(t, caption)
			if !strings.Contains(caption, "…") || !strings.HasSuffix(caption, filepath.Ext(name)) {
				t.Fatalf("long caption lost middle elision or extension: %q", caption)
			}
			if strings.HasPrefix(caption, "* ") != dirty {
				t.Fatalf("caption lost dirty state: %q", caption)
			}
			first, _ := utf8.DecodeRuneInString(name)
			if !strings.HasPrefix(strings.TrimPrefix(caption, "* "), string(first)) {
				t.Fatalf("caption lost filename prefix: %q", caption)
			}
		}
	}
	for _, short := range []string{"Home", "saber.sab", "* saber.sab"} {
		if got := documentTabCaption(short); got != short {
			t.Fatalf("short title changed: %q -> %q", short, got)
		}
	}
	wide := documentTabCaption(strings.Repeat("W", 80) + ".sab")
	narrow := documentTabCaption(strings.Repeat("i", 80) + ".sab")
	if utf8.RuneCountInString(narrow) <= utf8.RuneCountInString(wide) {
		t.Fatal("caption budget did not account for measured glyph widths")
	}
}

func TestDocumentTabFullIdentityAcrossLifecycle(t *testing.T) {
	app := newAuditLifecycleApp(t)
	name := "original_" + strings.Repeat("銀河", 12) + ".sab"
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(multiSabFixture), 0600); err != nil {
		t.Fatal(err)
	}
	tab := app.openFileFromPath(path)
	if tab == nil {
		t.Fatal("document did not open")
	}
	editor := app.editors[tab].(*SABEditor)
	if app.fullTabTitle(tab) != name || !strings.Contains(app.mainWindow.Title(), name) || editor.GetCurrentPath() != path {
		t.Fatal("opening abbreviated the full identity or changed the canonical path")
	}
	assertDocumentCaptionBound(t, tab.Text)
	editor.nameEntry.SetText("title_lifecycle_edit")
	editor.session.flushPending()
	if !editor.IsDirty() || !strings.HasPrefix(tab.Text, "* ") || app.fullTabTitle(tab) != "* "+name {
		t.Fatal("dirty transition lost the full name or visible marker")
	}
	app.requestQuit()
	overlay := app.mainWindow.Canvas().Overlays().Top()
	message := findLabelByPrefix(overlay, "These tabs have unsaved changes:")
	if message == nil || !strings.Contains(message.Text, name) {
		t.Fatal("quit guard did not expose the full document name")
	}
	test.Tap(saveDialogButton(t, overlay, "No"))

	app.popOutCurrentTab()
	window := tornOutWindow(t, app)
	if !strings.Contains(window.Title(), name) || !strings.Contains(app.detachedEditors[editor], name) {
		t.Fatal("detaching exposed only the abbreviated caption")
	}
	if _, retained := app.tabTitles[tab]; retained {
		t.Fatal("detaching retained tab metadata")
	}
	findTooltipButton(t, window.Content(), "Reattach tab").OnTapped()
	if app.docTabs.Selected() != tab || app.fullTabTitle(tab) != "* "+name {
		t.Fatal("reattaching lost full title or dirty state")
	}

	savedName := "saved_" + strings.Repeat("étoile", 10) + ".sab"
	destination := filepath.Join(t.TempDir(), savedName)
	completed := false
	var saveErr error
	app.saveWithReviewCompletion(editor, tab, destination, func(err error) {
		completed, saveErr = true, err
	})
	if overlay := app.mainWindow.Canvas().Overlays().Top(); overlay != nil {
		test.Tap(saveDialogButton(t, overlay, "Save"))
	}
	if !completed || saveErr != nil {
		t.Fatalf("Save As did not complete: completed=%v err=%v", completed, saveErr)
	}
	if editor.IsDirty() || strings.HasPrefix(tab.Text, "* ") || app.fullTabTitle(tab) != savedName {
		t.Fatal("Save As did not publish a clean full destination title")
	}
	if editor.GetCurrentPath() != destination || editor.Session().Path != destination || !strings.Contains(app.mainWindow.Title(), savedName) {
		t.Fatal("Save As changed canonical identity or failed to update the native window title")
	}
	assertDocumentCaptionBound(t, tab.Text)
	app.removeTab(tab)
	if _, retained := app.tabTitles[tab]; retained {
		t.Fatal("closing retained document title metadata")
	}
	if app.docTabs.Selected().Text != "Home" || strings.Contains(app.mainWindow.Title(), savedName) {
		t.Fatal("Home retained document-specific chrome")
	}
}

func TestDocumentTabRecoveryKeepsFullUntitledIdentity(t *testing.T) {
	app := newAuditLifecycleApp(t)
	name := "archive_saber.sab  ·  " + strings.Repeat("銀河", 12) + ".pk3"
	editor := auditLifecycleSaber(t, app)
	editor.SetCurrentPath("")
	app.createNewFile(name, editor)
	tab := app.docTabs.Selected()
	full := app.fullTabTitle(tab)
	if !strings.Contains(full, name) || !strings.Contains(app.mainWindow.Title(), name) {
		t.Fatal("new document lost its full title")
	}
	app.sourceDrafts.Set(editor, SourceDraft{Text: "pending\n{", ParseErr: "unbalanced braces", EditMode: true})
	if err := app.captureRecoveryEntry(editor); err != nil {
		t.Fatal(err)
	}
	entries := auditLifecycleEntries(t, app)
	if len(entries) != 1 || entries[0].Title != full {
		t.Fatal("recovery snapshot abbreviated the unsaved document identity")
	}
	if err := app.restoreRecoveredEntryError(entries[0]); err != nil {
		t.Fatal(err)
	}
	recoveredTab := app.docTabs.Selected()
	recovered := app.editors[recoveredTab].(SessionEditor)
	if !strings.Contains(app.fullTabTitle(recoveredTab), full) || !strings.Contains(app.mainWindow.Title(), full) {
		t.Fatal("restoration lost the full document identity")
	}
	assertDocumentCaptionBound(t, recoveredTab.Text)
	if recovered.Session().RecoveryID != entries[0].ID || app.editors[recoveredTab].GetCurrentPath() != "" {
		t.Fatal("caption handling changed recovery identity or the detached canonical path")
	}
}
