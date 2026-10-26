package main

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func withSourceEditor(t *testing.T) *SourceCodeEditor {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	return NewSourceCodeEditor()
}

func TestSourceCodeEditorUTF8CRLFDeltaUndoRedo(t *testing.T) {
	editor := withSourceEditor(t)
	changed := 0
	editor.OnChanged = func(string) { changed++ }
	editor.SetText("α\r\nbeta")
	if changed != 0 || editor.CanUndo() {
		t.Fatal("external SetText must be silent and start a clean local history")
	}

	editor.SelectRange(0, len("α"))
	editor.ReplaceRange(0, len("α"), "Ω")
	if got, want := editor.Text(), "Ω\r\nbeta"; got != want {
		t.Fatalf("UTF-8 replacement = %q, want %q", got, want)
	}
	if changed != 1 || !editor.CanUndo() || editor.CanRedo() {
		t.Fatalf("unexpected edit state: changed=%d undo=%v redo=%v", changed, editor.CanUndo(), editor.CanRedo())
	}

	editor.SetText(editor.Text())
	if !editor.CanUndo() {
		t.Fatal("identical SetText discarded document history")
	}
	if !editor.Undo() || editor.Text() != "α\r\nbeta" || !editor.CanRedo() {
		t.Fatalf("undo did not restore exact CRLF text: %q", editor.Text())
	}
	if !editor.Redo() || editor.Text() != "Ω\r\nbeta" {
		t.Fatalf("redo did not restore replacement: %q", editor.Text())
	}

	editor.SetText("external")
	if editor.CanUndo() || changed != 3 {
		t.Fatalf("external synchronization mutated callbacks/history: changed=%d undo=%v", changed, editor.CanUndo())
	}
}

func TestSourceCodeEditorAutoIndentPreservesCRLF(t *testing.T) {
	editor := withSourceEditor(t)
	if lines := sourceTextLines("line\r\n"); len(lines) != 2 || lines[1].Start != len("line\r\n") {
		t.Fatalf("trailing CRLF did not retain its empty line: %#v", lines)
	}
	editor.SetText("block {\r\n\tvalue\r\n}")
	caret := len("block {")
	editor.SelectRange(caret, caret)
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if got, want := editor.Text(), "block {\r\n\t\r\n\tvalue\r\n}"; got != want {
		t.Fatalf("autoindent = %q, want %q", got, want)
	}
	if !editor.Undo() || editor.Text() != "block {\r\n\tvalue\r\n}" {
		t.Fatalf("autoindent was not one undo transaction: %q", editor.Text())
	}
}

func TestSourceCodeEditorIndentCommentAndAtomicReplaceAll(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("one\r\n  two\r\nONE")
	editor.SelectRange(0, len(editor.Text()))
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if got, want := editor.Text(), "\tone\r\n\t  two\r\n\tONE"; got != want {
		t.Fatalf("multiline indent = %q, want %q", got, want)
	}
	if !editor.Undo() {
		t.Fatal("multiline indent did not create an undo transaction")
	}

	editor.SelectRange(0, len(editor.Text()))
	editor.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: fyne.KeyModifierControl})
	if got, want := editor.Text(), "//one\r\n  //two\r\n//ONE"; got != want {
		t.Fatalf("comment toggle = %q, want %q", got, want)
	}
	editor.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeySlash, Modifier: fyne.KeyModifierControl})
	if got, want := editor.Text(), "one\r\n  two\r\nONE"; got != want {
		t.Fatalf("comment untoggle = %q, want %q", got, want)
	}

	if count := editor.ReplaceAll("one", "item", false); count != 2 {
		t.Fatalf("ReplaceAll count = %d, want 2", count)
	}
	if got, want := editor.Text(), "item\r\n  two\r\nitem"; got != want {
		t.Fatalf("ReplaceAll = %q, want %q", got, want)
	}
	if !editor.Undo() || editor.Text() != "one\r\n  two\r\nONE" {
		t.Fatalf("ReplaceAll was not atomic: %q", editor.Text())
	}
}

func TestSourceCodeEditorSearchCompletionAndShortcutRouting(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("alpha βeta alpha")
	if !editor.FindNext("βeta", true, false) {
		t.Fatal("FindNext missed UTF-8 text")
	}
	start, end := editor.Selection()
	if got := editor.Text()[start:end]; got != "βeta" {
		t.Fatalf("FindNext selected %q", got)
	}

	editor.OnRequestCompletions = func(text string, caret int) []SourceCompletion {
		return []SourceCompletion{
			{Label: "first", InsertText: "FIRST", Replace: SourceRange{Start: start, End: end}},
			{Label: "second", InsertText: "SECOND", Replace: SourceRange{Start: start, End: end}},
		}
	}
	editor.RequestCompletion()
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if got, want := editor.Text(), "alpha SECOND alpha"; got != want {
		t.Fatalf("keyboard completion = %q, want %q", got, want)
	}

	saveAs := false
	editor.OnSaveRequested = func(as bool) { saveAs = as }
	editor.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierSuper | fyne.KeyModifierShift})
	if !saveAs {
		t.Fatal("Cmd+Shift+S did not request Save As")
	}

	fallbacks := 0
	editor.SetText("clean")
	editor.OnUndoFallback = func() bool { fallbacks++; return true }
	if !editor.Undo() || fallbacks != 1 {
		t.Fatalf("empty local undo did not route exactly once to fallback: %d", fallbacks)
	}

	unhandled := 0
	editor.OnUnhandledShortcut = func(fyne.Shortcut) { unhandled++ }
	editor.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyN, Modifier: fyne.KeyModifierSuper})
	if unhandled != 1 {
		t.Fatalf("unhandled application shortcut was not forwarded exactly once: %d", unhandled)
	}
	editor.OnSearchRequested = func(bool) {}
	editor.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF, Modifier: fyne.KeyModifierSuper})
	if unhandled != 1 {
		t.Fatal("core-handled search shortcut was also forwarded")
	}
}

func TestSourceCodeEditorPreferredColumnSurvivesShortLinesAndPages(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("abcdefgh\nx\nabcdefgh")
	editor.SelectRange(8, 8)
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if got, want := editor.CursorOffset(), len("abcdefgh\nx\nabcdefgh"); got != want {
		t.Fatalf("vertical navigation caret = %d, want restored column at %d", got, want)
	}

	editor.SelectRange(8, 8)
	editor.Resize(fyne.NewSize(480, 180))
	test.LaidOutObjects(editor)
	cellHeight := editor.codeGrid.PositionForCursorLocation(1, 0).Y
	if cellHeight <= 0 {
		t.Fatalf("TextGrid returned invalid cell height %v", cellHeight)
	}
	editor.codeScroll.Resize(fyne.NewSize(320, cellHeight*2.5))
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyPageDown})
	if got, want := editor.CursorOffset(), len("abcdefgh\nx\nabcdefgh"); got != want {
		t.Fatalf("page navigation caret = %d, want restored column at %d", got, want)
	}

	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	if got, want := editor.CursorOffset(), 7; got != want {
		t.Fatalf("horizontal move did not reset preferred column: caret=%d want=%d", got, want)
	}
}

func TestSourceCodeEditorPublicPointerDispatchAndWideUnicodeGeometry(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("A界🙂B")
	window := test.NewWindow(editor)
	window.SetPadded(false)
	window.Resize(fyne.NewSize(480, 180))
	test.LaidOutObjects(editor)

	cellWidth := editor.codeGrid.PositionForCursorLocation(0, 1).X
	cellHeight := editor.codeGrid.PositionForCursorLocation(1, 0).Y
	if cellWidth <= 0 || cellHeight <= 0 {
		t.Fatalf("TextGrid returned invalid cell geometry %vx%v", cellWidth, cellHeight)
	}
	point := editor.codeScroll.Position().Add(editor.codeGrid.Position()).Add(fyne.NewPos(cellWidth*3.25, cellHeight*.5))
	test.TapCanvas(window.Canvas(), point)
	if got, want := editor.CursorOffset(), len("A界"); got != want {
		t.Fatalf("public tap after CJK rune mapped to byte %d, want %d", got, want)
	}

	editor.SetText("abcdef")
	test.LaidOutObjects(editor)
	start := editor.codeScroll.Position().Add(editor.codeGrid.Position()).Add(fyne.NewPos(cellWidth*1.25, cellHeight*.5))
	// GLFW delivers MouseDown before a drag; Fyne's test.Drag sends only drag events.
	editor.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: start}, Button: desktop.MouseButtonPrimary})
	test.Drag(window.Canvas(), start, cellWidth*4, 0)
	if gotStart, gotEnd := editor.Selection(); gotStart != 1 || gotEnd != 5 {
		t.Fatalf("public drag through nested scroll selected [%d,%d), want [1,5)", gotStart, gotEnd)
	}
}

func TestSourceCodeEditorShiftClickIsNotCollapsedByTap(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("abcdef")
	editor.Resize(fyne.NewSize(480, 180))
	test.LaidOutObjects(editor)
	editor.SelectRange(1, 1)

	cellWidth := editor.codeGrid.PositionForCursorLocation(0, 1).X
	cellHeight := editor.codeGrid.PositionForCursorLocation(1, 0).Y
	point := editor.codeScroll.Position().Add(editor.codeGrid.Position()).Add(fyne.NewPos(cellWidth*4.25, cellHeight*.5))
	// GLFW order for one click: MouseDown on press, MouseUp and Tapped on release.
	editor.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: point},
		Button:     desktop.MouseButtonPrimary,
		Modifier:   fyne.KeyModifierShift,
	})
	editor.MouseUp(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: point}, Button: desktop.MouseButtonPrimary})
	editor.Tapped(&fyne.PointEvent{Position: point})
	if gotStart, gotEnd := editor.Selection(); gotStart != 1 || gotEnd != 4 {
		t.Fatalf("Shift-click selection collapsed to [%d,%d), want [1,4)", gotStart, gotEnd)
	}
}

func TestSourceCodeEditorCompletionSurvivesToolbarFocusLossAndAcceptsPointerSelection(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("value old")
	window := test.NewWindow(editor)
	window.SetPadded(false)
	window.Resize(fyne.NewSize(640, 240))
	test.LaidOutObjects(editor)
	editor.SelectRange(len("value old"), len("value old"))
	editor.OnRequestCompletions = func(text string, caret int) []SourceCompletion {
		start := strings.LastIndex(text, " ") + 1
		return []SourceCompletion{
			{Label: "first", InsertText: "first", Replace: SourceRange{Start: start, End: caret}},
			{Label: "second", InsertText: "second", Replace: SourceRange{Start: start, End: caret}},
		}
	}

	// A toolbar Button clears canvas focus before invoking its action.
	window.Canvas().Unfocus()
	editor.RequestCompletion()
	focused, ok := window.Canvas().Focused().(*SourceCodeEditor)
	if !ok || focused != editor {
		t.Fatalf("toolbar completion left focus on %T", window.Canvas().Focused())
	}
	focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if editor.completion.Visible() || editor.Text() != "value old" {
		t.Fatalf("focused Escape must dismiss without editing: visible=%v text=%q", editor.completion.Visible(), editor.Text())
	}

	editor.RequestCompletion()
	// GLFW keeps focus when the focused editor contains the clicked row; the
	// test driver unfocuses unconditionally, so invoke the row's own handler.
	// widget.Button.Tapped clears canvas focus here and dismissed the list.
	editor.completionRows[1].Tapped(&fyne.PointEvent{})
	if got, want := editor.Text(), "value second"; got != want {
		t.Fatalf("pointer completion = %q, want %q", got, want)
	}
	if editor.completion.Visible() || window.Canvas().Focused() != editor {
		t.Fatalf("accepted completion left visible=%v focus=%T", editor.completion.Visible(), window.Canvas().Focused())
	}
}

func TestSourceCodeEditorReadOnlyStateHasNoCompletionAction(t *testing.T) {
	editor := withSourceEditor(t)
	if editor.IsReadOnly() {
		t.Fatal("new editor unexpectedly read-only")
	}
	requests := 0
	editor.OnRequestCompletions = func(string, int) []SourceCompletion {
		requests++
		return []SourceCompletion{{Label: "value", InsertText: "value"}}
	}
	editor.SetReadOnly(true)
	editor.RequestCompletion()
	if !editor.IsReadOnly() || requests != 0 || editor.completion.Visible() {
		t.Fatalf("read-only completion state: readonly=%v requests=%d visible=%v", editor.IsReadOnly(), requests, editor.completion.Visible())
	}
}

func TestSourceCodeEditorAnalysisDoesNotChangeText(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("{\nvalue\n}")
	before := editor.Text()
	editor.SetAnalysis(SourceAnalysis{
		Syntax:      []SourceSyntaxRun{{Range: SourceRange{Start: 2, End: 7}, Class: SourceSyntaxConstant}},
		Diagnostics: []SourceDiagnostic{{Range: SourceRange{Start: 2, End: 7}, Line: 2, Column: 1, Severity: SourceWarning, Message: "advisory"}},
		Brackets:    []SourceBracketPair{{Open: 0, Close: 8}},
	})
	if editor.Text() != before || editor.CanUndo() {
		t.Fatal("presentation analysis mutated source text or history")
	}
	if got := editor.gutterGrid.RowText(1); !strings.HasPrefix(got, "▲") {
		t.Fatalf("warning marker missing from diagnostic line: %q", got)
	}
}

func TestSourceCodeEditorResizeKeepsWideCaretAndSelectionVisible(t *testing.T) {
	editor := withSourceEditor(t)
	original := strings.Repeat("line\n", 120) + strings.Repeat("identifier_", 30) + "界🙂\nlast"
	editor.SetText(original)
	editor.ReplaceRange(0, 0, "// draft\n")
	text := editor.Text()
	caret := strings.Index(text, "界")
	editor.SelectRange(caret-10, caret)
	changes := 0
	editor.OnChanged = func(string) { changes++ }

	for _, width := range []float32{760, 260, 400, 260, 760} {
		editor.Resize(fyne.NewSize(width, 180))
		test.LaidOutObjects(editor)
		position := editor.codeGrid.PositionForCursorLocation(121, 330).Add(editor.codeGrid.Position())
		position = position.Subtract(editor.codeScroll.Offset)
		cell := editor.codeGrid.PositionForCursorLocation(1, 1)
		view := editor.codeScroll.Size()
		if position.X < 0 || position.Y < 0 || position.X+2*cell.X > view.Width+0.1 || position.Y+cell.Y > view.Height+0.1 {
			t.Fatalf("wide caret is clipped at width %.0f: position=%v cell=%v viewport=%v", width, position, cell, view)
		}
		if editor.gutterScroll.Offset.Y != editor.codeScroll.Offset.Y || editor.gutterGrid.Position().Y != editor.codeGrid.Position().Y {
			t.Fatalf("gutter drifted after resize: gutter=%v code=%v", editor.gutterScroll.Offset, editor.codeScroll.Offset)
		}
		if start, end := editor.Selection(); start != caret-10 || end != caret {
			t.Fatalf("resize changed selection to [%d,%d)", start, end)
		}
	}
	if changes != 0 || editor.Text() != text {
		t.Fatal("resizing modified the document")
	}
	editor.codeScroll.ScrollToOffset(fyne.Position{})
	editor.Refresh()
	if editor.codeScroll.Offset != (fyne.Position{}) {
		t.Fatal("ordinary refresh undid deliberate code scrolling")
	}
	if !editor.Undo() || editor.Text() != original {
		t.Fatal("resize damaged the pending edit's undo history")
	}
}

func TestSourceCodeEditorResizeKeepsCRLFBoundaryVisible(t *testing.T) {
	editor := withSourceEditor(t)
	firstLine := strings.Repeat("identifier_", 30)
	editor.SetText(firstLine + "\r\nnext")
	for _, boundary := range []struct {
		offset int
		row    int
		column int
	}{
		{len(firstLine), 0, len(firstLine)},
		{len(firstLine) + 2, 1, 0},
	} {
		editor.SelectRange(0, boundary.offset)
		for _, width := range []float32{760, 260, 400} {
			editor.Resize(fyne.NewSize(width, 180))
			test.LaidOutObjects(editor)
			position := editor.codeGrid.PositionForCursorLocation(boundary.row, boundary.column).Add(editor.codeGrid.Position())
			position = position.Subtract(editor.codeScroll.Offset)
			cell := editor.codeGrid.PositionForCursorLocation(1, 1)
			view := editor.codeScroll.Size()
			if position.X < 0 || position.Y < 0 || position.X+cell.X > view.Width+0.1 || position.Y+cell.Y > view.Height+0.1 {
				t.Fatalf("CRLF boundary is outside resized viewport: position=%v offset=%v viewport=%v", position, editor.codeScroll.Offset, view)
			}
			if start, end := editor.Selection(); start != 0 || end != boundary.offset {
				t.Fatalf("resize changed CRLF selection to [%d,%d)", start, end)
			}
		}
	}
}

type sourceViewportLargeTextTheme struct {
	fyne.Theme
}

func (t sourceViewportLargeTextTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 32
	}
	return t.Theme.Size(name)
}

func TestSourceCodeEditorGutterFitsMeasuredLineNumbers(t *testing.T) {
	editor := withSourceEditor(t)
	fyne.CurrentApp().Settings().SetTheme(sourceViewportLargeTextTheme{Theme: &FoundryTheme{}})
	editor.SetText(strings.Repeat("line\n", 100))
	editor.Resize(fyne.NewSize(260, 240))
	test.LaidOutObjects(editor)
	if editor.gutterScroll.Size().Width < editor.gutterScroll.Content.MinSize().Width {
		t.Fatalf("measured line numbers exceed their gutter: needed=%v allocated=%v", editor.gutterScroll.Content.MinSize(), editor.gutterScroll.Size())
	}
	if editor.codeScroll.Position().X < editor.gutterScroll.Size().Width {
		t.Fatal("line numbers overlap the source viewport")
	}
}

func TestSourceCodeEditorNarrowCompletionScrollsToSelectionAndKeepsFullIdentifier(t *testing.T) {
	editor := withSourceEditor(t)
	editor.SetText("old")
	window := test.NewWindow(editor)
	t.Cleanup(window.Close)
	window.SetPadded(false)
	window.Resize(fyne.NewSize(260, 180))
	editor.SelectRange(3, 3)
	items := make([]SourceCompletion, sourceCompletionLimit)
	for index := range items {
		identifier := strings.Repeat("models/界/", 18) + string(rune('A'+index))
		items[index] = SourceCompletion{Label: identifier, InsertText: identifier, Replace: SourceRange{Start: 0, End: 3}}
	}
	editor.OnRequestCompletions = func(string, int) []SourceCompletion { return items }
	editor.RequestCompletion()
	test.LaidOutObjects(editor)
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})

	selected := editor.completionRows[len(items)-1]
	y := selected.Position().Y - editor.completionScroll.Offset.Y
	if y < 0 || y+selected.Size().Height > editor.completionScroll.Size().Height+0.1 {
		t.Fatalf("keyboard-selected completion is outside popup: row=%v offset=%v viewport=%v", selected.Size(), editor.completionScroll.Offset, editor.completionScroll.Size())
	}
	if selected.text.Text != items[len(items)-1].Label {
		t.Fatal("long completion identifier was irreversibly truncated")
	}
	if end := editor.completion.Position().X + editor.completion.Size().Width; end > editor.Size().Width {
		t.Fatal("completion popup exceeds the code viewport")
	}
	editor.completionScroll.ScrollToOffset(fyne.NewPos(200, editor.completionScroll.Offset.Y))
	if editor.completionScroll.Offset.X == 0 {
		t.Fatal("long completion fixture did not exercise horizontal scrolling")
	}
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	editor.RequestCompletion()
	test.LaidOutObjects(editor)
	if editor.completionScroll.Offset.X != 0 {
		t.Fatal("fresh completion request hides identifier beginnings at a stale horizontal offset")
	}
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyUp})
	editor.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if editor.Text() != items[len(items)-1].InsertText || !editor.Undo() || editor.Text() != "old" {
		t.Fatal("narrow completion changed insertion or undo semantics")
	}
}

func TestSourcePaletteOverridesAreStrictAndIndependent(t *testing.T) {
	base, ok := SourcePaletteByName("Foundry Dark")
	if !ok {
		t.Fatal("Foundry Dark palette missing")
	}
	updated, err := SourcePaletteWithOverrides(base, map[string]string{
		"background": "#010203",
		"comment":    "#10203040",
		"caret":      "#fefefe",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Background != (color.NRGBA{R: 1, G: 2, B: 3, A: 0xff}) || updated.Colors[SourceSyntaxComment].A != 0x40 {
		t.Fatalf("palette overrides were not applied: %#v", updated)
	}
	if base.Background == updated.Background {
		t.Fatal("palette override mutated the base palette")
	}
	if _, err := SourcePaletteWithOverrides(base, map[string]string{"bogus": "#ffffff"}); err == nil {
		t.Fatal("unknown override key was accepted")
	}
	if _, err := SourcePaletteWithOverrides(base, map[string]string{"text": "red"}); err == nil {
		t.Fatal("invalid custom color was accepted")
	}
	if palettes := SourcePalettes(); len(palettes) < 3 {
		t.Fatalf("SourcePalettes returned %d presets, want at least 3", len(palettes))
	}
}
