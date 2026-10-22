package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	fyneTest "fyne.io/fyne/v2/test"
)

func newAuditSourcePanel(t *testing.T, app *App, editor Editor) *SourcePanel {
	t.Helper()
	panel := NewSourcePanel(app)
	t.Cleanup(panel.Close)
	panel.SetActiveEditor(editor)
	return panel
}

func newAuditSourceSaber(t *testing.T, app *App) *SABEditor {
	t.Helper()
	editor := NewSABEditor(app)
	t.Cleanup(editor.Session().flushPending)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	return editor
}

func TestAuditSourceCompetingMirrorTransitionsKeepLatestDraft(t *testing.T) {
	for _, transition := range []string{"switch", "rebind", "close"} {
		t.Run(transition, func(t *testing.T) {
			useEditorTestApp(t)
			app := &App{sourceDrafts: NewSourceDraftStore()}
			editor := newAuditSourceSaber(t, app)
			primary := newAuditSourcePanel(t, app, editor)
			mirror := newAuditSourcePanel(t, app, editor)
			var want SourceDraft
			var retained bool

			// Simulate a mirror whose queued push has not reached its
			// surface yet, without republishing its stale text as user input.
			fyne.DoAndWait(func() {
				fyneTest.Tap(primary.editToggle)
				primary.editor.SetText("saber_a\n{\n\tname older_draft")
				older := primary.editor.Text
				primary.editor.SetText("saber_a\n{\n\tname latest_draft")
				mirror.setEditorText(older)
				mirror.validateEdits(older)
				want, retained = app.sourceDrafts.Get(editor)
				switch transition {
				case "switch":
					mirror.SetActiveEditor(nil)
				case "rebind":
					mirror.SetActiveEditor(editor)
				case "close":
					mirror.Close()
				}
			})

			if !retained || want.ParseErr == "" || !want.EditMode {
				t.Fatalf("latest invalid edit was not retained: %+v, present=%v", want, retained)
			}
			if got, ok := app.sourceDrafts.Get(editor); !ok || got != want {
				t.Fatalf("stale mirror %s replaced the latest draft: got %+v, present=%v; want %+v", transition, got, ok, want)
			}
			primary.SetActiveEditor(nil)
			primary.SetActiveEditor(editor)
			if primary.currentText() != want.Text || primary.lastParseErr != want.ParseErr || !primary.inEditMode {
				t.Fatalf("returning to the document lost its latest editable draft: text=%q, error=%q, edit=%v", primary.currentText(), primary.lastParseErr, primary.inEditMode)
			}
		})
	}
}

func TestAuditSourceRevertPreservesNewerMirrorThenRefreshesEditableText(t *testing.T) {
	useEditorTestApp(t)
	app := &App{sourceDrafts: NewSourceDraftStore()}
	editor := newAuditSourceSaber(t, app)
	primary := newAuditSourcePanel(t, app, editor)
	mirror := newAuditSourcePanel(t, app, editor)
	formSource := editor.CurrentSource()
	var want SourceDraft

	fyne.DoAndWait(func() {
		fyneTest.Tap(primary.editToggle)
		primary.editor.SetText("saber_a\n{\n\tname older_draft")
		older := primary.editor.Text
		primary.editor.SetText("saber_a\n{\n\tname latest_draft")
		mirror.setEditorText(older)
		mirror.validateEdits(older)
		want, _ = app.sourceDrafts.Get(editor)
		fyneTest.Tap(mirror.revertBtn)
	})
	if got, ok := app.sourceDrafts.Get(editor); !ok || got != want {
		t.Fatalf("reverting an older mirror consumed newer work: got %+v, present=%v; want %+v", got, ok, want)
	}
	if mirror.currentText() != want.Text || mirror.lastParseErr != want.ParseErr {
		t.Fatalf("stale Revert did not reveal the competing draft: text=%q, error=%q", mirror.currentText(), mirror.lastParseErr)
	}

	// This time the user has seen the latest draft, so Revert may consume
	// it. The Entry must show form bytes without leaving edit mode.
	fyneTest.Tap(mirror.revertBtn)
	if mirror.DraftPendingFor(editor) || primary.DraftPendingFor(editor) {
		t.Fatal("a consumed draft still counts as unapplied work")
	}
	if !mirror.inEditMode || mirror.currentText() != formSource || mirror.editor.Text != formSource {
		t.Fatalf("Revert left discarded text in the edit surface: edit=%v, current=%q, entry=%q", mirror.inEditMode, mirror.currentText(), mirror.editor.Text)
	}
	if mirror.lastParseErr != "" || mirror.applyBtn.Disabled() {
		t.Fatalf("Revert left the discarded draft's validation behind: %q", mirror.lastParseErr)
	}
	primary.Close()
	if app.sourceDrafts.Has(editor) {
		t.Fatal("closing a stale mirror resurrected the reverted draft")
	}

	fyneTest.Type(mirror.editor, "\n// fresh typing")
	draft, ok := app.sourceDrafts.Get(editor)
	if !ok || !strings.Contains(draft.Text, "fresh typing") || strings.Contains(draft.Text, "latest_draft") || strings.Contains(draft.Text, "older_draft") {
		t.Fatalf("typing after Revert revived discarded bytes: %+v, present=%v", draft, ok)
	}
	if editor.CurrentSource() != formSource || editor.IsDirty() {
		t.Fatal("an unapplied raw draft changed the working form")
	}
}

func TestAuditSourceDocumentSwitchRestoresDraftModeAndDiagnostics(t *testing.T) {
	useEditorTestApp(t)
	app := &App{sourceDrafts: NewSourceDraftStore()}
	first := newAuditSourceSaber(t, app)
	second := newAuditSourceSaber(t, app)
	panel := newAuditSourcePanel(t, app, first)

	fyneTest.Tap(panel.editToggle)
	panel.editor.SetText("saber_a\n{\n\tname first_unapplied")
	fyneTest.Tap(panel.editToggle) // retain the draft in highlighted mode
	firstDraft, firstOK := app.sourceDrafts.Get(first)
	panel.SetActiveEditor(second)
	if panel.inEditMode || panel.DraftPendingFor(second) {
		t.Fatal("a fresh document inherited the outgoing draft's mode or work")
	}
	fyneTest.Tap(panel.editToggle)
	panel.editor.SetText("saber_a\n{\n\tname \"second_unapplied")
	secondDraft, secondOK := app.sourceDrafts.Get(second)
	if !firstOK || !secondOK || firstDraft.EditMode || !secondDraft.EditMode || firstDraft.ParseErr == "" || secondDraft.ParseErr == "" {
		t.Fatalf("document-local draft setup failed: first=%+v, second=%+v", firstDraft, secondDraft)
	}

	panel.SetActiveEditor(first)
	if panel.inEditMode || panel.currentText() != firstDraft.Text || panel.lastParseErr != firstDraft.ParseErr {
		t.Fatalf("first draft mode/text/diagnostics were not restored: edit=%v, text=%q, error=%q", panel.inEditMode, panel.currentText(), panel.lastParseErr)
	}
	panel.SetActiveEditor(second)
	if !panel.inEditMode || panel.currentText() != secondDraft.Text || panel.lastParseErr != secondDraft.ParseErr {
		t.Fatalf("second draft mode/text/diagnostics were not restored: edit=%v, text=%q, error=%q", panel.inEditMode, panel.currentText(), panel.lastParseErr)
	}
	panel.DiscardDraft(first)
	if panel.DraftPendingFor(first) || !panel.DraftPendingFor(second) || panel.currentText() != secondDraft.Text {
		t.Fatal("discarding another document's draft disturbed the active draft")
	}
}

func TestAuditSourceAllEditorsKeepCompetingDraftAcrossApplyUndoRedo(t *testing.T) {
	cases := []struct {
		name   string
		ext    string
		create func(*App) SessionEditor
		source string
	}{
		{
			name: "MBCH", ext: ".mbch",
			create: func(app *App) SessionEditor { return NewMBCHEditor(app) },
			source: "ClassInfo\n{\n\tname \"source_base\"\n\tMBClass MB_CLASS_JEDI\n\tmaxhealth 100\n}\n",
		},
		{
			name: "SAB", ext: ".sab",
			create: func(app *App) SessionEditor { return NewSABEditor(app) },
			source: "source_base\n{\n\tname source_base\n\tsaberType SABER_SINGLE\n\tnumblades 1\n}\n",
		},
		{
			name: "VEH", ext: ".veh",
			create: func(app *App) SessionEditor { return NewVEHEditor(app) },
			source: "source_base\n{\n\tname source_base\n\ttype VH_SPEEDER\n\tspeedMax 450\n}\n",
		},
		{
			name: "Siege", ext: ".siege",
			create: func(app *App) SessionEditor { return NewSiegeEditor(app) },
			source: "Teams\n{\n\tteam1 Blue\n\tteam2 Red\n}\nmissionname \"source_base\"\nBlue\n{\n\tUseTeam blue\n}\nRed\n{\n\tUseTeam red\n}\n",
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			useEditorTestApp(t)
			app := &App{sourceDrafts: NewSourceDraftStore()}
			editor := item.create(app)
			t.Cleanup(editor.Session().flushPending)
			path := filepath.Join(t.TempDir(), "source"+item.ext)
			if err := os.WriteFile(path, []byte(item.source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := editor.LoadFile(path); err != nil {
				t.Fatal(err)
			}
			before := editor.CurrentSource()
			applied := strings.ReplaceAll(before, "source_base", "source_applied")
			pending := strings.ReplaceAll(before, "source_base", "source_pending")
			if applied == before || pending == before {
				t.Fatal("source fixture does not expose the field being edited")
			}
			primary := newAuditSourcePanel(t, app, editor)
			mirror := newAuditSourcePanel(t, app, editor)
			var want SourceDraft

			fyne.DoAndWait(func() {
				fyneTest.Tap(primary.editToggle)
				primary.editor.SetText(applied)
				primary.editor.SetText(pending)
				mirror.setEditorText(applied)
				mirror.validateEdits(applied)
				want, _ = app.sourceDrafts.Get(editor)
				fyneTest.Tap(mirror.applyBtn)
			})
			working := editor.CurrentSource()
			if !strings.Contains(working, "source_applied") || strings.Contains(working, "source_pending") || !editor.IsDirty() {
				t.Fatalf("Apply did not update only the reviewed working source: dirty=%v, source=%q", editor.IsDirty(), working)
			}
			if got, ok := app.sourceDrafts.Get(editor); !ok || got != want || mirror.currentText() != want.Text {
				t.Fatalf("Apply consumed the competing newer draft: got %+v, present=%v, displayed=%q; want %+v", got, ok, mirror.currentText(), want)
			}
			if !editor.Undo() || editor.CurrentSource() != before || editor.IsDirty() {
				t.Fatal("Undo did not restore the clean form independently of the raw draft")
			}
			if got, ok := app.sourceDrafts.Get(editor); !ok || got != want {
				t.Fatalf("Undo changed the retained source draft: %+v, present=%v", got, ok)
			}
			if primary.currentText() != want.Text || mirror.currentText() != want.Text {
				t.Fatal("Undo overwrote a retained raw draft in a source mirror")
			}
			if !editor.Redo() || editor.CurrentSource() != working || !editor.IsDirty() {
				t.Fatal("Redo did not restore the applied form independently of the raw draft")
			}
			fyneTest.Tap(mirror.revertBtn)
			if app.sourceDrafts.Has(editor) || mirror.currentText() != working || primary.currentText() != working || !editor.IsDirty() {
				t.Fatal("Revert did not discard only the unapplied draft and refresh both edit surfaces")
			}
			typed := strings.ReplaceAll(working, "source_applied", "source_typed")
			primary.editor.SetText(typed)
			if mirror.currentText() != typed || mirror.lastParseErr != "" || !mirror.inEditMode {
				t.Fatal("raw typing did not push its text, diagnostics and mode to the mirror")
			}
			fyneTest.Tap(primary.applyBtn)
			typedWorking := editor.CurrentSource()
			if app.sourceDrafts.Has(editor) || !strings.Contains(typedWorking, "source_typed") ||
				primary.currentText() != typedWorking || mirror.currentText() != typedWorking {
				t.Fatal("Apply did not consume the reviewed draft and push working source to both panels")
			}
			if !editor.Undo() || primary.currentText() != working || mirror.currentText() != working {
				t.Fatal("Undo did not push restored working source to both panels")
			}
			if !editor.Redo() || primary.currentText() != typedWorking || mirror.currentText() != typedWorking {
				t.Fatal("Redo did not push restored working source to both panels")
			}

			rename := func(name string) {
				switch e := editor.(type) {
				case *MBCHEditor:
					e.nameEntry.SetText(name)
				case *SABEditor:
					e.nameEntry.SetText(name)
				case *VEHEditor:
					e.nameEntry.SetText(name)
				case *SiegeEditor:
					e.missionNameEntry.SetText(name)
				}
			}
			rename("source_form_live")
			formSource := editor.CurrentSource()
			if !strings.Contains(formSource, "source_form_live") ||
				primary.currentText() != formSource || mirror.currentText() != formSource {
				t.Fatal("form mutation did not push live source to both panels")
			}
			mirror.Close()
			rename("source_after_close")
			afterClose := editor.CurrentSource()
			if primary.currentText() != afterClose || mirror.currentText() != formSource {
				t.Fatal("form mutation failed to update the live panel or changed a closed mirror")
			}
			primary.editor.SetText("source_invalid\n{")
			if mirror.currentText() != formSource {
				t.Fatal("raw draft mutation changed a closed mirror")
			}
			fyneTest.Tap(primary.revertBtn)
			if primary.currentText() != afterClose || mirror.currentText() != formSource {
				t.Fatal("Revert failed to update the live panel or changed a closed mirror")
			}
			if editor.GetCurrentPath() != path || editor.Session().OriginalBytes != item.source {
				t.Fatal("source/session transitions changed document identity or its saved baseline")
			}
		})
	}
}
