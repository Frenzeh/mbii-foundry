package main

import (
	"image/color"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

const sourceCompletionLimit = 12

// SourceCodeEditor is an editable, syntax-coloured TextGrid code surface. The
// text buffer is authoritative; the TextGrid is rebuilt only as its native
// styled presentation.
type SourceCodeEditor struct {
	widget.BaseWidget

	OnChanged            func(string)
	OnCursorChanged      func(int)
	OnRequestCompletions func(string, int) []SourceCompletion
	OnSearchRequested    func(bool)
	OnAssetRequested     func()
	OnSaveRequested      func(bool)
	OnUndoFallback       func() bool
	OnRedoFallback       func() bool
	OnUnhandledShortcut  func(fyne.Shortcut)

	buffer          sourceCodeBuffer
	analysis        SourceAnalysis
	palette         SourcePalette
	readOnly        bool
	focused         bool
	shift           bool
	preferredColumn int

	codeGrid     *widget.TextGrid
	gutterGrid   *widget.TextGrid
	codeScroll   *container.Scroll
	gutterScroll *container.Scroll
	background   *canvas.Rectangle
	completionBG *canvas.Rectangle
	completion   *fyne.Container
	completionUI *fyne.Container

	completionItems []SourceCompletion
	completionRows  []*sourceCompletionRow
	completionIndex int
	mouseSelecting  bool
	// GLFW delivers MouseDown on press and Tapped on release for the same
	// click; the press already placed or extended the caret.
	suppressTap  bool
	dragPosition fyne.Position
	styleCache   map[sourceCellStyleKey]*widget.CustomTextGridStyle
}

// sourceCompletionRow is deliberately not a widget.Button: Button.Tapped clears
// canvas focus before its callback, which would dismiss the suggestion list via
// FocusLost before the selection could be accepted.
type sourceCompletionRow struct {
	widget.BaseWidget
	background *canvas.Rectangle
	text       *canvas.Text
	onTapped   func()
}

var _ fyne.Tappable = (*sourceCompletionRow)(nil)

func newSourceCompletionRow(onTapped func()) *sourceCompletionRow {
	row := &sourceCompletionRow{
		background: canvas.NewRectangle(color.Transparent),
		text:       canvas.NewText("", color.White),
		onTapped:   onTapped,
	}
	row.ExtendBaseWidget(row)
	return row
}

func (r *sourceCompletionRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(r.background, container.NewPadded(r.text)))
}

func (r *sourceCompletionRow) Tapped(*fyne.PointEvent) {
	r.onTapped()
}

func (r *sourceCompletionRow) setState(label string, selected bool, palette SourcePalette) {
	r.text.Text = label
	r.text.Color = palette.Colors[SourceSyntaxText]
	r.text.TextStyle.Bold = selected
	if selected {
		r.background.FillColor = palette.Selection
	} else {
		r.background.FillColor = color.Transparent
	}
	r.Refresh()
}

type sourceCellStyleKey struct {
	Class      SourceSyntaxClass
	Background uint8
	Underline  bool
}

var (
	_ fyne.Widget        = (*SourceCodeEditor)(nil)
	_ fyne.Focusable     = (*SourceCodeEditor)(nil)
	_ fyne.Shortcutable  = (*SourceCodeEditor)(nil)
	_ fyne.Tabbable      = (*SourceCodeEditor)(nil)
	_ fyne.Draggable     = (*SourceCodeEditor)(nil)
	_ fyne.Tappable      = (*SourceCodeEditor)(nil)
	_ desktop.Keyable    = (*SourceCodeEditor)(nil)
	_ desktop.Mouseable  = (*SourceCodeEditor)(nil)
	_ desktop.Cursorable = (*SourceCodeEditor)(nil)
)

func NewSourceCodeEditor() *SourceCodeEditor {
	palette, _ := SourcePaletteByName("Foundry Dark")
	editor := &SourceCodeEditor{palette: palette, preferredColumn: -1, styleCache: make(map[sourceCellStyleKey]*widget.CustomTextGridStyle)}
	editor.codeGrid = widget.NewTextGrid()
	editor.codeGrid.TabWidth = 4
	editor.gutterGrid = widget.NewTextGrid()
	editor.codeScroll = container.NewScroll(editor.codeGrid)
	editor.gutterScroll = container.NewVScroll(editor.gutterGrid)
	editor.codeScroll.OnScrolled = func(offset fyne.Position) {
		editor.gutterScroll.ScrollToOffset(fyne.NewPos(0, offset.Y))
	}
	editor.gutterScroll.OnScrolled = func(offset fyne.Position) {
		editor.codeScroll.ScrollToOffset(fyne.NewPos(editor.codeScroll.Offset.X, offset.Y))
	}
	editor.background = canvas.NewRectangle(palette.Background)
	editor.completionBG = canvas.NewRectangle(palette.Background)
	editor.completionBG.StrokeColor = palette.LineNumber
	editor.completionBG.StrokeWidth = 1
	editor.completionUI = container.NewVBox()
	editor.completion = container.NewStack(editor.completionBG, container.NewPadded(editor.completionUI))
	editor.completion.Hide()
	editor.ExtendBaseWidget(editor)
	editor.rebuildTextSurfaces()
	editor.refreshStyles()
	return editor
}

func (e *SourceCodeEditor) CreateRenderer() fyne.WidgetRenderer {
	return &sourceCodeEditorRenderer{
		editor: e,
		objects: []fyne.CanvasObject{
			e.background,
			e.gutterScroll,
			e.codeScroll,
			e.completion,
		},
	}
}

func (e *SourceCodeEditor) Text() string {
	return e.buffer.text
}

// SetText silently synchronizes external state. A real change starts a new
// local history; identical text deliberately preserves selection and history.
func (e *SourceCodeEditor) SetText(text string) {
	if !e.buffer.reset(text) {
		return
	}
	e.analysis = SourceAnalysis{}
	e.preferredColumn = -1
	e.dismissCompletion()
	e.rebuildTextSurfaces()
	e.refreshStyles()
}

func (e *SourceCodeEditor) SetReadOnly(readOnly bool) {
	if e.readOnly == readOnly {
		return
	}
	e.readOnly = readOnly
	if readOnly {
		e.dismissCompletion()
	}
	e.refreshStyles()
}

func (e *SourceCodeEditor) IsReadOnly() bool {
	return e.readOnly
}

func (e *SourceCodeEditor) CursorOffset() int {
	return e.buffer.caret
}

func (e *SourceCodeEditor) Selection() (int, int) {
	return e.buffer.selection()
}

func (e *SourceCodeEditor) SelectRange(start, end int) {
	old := e.buffer.caret
	e.buffer.selectRange(start, end)
	e.preferredColumn = -1
	e.dismissCompletion()
	e.afterCursorMove(old)
}

func (e *SourceCodeEditor) ReplaceRange(start, end int, text string) {
	if e.readOnly {
		return
	}
	old := e.buffer.caret
	e.preferredColumn = -1
	if !e.buffer.replace(start, end, text) {
		e.afterCursorMove(old)
		return
	}
	e.dismissCompletion()
	e.afterUserEdit(old)
}

func (e *SourceCodeEditor) Undo() bool {
	if !e.readOnly && e.buffer.undoEdit() {
		e.dismissCompletion()
		e.afterUserEdit(-1)
		return true
	}
	return e.OnUndoFallback != nil && e.OnUndoFallback()
}

func (e *SourceCodeEditor) Redo() bool {
	if !e.readOnly && e.buffer.redoEdit() {
		e.dismissCompletion()
		e.afterUserEdit(-1)
		return true
	}
	return e.OnRedoFallback != nil && e.OnRedoFallback()
}

func (e *SourceCodeEditor) CanUndo() bool {
	return len(e.buffer.undo) != 0
}

func (e *SourceCodeEditor) CanRedo() bool {
	return len(e.buffer.redo) != 0
}

func (e *SourceCodeEditor) SetAnalysis(analysis SourceAnalysis) {
	e.analysis.Syntax = append(e.analysis.Syntax[:0], analysis.Syntax...)
	sort.SliceStable(e.analysis.Syntax, func(i, j int) bool {
		return e.analysis.Syntax[i].Range.Start < e.analysis.Syntax[j].Range.Start
	})
	e.analysis.Diagnostics = append(e.analysis.Diagnostics[:0], analysis.Diagnostics...)
	e.analysis.Brackets = append(e.analysis.Brackets[:0], analysis.Brackets...)
	e.rebuildGutter(sourceTextLines(e.buffer.text))
	e.refreshStyles()
}

func (e *SourceCodeEditor) SetPalette(palette SourcePalette) {
	e.palette = palette
	e.styleCache = make(map[sourceCellStyleKey]*widget.CustomTextGridStyle)
	e.background.FillColor = palette.Background
	e.completionBG.FillColor = palette.Background
	e.completionBG.StrokeColor = palette.LineNumber
	e.updateCompletionLabels()
	e.refreshStyles()
}

func (e *SourceCodeEditor) RequestCompletion() {
	if e.readOnly || e.OnRequestCompletions == nil {
		e.dismissCompletion()
		return
	}
	items := e.OnRequestCompletions(e.buffer.text, e.buffer.caret)
	valid := make([]SourceCompletion, 0, len(items))
	for _, item := range items {
		start, end := normalizedSourceRange(e.buffer.text, item.Replace.Start, item.Replace.End)
		if item.Label == "" || start != item.Replace.Start || end != item.Replace.End {
			continue
		}
		item.Replace = SourceRange{Start: start, End: end}
		valid = append(valid, item)
		if len(valid) == sourceCompletionLimit {
			break
		}
	}
	e.completionItems = valid
	e.completionIndex = 0
	e.rebuildCompletionUI()
	if len(valid) != 0 {
		// Toolbar requests arrive after a Button cleared canvas focus; keyboard
		// navigation and Escape must still reach the editor.
		e.focus()
	}
}

func (e *SourceCodeEditor) FindNext(query string, matchCase, backwards bool) bool {
	if query == "" {
		return false
	}
	start, end := e.buffer.selection()
	if start == end {
		start, end = e.buffer.caret, e.buffer.caret
	}
	from := end
	if backwards {
		from = start
	}
	matchStart, matchEnd, ok := findSourceMatch(e.buffer.text, query, from, matchCase, backwards)
	if !ok {
		return false
	}
	e.SelectRange(matchStart, matchEnd)
	e.ensureCaretVisible()
	return true
}

func (e *SourceCodeEditor) ReplaceNext(query, replacement string, matchCase bool) bool {
	if e.readOnly || query == "" {
		return false
	}
	start, end := e.buffer.selection()
	if start == end || !sourceTextEqual(e.buffer.text[start:end], query, matchCase) {
		if !e.FindNext(query, matchCase, false) {
			return false
		}
		start, end = e.buffer.selection()
	}
	e.ReplaceRange(start, end, replacement)
	return true
}

func (e *SourceCodeEditor) ReplaceAll(query, replacement string, matchCase bool) int {
	if e.readOnly || query == "" {
		return 0
	}
	matches := allSourceMatches(e.buffer.text, query, matchCase)
	if len(matches) == 0 {
		return 0
	}
	deltas := make([]sourceEditDelta, 0, len(matches))
	for _, match := range matches {
		deltas = append(deltas, sourceEditDelta{Start: match.Start, Before: e.buffer.text[match.Start:match.End], After: replacement})
	}
	sortedSourceDeltas(deltas)
	caretAfter := matches[len(matches)-1].Start + len(replacement)
	for _, match := range matches[:len(matches)-1] {
		caretAfter += len(replacement) - (match.End - match.Start)
	}
	tx := sourceEditTransaction{
		Deltas:       deltas,
		CaretBefore:  e.buffer.caret,
		AnchorBefore: e.buffer.anchor,
		CaretAfter:   caretAfter,
		AnchorAfter:  caretAfter,
	}
	e.buffer.applyNew(tx)
	e.dismissCompletion()
	e.afterUserEdit(-1)
	return len(matches)
}

func (e *SourceCodeEditor) FocusGained() {
	e.focused = true
	e.refreshStyles()
}

func (e *SourceCodeEditor) FocusLost() {
	e.focused = false
	e.shift = false
	e.mouseSelecting = false
	e.preferredColumn = -1
	e.dismissCompletion()
	e.refreshStyles()
}

func (e *SourceCodeEditor) AcceptsTab() bool {
	return true
}

func (e *SourceCodeEditor) Cursor() desktop.Cursor {
	return desktop.TextCursor
}

func (e *SourceCodeEditor) KeyDown(key *fyne.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shift = true
	}
}

func (e *SourceCodeEditor) KeyUp(key *fyne.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shift = false
	}
}

func (e *SourceCodeEditor) TypedRune(r rune) {
	if e.readOnly || unicode.IsControl(r) {
		return
	}
	start, end := e.buffer.selection()
	e.ReplaceRange(start, end, string(r))
}

func (e *SourceCodeEditor) TypedKey(key *fyne.KeyEvent) {
	if e.completion.Visible() {
		switch key.Name {
		case fyne.KeyUp:
			e.moveCompletion(-1)
			return
		case fyne.KeyDown:
			e.moveCompletion(1)
			return
		case fyne.KeyReturn, fyne.KeyEnter, fyne.KeyTab:
			e.acceptCompletion(e.completionIndex)
			return
		case fyne.KeyEscape:
			e.dismissCompletion()
			return
		}
	}
	if e.readOnly && key.Name != fyne.KeyLeft && key.Name != fyne.KeyRight && key.Name != fyne.KeyUp && key.Name != fyne.KeyDown && key.Name != fyne.KeyHome && key.Name != fyne.KeyEnd && key.Name != fyne.KeyPageUp && key.Name != fyne.KeyPageDown {
		return
	}
	old := e.buffer.caret
	switch key.Name {
	case fyne.KeyLeft:
		e.moveHorizontal(-1)
	case fyne.KeyRight:
		e.moveHorizontal(1)
	case fyne.KeyUp:
		e.moveVertical(-1)
	case fyne.KeyDown:
		e.moveVertical(1)
	case fyne.KeyHome:
		e.moveLineBoundary(false)
	case fyne.KeyEnd:
		e.moveLineBoundary(true)
	case fyne.KeyPageUp:
		e.movePage(-1)
	case fyne.KeyPageDown:
		e.movePage(1)
	case fyne.KeyBackspace:
		e.backspace()
		return
	case fyne.KeyDelete:
		e.deleteForward()
		return
	case fyne.KeyReturn, fyne.KeyEnter:
		e.insertNewline()
		return
	case fyne.KeyTab:
		if e.shift {
			e.changeIndent(false)
		} else {
			e.changeIndent(true)
		}
		return
	case fyne.KeyEscape:
		e.dismissCompletion()
		return
	default:
		return
	}
	e.afterCursorMove(old)
}

func (e *SourceCodeEditor) TypedShortcut(shortcut fyne.Shortcut) {
	switch typed := shortcut.(type) {
	case *fyne.ShortcutCopy:
		e.copyTo(typed.Clipboard)
		return
	case *fyne.ShortcutCut:
		e.copyTo(typed.Clipboard)
		if !e.readOnly {
			start, end := e.buffer.selection()
			if start != end {
				e.ReplaceRange(start, end, "")
			}
		}
		return
	case *fyne.ShortcutPaste:
		if !e.readOnly && typed.Clipboard != nil {
			start, end := e.buffer.selection()
			e.ReplaceRange(start, end, typed.Clipboard.Content())
		}
		return
	case *fyne.ShortcutSelectAll:
		e.SelectRange(0, len(e.buffer.text))
		return
	case *fyne.ShortcutUndo:
		e.Undo()
		return
	case *fyne.ShortcutRedo:
		e.Redo()
		return
	}
	key, ok := shortcut.(fyne.KeyboardShortcut)
	if !ok {
		e.forwardUnhandledShortcut(shortcut)
		return
	}
	mods := key.Mod()
	defaultMod := mods&(fyne.KeyModifierControl|fyne.KeyModifierSuper) != 0
	shift := mods&fyne.KeyModifierShift != 0
	if key.Key() == fyne.KeySpace && mods&fyne.KeyModifierControl != 0 {
		e.RequestCompletion()
		return
	}
	if !defaultMod {
		if key.Key() == fyne.KeyTab && shift {
			e.changeIndent(false)
			return
		}
		e.forwardUnhandledShortcut(shortcut)
		return
	}
	handled := true
	switch key.Key() {
	case fyne.KeyS:
		if e.OnSaveRequested != nil {
			e.OnSaveRequested(shift)
		} else {
			handled = false
		}
	case fyne.KeyF:
		if e.OnSearchRequested != nil {
			e.OnSearchRequested(false)
		} else {
			handled = false
		}
	case fyne.KeyH:
		if e.OnSearchRequested != nil {
			e.OnSearchRequested(true)
		} else {
			handled = false
		}
	case fyne.KeyA:
		if shift {
			if e.OnAssetRequested != nil {
				e.OnAssetRequested()
			} else {
				handled = false
			}
		} else {
			e.SelectRange(0, len(e.buffer.text))
		}
	case fyne.KeySlash:
		e.toggleComments()
	case fyne.KeyZ:
		if shift {
			e.Redo()
		} else {
			e.Undo()
		}
	case fyne.KeyY:
		e.Redo()
	case fyne.KeySpace:
		e.RequestCompletion()
	default:
		handled = false
	}
	if !handled {
		e.forwardUnhandledShortcut(shortcut)
	}
}

func (e *SourceCodeEditor) forwardUnhandledShortcut(shortcut fyne.Shortcut) {
	if e.OnUnhandledShortcut != nil {
		e.OnUnhandledShortcut(shortcut)
	}
}

func (e *SourceCodeEditor) Tapped(event *fyne.PointEvent) {
	if e.suppressTap {
		e.suppressTap = false
		return
	}
	e.focus()
	e.moveCaretFromPoint(event.Position, false)
}

func (e *SourceCodeEditor) MouseDown(event *desktop.MouseEvent) {
	if event.Button != desktop.MouseButtonPrimary {
		return
	}
	e.focus()
	extend := event.Modifier&fyne.KeyModifierShift != 0
	e.moveCaretFromPoint(event.Position, extend)
	e.mouseSelecting = true
	e.suppressTap = true
	e.dragPosition = event.Position
}

func (e *SourceCodeEditor) MouseUp(*desktop.MouseEvent) {
	e.mouseSelecting = false
}

func (e *SourceCodeEditor) Dragged(event *fyne.DragEvent) {
	if !e.mouseSelecting {
		return
	}
	// Accumulate deltas from the press: equals the pointer position on GLFW
	// and remains correct for drivers that report the drag origin.
	e.dragPosition = e.dragPosition.Add(fyne.NewPos(event.Dragged.DX, event.Dragged.DY))
	e.moveCaretFromPoint(e.dragPosition, true)
}

func (e *SourceCodeEditor) DragEnd() {
	e.mouseSelecting = false
	e.suppressTap = false
}

func (e *SourceCodeEditor) focus() {
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}
	if codeCanvas := app.Driver().CanvasForObject(e); codeCanvas != nil {
		codeCanvas.Focus(e)
	}
}

func (e *SourceCodeEditor) moveCaretFromPoint(point fyne.Position, extend bool) {
	gutterWidth := e.gutterScroll.Size().Width
	if point.X < gutterWidth {
		point.X = gutterWidth
	}
	local := fyne.NewPos(point.X-gutterWidth+e.codeScroll.Offset.X, point.Y+e.codeScroll.Offset.Y)
	row, column := e.codeGrid.CursorLocationForPosition(local)
	lines := sourceTextLines(e.buffer.text)
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	offset := sourceOffsetForColumn(lines[row], column)
	e.dismissCompletion()
	old := e.buffer.caret
	if !extend {
		e.buffer.anchor = offset
	}
	e.buffer.caret = offset
	e.preferredColumn = -1
	e.afterCursorMove(old)
}

func (e *SourceCodeEditor) moveHorizontal(direction int) {
	e.preferredColumn = -1
	start, end := e.buffer.selection()
	if !e.shift && start != end {
		if direction < 0 {
			e.buffer.caret = start
		} else {
			e.buffer.caret = end
		}
		e.buffer.anchor = e.buffer.caret
		return
	}
	old := e.buffer.caret
	if direction < 0 {
		e.buffer.caret = previousSourceOffset(e.buffer.text, e.buffer.caret)
	} else {
		e.buffer.caret = nextSourceOffset(e.buffer.text, e.buffer.caret)
	}
	e.updateAnchorAfterMove(old)
}

func (e *SourceCodeEditor) moveVertical(direction int) {
	lines := sourceTextLines(e.buffer.text)
	row := sourceLineForOffset(lines, e.buffer.caret)
	if e.preferredColumn < 0 {
		e.preferredColumn = sourceColumnForOffset(lines[row], e.buffer.caret)
	}
	target := row + direction
	if target < 0 {
		target = 0
	}
	if target >= len(lines) {
		target = len(lines) - 1
	}
	old := e.buffer.caret
	e.buffer.caret = sourceOffsetForColumn(lines[target], e.preferredColumn)
	e.updateAnchorAfterMove(old)
}

func (e *SourceCodeEditor) movePage(direction int) {
	lines := sourceTextLines(e.buffer.text)
	row := sourceLineForOffset(lines, e.buffer.caret)
	if e.preferredColumn < 0 {
		e.preferredColumn = sourceColumnForOffset(lines[row], e.buffer.caret)
	}
	cellHeight := e.codeGrid.PositionForCursorLocation(1, 0).Y
	rows := 10
	if cellHeight > 0 && e.codeScroll.Size().Height > cellHeight {
		rows = int(e.codeScroll.Size().Height / cellHeight)
	}
	target := row + direction*rows
	if target < 0 {
		target = 0
	}
	if target >= len(lines) {
		target = len(lines) - 1
	}
	old := e.buffer.caret
	e.buffer.caret = sourceOffsetForColumn(lines[target], e.preferredColumn)
	e.updateAnchorAfterMove(old)
}

func (e *SourceCodeEditor) moveLineBoundary(end bool) {
	e.preferredColumn = -1
	lines := sourceTextLines(e.buffer.text)
	line := lines[sourceLineForOffset(lines, e.buffer.caret)]
	old := e.buffer.caret
	if end {
		e.buffer.caret = line.ContentEnd
	} else {
		e.buffer.caret = line.Start
	}
	e.updateAnchorAfterMove(old)
}

func (e *SourceCodeEditor) updateAnchorAfterMove(old int) {
	if e.shift {
		if e.buffer.anchor == old && e.buffer.anchor == e.buffer.caret {
			e.buffer.anchor = old
		}
		return
	}
	e.buffer.anchor = e.buffer.caret
}

func (e *SourceCodeEditor) backspace() {
	if e.readOnly {
		return
	}
	start, end := e.buffer.selection()
	if start == end {
		start = previousSourceOffset(e.buffer.text, start)
	}
	e.ReplaceRange(start, end, "")
}

func (e *SourceCodeEditor) deleteForward() {
	if e.readOnly {
		return
	}
	start, end := e.buffer.selection()
	if start == end {
		end = nextSourceOffset(e.buffer.text, end)
	}
	e.ReplaceRange(start, end, "")
}

func (e *SourceCodeEditor) insertNewline() {
	if e.readOnly {
		return
	}
	lines := sourceTextLines(e.buffer.text)
	line := lines[sourceLineForOffset(lines, e.buffer.caret)]
	prefix := e.buffer.text[line.Start:e.buffer.caret]
	indentLength := 0
	for indentLength < len(prefix) && (prefix[indentLength] == ' ' || prefix[indentLength] == '\t') {
		indentLength++
	}
	indent := prefix[:indentLength]
	if strings.HasSuffix(strings.TrimSpace(prefix), "{") {
		indent += "\t"
	}
	start, end := e.buffer.selection()
	e.ReplaceRange(start, end, preferredSourceLineEnding(e.buffer.text, line)+indent)
}

func preferredSourceLineEnding(text string, line sourceLine) string {
	if line.ContentEnd < line.End {
		return text[line.ContentEnd:line.End]
	}
	if index := strings.Index(text, "\n"); index >= 0 {
		if index > 0 && text[index-1] == '\r' {
			return "\r\n"
		}
		return "\n"
	}
	if strings.Contains(text, "\r") {
		return "\r"
	}
	return "\n"
}

func (e *SourceCodeEditor) changeIndent(indent bool) {
	if e.readOnly {
		return
	}
	start, end := e.buffer.selection()
	if start == end && indent {
		e.ReplaceRange(start, end, "\t")
		return
	}
	lines := sourceTextLines(e.buffer.text)
	first, last := selectedSourceLines(lines, start, end)
	deltas := make([]sourceEditDelta, 0, last-first+1)
	for row := first; row <= last; row++ {
		line := lines[row]
		if indent {
			deltas = append(deltas, sourceEditDelta{Start: line.Start, After: "\t"})
			continue
		}
		remove := 0
		if line.Start < line.ContentEnd && e.buffer.text[line.Start] == '\t' {
			remove = 1
		} else {
			for remove < 4 && line.Start+remove < line.ContentEnd && e.buffer.text[line.Start+remove] == ' ' {
				remove++
			}
		}
		if remove > 0 {
			deltas = append(deltas, sourceEditDelta{Start: line.Start, Before: e.buffer.text[line.Start : line.Start+remove]})
		}
	}
	e.applyLineEdits(deltas)
}

func (e *SourceCodeEditor) toggleComments() {
	if e.readOnly {
		return
	}
	start, end := e.buffer.selection()
	lines := sourceTextLines(e.buffer.text)
	first, last := selectedSourceLines(lines, start, end)
	allCommented := true
	hasContent := false
	commentAt := make([]int, last-first+1)
	for row := first; row <= last; row++ {
		line := lines[row]
		position := line.Start
		for position < line.ContentEnd && (e.buffer.text[position] == ' ' || e.buffer.text[position] == '\t') {
			position++
		}
		commentAt[row-first] = position
		if position == line.ContentEnd {
			continue
		}
		hasContent = true
		if !strings.HasPrefix(e.buffer.text[position:line.ContentEnd], "//") {
			allCommented = false
		}
	}
	deltas := make([]sourceEditDelta, 0, last-first+1)
	for row := first; row <= last; row++ {
		position := commentAt[row-first]
		if position == lines[row].ContentEnd {
			continue
		}
		if hasContent && allCommented {
			deltas = append(deltas, sourceEditDelta{Start: position, Before: "//"})
		} else {
			deltas = append(deltas, sourceEditDelta{Start: position, After: "//"})
		}
	}
	e.applyLineEdits(deltas)
}

func selectedSourceLines(lines []sourceLine, start, end int) (int, int) {
	first := sourceLineForOffset(lines, start)
	lastOffset := end
	if end > start && first < len(lines)-1 && end == lines[sourceLineForOffset(lines, end)].Start {
		lastOffset = previousSourceOffsetFromLines(lines, end)
	}
	return first, sourceLineForOffset(lines, lastOffset)
}

func previousSourceOffsetFromLines(lines []sourceLine, offset int) int {
	for i := 1; i < len(lines); i++ {
		if lines[i].Start == offset {
			return lines[i-1].ContentEnd
		}
	}
	return offset
}

func (e *SourceCodeEditor) applyLineEdits(deltas []sourceEditDelta) {
	if len(deltas) == 0 {
		return
	}
	sortedSourceDeltas(deltas)
	tx := sourceEditTransaction{
		Deltas:       deltas,
		CaretBefore:  e.buffer.caret,
		AnchorBefore: e.buffer.anchor,
		CaretAfter:   transformedSourceOffset(e.buffer.caret, deltas),
		AnchorAfter:  transformedSourceOffset(e.buffer.anchor, deltas),
	}
	e.buffer.applyNew(tx)
	e.afterUserEdit(-1)
}

func transformedSourceOffset(offset int, deltas []sourceEditDelta) int {
	result := offset
	for _, delta := range deltas {
		end := delta.Start + len(delta.Before)
		if offset > end {
			result += len(delta.After) - len(delta.Before)
		} else if offset > delta.Start {
			result = delta.Start + len(delta.After)
		}
	}
	return result
}

func (e *SourceCodeEditor) copyTo(clipboard fyne.Clipboard) {
	if clipboard == nil {
		return
	}
	start, end := e.buffer.selection()
	if start != end {
		clipboard.SetContent(e.buffer.text[start:end])
	}
}

func (e *SourceCodeEditor) afterUserEdit(oldCaret int) {
	e.preferredColumn = -1
	e.rebuildTextSurfaces()
	if e.OnChanged != nil {
		e.OnChanged(e.buffer.text)
	}
	e.refreshStyles()
	e.ensureCaretVisible()
	if oldCaret != e.buffer.caret && e.OnCursorChanged != nil {
		e.OnCursorChanged(e.buffer.caret)
	}
}

func (e *SourceCodeEditor) afterCursorMove(oldCaret int) {
	e.refreshStyles()
	e.ensureCaretVisible()
	if oldCaret != e.buffer.caret && e.OnCursorChanged != nil {
		e.OnCursorChanged(e.buffer.caret)
	}
}

func (e *SourceCodeEditor) ensureCaretVisible() {
	lines := sourceTextLines(e.buffer.text)
	row := sourceLineForOffset(lines, e.buffer.caret)
	column := sourceColumnForOffset(lines[row], e.buffer.caret)
	position := e.codeGrid.PositionForCursorLocation(row, column)
	cellWidth := e.codeGrid.PositionForCursorLocation(0, 1).X
	cellHeight := e.codeGrid.PositionForCursorLocation(1, 0).Y
	if cellWidth <= 0 {
		cellWidth = 10
	}
	if cellHeight <= 0 {
		cellHeight = 18
	}
	offset := e.codeScroll.Offset
	view := e.codeScroll.Size()
	if position.X < offset.X {
		offset.X = position.X
	} else if position.X+cellWidth > offset.X+view.Width {
		offset.X = position.X + cellWidth - view.Width
	}
	if position.Y < offset.Y {
		offset.Y = position.Y
	} else if position.Y+cellHeight > offset.Y+view.Height {
		offset.Y = position.Y + cellHeight - view.Height
	}
	if offset.X < 0 {
		offset.X = 0
	}
	if offset.Y < 0 {
		offset.Y = 0
	}
	e.codeScroll.ScrollToOffset(offset)
	e.gutterScroll.ScrollToOffset(fyne.NewPos(0, offset.Y))
}

func (e *SourceCodeEditor) rebuildCompletionUI() {
	e.completionUI.RemoveAll()
	e.completionRows = e.completionRows[:0]
	if len(e.completionItems) == 0 {
		e.completion.Hide()
		return
	}
	for index := range e.completionItems {
		row := newSourceCompletionRow(func() { e.acceptCompletion(index) })
		e.completionRows = append(e.completionRows, row)
		e.completionUI.Add(row)
	}
	e.updateCompletionLabels()
	e.completion.Show()
	e.Refresh()
}

func (e *SourceCodeEditor) updateCompletionLabels() {
	for index, row := range e.completionRows {
		row.setState(sourceCompletionRowLabel(e.completionItems[index]), index == e.completionIndex, e.palette)
	}
}

const sourceCompletionRowRunes = 96

// sourceCompletionRowLabel fits label and detail on one canvas.Text line:
// catalog descriptions can contain newlines, which canvas.Text cannot draw.
func sourceCompletionRowLabel(item SourceCompletion) string {
	label := item.Label
	if item.Detail != "" {
		label += " — " + item.Detail
	}
	label = strings.Join(strings.Fields(label), " ")
	if utf8.RuneCountInString(label) <= sourceCompletionRowRunes {
		return label
	}
	runes := []rune(label)
	return string(runes[:sourceCompletionRowRunes-1]) + "…"
}

func (e *SourceCodeEditor) moveCompletion(delta int) {
	if len(e.completionItems) == 0 {
		return
	}
	e.completionIndex = (e.completionIndex + delta + len(e.completionItems)) % len(e.completionItems)
	e.updateCompletionLabels()
}

func (e *SourceCodeEditor) acceptCompletion(index int) {
	if index < 0 || index >= len(e.completionItems) {
		return
	}
	item := e.completionItems[index]
	e.dismissCompletion()
	e.ReplaceRange(item.Replace.Start, item.Replace.End, item.InsertText)
	e.focus()
}

func (e *SourceCodeEditor) dismissCompletion() {
	e.completionItems = nil
	e.completionRows = nil
	e.completionUI.RemoveAll()
	e.completion.Hide()
}

func (e *SourceCodeEditor) rebuildTextSurfaces() {
	lines := sourceTextLines(e.buffer.text)
	var display strings.Builder
	for index, line := range lines {
		display.WriteString(e.buffer.text[line.Start:line.ContentEnd])
		display.WriteByte(' ')
		if index != len(lines)-1 {
			display.WriteByte('\n')
		}
	}
	e.codeGrid.SetText(display.String())
	e.rebuildGutter(lines)
	e.codeGrid.Refresh()
	e.gutterGrid.Refresh()
	e.BaseWidget.Refresh()
}

func (e *SourceCodeEditor) rebuildGutter(lines []sourceLine) {
	digits := len(sourceLineNumber(len(lines)))
	markers := e.diagnosticLineMarkers(lines)
	var gutter strings.Builder
	for index := range lines {
		marker := ' '
		if markers[index] == SourceError {
			marker = '!'
		} else if markers[index] == SourceWarning {
			marker = '▲'
		}
		gutter.WriteRune(marker)
		gutter.WriteByte(' ')
		number := sourceLineNumber(index + 1)
		gutter.WriteString(strings.Repeat(" ", digits-len(number)))
		gutter.WriteString(number)
		gutter.WriteByte(' ')
		if index != len(lines)-1 {
			gutter.WriteByte('\n')
		}
	}
	e.gutterGrid.SetText(gutter.String())
	e.gutterGrid.Refresh()
}

func (e *SourceCodeEditor) refreshStyles() {
	lines := sourceTextLines(e.buffer.text)
	markers := e.diagnosticLineMarkers(lines)
	e.styleCodeGrid(lines)
	e.styleGutter(markers)
	e.background.FillColor = e.palette.Background
	e.codeGrid.Refresh()
	e.gutterGrid.Refresh()
	e.background.Refresh()
	e.BaseWidget.Refresh()
}

func sourceLineNumber(number int) string {
	if number == 0 {
		return "0"
	}
	var digits [24]byte
	index := len(digits)
	for number > 0 {
		index--
		digits[index] = byte('0' + number%10)
		number /= 10
	}
	return string(digits[index:])
}

func (e *SourceCodeEditor) diagnosticLineMarkers(lines []sourceLine) []SourceSeverity {
	markers := make([]SourceSeverity, len(lines))
	for _, diagnostic := range e.analysis.Diagnostics {
		line := diagnostic.Line - 1
		if line < 0 || line >= len(lines) {
			line = sourceLineForOffset(lines, clampSourceOffset(e.buffer.text, diagnostic.Range.Start))
		}
		if diagnostic.Severity == SourceError || markers[line] == "" {
			markers[line] = diagnostic.Severity
		}
	}
	return markers
}

func (e *SourceCodeEditor) styleCodeGrid(lines []sourceLine) {
	selectionStart, selectionEnd := e.buffer.selection()
	matchOpen, matchClose := e.matchingBrackets()
	runIndex := 0
	for row, line := range lines {
		gridColumn := 0
		for _, cell := range line.Cells {
			for runIndex < len(e.analysis.Syntax) && e.analysis.Syntax[runIndex].Range.End <= cell.Start {
				runIndex++
			}
			class := SourceSyntaxText
			if runIndex < len(e.analysis.Syntax) {
				run := e.analysis.Syntax[runIndex]
				if run.Range.Start < cell.End && run.Range.End > cell.Start && run.Class < sourceSyntaxClassCount {
					class = run.Class
				}
			}
			background := uint8(0)
			if selectionStart != selectionEnd && cell.End > selectionStart && cell.Start < selectionEnd {
				background = 1
			}
			if cell.Start == matchOpen || cell.Start == matchClose {
				background = 3
			}
			if e.focused && e.buffer.caret == cell.Start && selectionStart == selectionEnd {
				background = 2
			}
			underline := e.cellHasDiagnostic(cell.Start, cell.End)
			style := e.sourceStyle(sourceCellStyleKey{Class: class, Background: background, Underline: underline})
			for column := 0; column < cell.Width; column++ {
				e.codeGrid.SetStyle(row, gridColumn+column, style)
			}
			gridColumn += cell.Width
		}
		background := uint8(0)
		if selectionStart != selectionEnd && line.ContentEnd >= selectionStart && line.ContentEnd < selectionEnd {
			background = 1
		}
		if e.focused && e.buffer.caret == line.ContentEnd && selectionStart == selectionEnd {
			background = 2
		}
		e.codeGrid.SetStyle(row, gridColumn, e.sourceStyle(sourceCellStyleKey{Class: SourceSyntaxText, Background: background}))
	}
}

func (e *SourceCodeEditor) sourceStyle(key sourceCellStyleKey) *widget.CustomTextGridStyle {
	if style, ok := e.styleCache[key]; ok {
		return style
	}
	foreground := color.Color(e.palette.Colors[key.Class])
	background := color.Color(e.palette.Background)
	switch key.Background {
	case 1:
		background = e.palette.Selection
	case 2:
		background = e.palette.Caret
		foreground = e.palette.Background
	case 3:
		background = e.palette.MatchingBracket
	}
	style := &widget.CustomTextGridStyle{
		FGColor: foreground,
		BGColor: background,
		TextStyle: fyne.TextStyle{
			Monospace: true,
			Italic:    key.Class == SourceSyntaxComment,
			Bold:      key.Class == SourceSyntaxHeader,
			Underline: key.Underline,
		},
	}
	e.styleCache[key] = style
	return style
}

func (e *SourceCodeEditor) styleGutter(markers []SourceSeverity) {
	for row, gridRow := range e.gutterGrid.Rows {
		foreground := e.palette.LineNumber
		if markers[row] == SourceError {
			foreground = e.palette.Error
		} else if markers[row] == SourceWarning {
			foreground = e.palette.Warning
		}
		style := &widget.CustomTextGridStyle{
			FGColor:   foreground,
			BGColor:   e.palette.Background,
			TextStyle: fyne.TextStyle{Monospace: true},
		}
		for column := range gridRow.Cells {
			e.gutterGrid.SetStyle(row, column, style)
		}
	}
}

func (e *SourceCodeEditor) cellHasDiagnostic(start, end int) bool {
	for _, diagnostic := range e.analysis.Diagnostics {
		if diagnostic.Range.End > start && diagnostic.Range.Start < end {
			return true
		}
	}
	return false
}

func (e *SourceCodeEditor) matchingBrackets() (int, int) {
	for _, pair := range e.analysis.Brackets {
		if e.buffer.caret == pair.Open || e.buffer.caret == pair.Open+1 || e.buffer.caret == pair.Close || e.buffer.caret == pair.Close+1 {
			return pair.Open, pair.Close
		}
	}
	return -1, -1
}

type sourceCodeEditorRenderer struct {
	editor  *SourceCodeEditor
	objects []fyne.CanvasObject
}

func (r *sourceCodeEditorRenderer) Destroy() {}

func (r *sourceCodeEditorRenderer) Layout(size fyne.Size) {
	r.editor.background.Move(fyne.Position{})
	r.editor.background.Resize(size)
	gutterWidth := r.editor.gutterGrid.MinSize().Width
	if gutterWidth < 44 {
		gutterWidth = 44
	}
	if gutterWidth > 96 {
		gutterWidth = 96
	}
	r.editor.gutterScroll.Move(fyne.Position{})
	r.editor.gutterScroll.Resize(fyne.NewSize(gutterWidth, size.Height))
	r.editor.codeScroll.Move(fyne.NewPos(gutterWidth, 0))
	r.editor.codeScroll.Resize(fyne.NewSize(fyne.Max(0, size.Width-gutterWidth), size.Height))

	if r.editor.completion.Visible() {
		popupSize := r.editor.completion.MinSize()
		popupSize.Width = fyne.Min(fyne.Max(popupSize.Width, 220), fyne.Max(0, size.Width-gutterWidth-16))
		popupSize.Height = fyne.Min(popupSize.Height, fyne.Max(0, size.Height-16))
		r.editor.completion.Resize(popupSize)
		r.editor.completion.Move(fyne.NewPos(fyne.Max(gutterWidth, size.Width-popupSize.Width-8), 8))
	}
}

func (r *sourceCodeEditorRenderer) MinSize() fyne.Size {
	return fyne.NewSize(220, 120)
}

func (r *sourceCodeEditorRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *sourceCodeEditorRenderer) Refresh() {
	r.Layout(r.editor.Size())
	for _, object := range r.objects {
		object.Refresh()
	}
}

type sourceMatch struct {
	Start int
	End   int
}

func findSourceMatch(text, query string, from int, matchCase, backwards bool) (int, int, bool) {
	matches := allSourceMatches(text, query, matchCase)
	if len(matches) == 0 {
		return 0, 0, false
	}
	from = clampSourceOffset(text, from)
	if backwards {
		for index := len(matches) - 1; index >= 0; index-- {
			if matches[index].Start < from {
				return matches[index].Start, matches[index].End, true
			}
		}
		last := matches[len(matches)-1]
		return last.Start, last.End, true
	}
	for _, match := range matches {
		if match.Start >= from {
			return match.Start, match.End, true
		}
	}
	return matches[0].Start, matches[0].End, true
}

func allSourceMatches(text, query string, matchCase bool) []sourceMatch {
	if query == "" {
		return nil
	}
	queryRunes := utf8.RuneCountInString(query)
	boundaries := make([]int, 0, utf8.RuneCountInString(text)+1)
	for offset := 0; offset < len(text); {
		boundaries = append(boundaries, offset)
		_, size := utf8.DecodeRuneInString(text[offset:])
		offset += size
	}
	boundaries = append(boundaries, len(text))
	matches := make([]sourceMatch, 0)
	for index := 0; index+queryRunes < len(boundaries); {
		endIndex := index + queryRunes
		candidate := text[boundaries[index]:boundaries[endIndex]]
		if sourceTextEqual(candidate, query, matchCase) {
			matches = append(matches, sourceMatch{Start: boundaries[index], End: boundaries[endIndex]})
			index = endIndex
		} else {
			index++
		}
	}
	return matches
}

func sourceTextEqual(left, right string, matchCase bool) bool {
	if matchCase {
		return left == right
	}
	return strings.EqualFold(left, right)
}
