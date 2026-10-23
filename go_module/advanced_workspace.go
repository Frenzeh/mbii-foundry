package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	editorModeGuided   = "guided"
	editorModeAdvanced = "advanced"
)

type advancedDocumentView struct {
	editor Editor
	format SourceFormat
	code   *SourceCodeEditor

	content       fyne.CanvasObject
	guidedContent fyne.CanvasObject
	searchBar     *fyne.Container
	replaceRow    *fyne.Container
	findEntry     *widget.Entry
	replaceEntry  *widget.Entry
	matchCase     *widget.Check
	searchStatus  *widget.Label
	cursorStatus  *widget.Label
	draftStatus   *widget.Label
	diagnosticBtn *widget.Button
	applyBtn      *widget.Button

	analysis     SourceAnalysis
	analyzed     bool
	draftRemove  func()
	sourceRemove func()
	publishing   bool
}

// AdvancedWorkspace is a presentation of the same SessionEditor and
// SourceDraftStore used by Guided mode. Its per-document code widgets retain
// cursor, selection and undo history while tabs or presentation modes change;
// raw text remains authoritative only in SourceDraftStore.
type AdvancedWorkspace struct {
	app      *App
	language *SourceLanguage

	documents  map[Editor]*advancedDocumentView
	active     Editor
	palette    SourcePalette
	hasPalette bool

	guidedBanner *fyne.Container
	guidedLabel  *widget.Label
	guidedApply  *widget.Button
}

func NewAdvancedWorkspace(a *App) *AdvancedWorkspace {
	w := &AdvancedWorkspace{
		app:       a,
		language:  NewSourceLanguage(a.sourceAssetSuggestions),
		documents: make(map[Editor]*advancedDocumentView),
	}

	w.guidedLabel = widget.NewLabel("")
	w.guidedLabel.Wrapping = fyne.TextWrapWord
	w.guidedApply = widget.NewButton("Apply to Guided", w.applyActiveDraft)
	w.guidedApply.Importance = widget.HighImportance
	openAdvanced := widget.NewButton("Edit in Advanced", func() {
		w.app.setEditorMode(editorModeAdvanced, true)
	})
	w.guidedBanner = container.NewPadded(container.NewVBox(
		w.guidedLabel,
		container.NewHScroll(container.NewHBox(w.guidedApply, openAdvanced)),
	))
	w.guidedBanner.Hide()

	if err := w.RefreshPalette(); err != nil && a != nil {
		a.configWarning = strings.TrimSpace(a.configWarning + "\nSyntax colours were not applied: " + err.Error())
	}
	return w
}

func (w *AdvancedWorkspace) GuidedBanner() fyne.CanvasObject {
	if w == nil {
		return nil
	}
	return w.guidedBanner
}

func (w *AdvancedWorkspace) Close() {
	if w == nil {
		return
	}
	for ed := range w.documents {
		w.RemoveEditor(ed)
	}
}

func (w *AdvancedWorkspace) RemoveEditor(ed Editor) {
	if w == nil || ed == nil {
		return
	}
	doc := w.documents[ed]
	if doc == nil {
		return
	}
	// Invalidate the code surface before releasing subscriptions. Independent
	// picker windows may still retain it after the tab closes; read-only state
	// and detached callbacks make every such target inert.
	doc.code.SetReadOnly(true)
	doc.code.OnChanged = nil
	doc.code.OnCursorChanged = nil
	doc.code.OnRequestCompletions = nil
	doc.code.OnSearchRequested = nil
	doc.code.OnAssetRequested = nil
	doc.code.OnSaveRequested = nil
	doc.code.OnUndoFallback = nil
	doc.code.OnRedoFallback = nil
	doc.code.OnUnhandledShortcut = nil
	if doc.draftRemove != nil {
		doc.draftRemove()
		doc.draftRemove = nil
	}
	if doc.sourceRemove != nil {
		doc.sourceRemove()
		doc.sourceRemove = nil
	}
	delete(w.documents, ed)
	if w.active == ed {
		w.active = nil
	}
}

func (w *AdvancedWorkspace) SetActiveEditor(ed Editor) {
	if w == nil {
		return
	}
	w.active = ed
	if ed == nil {
		w.refreshGuidedBanner()
		return
	}
	doc := w.documents[ed]
	if doc == nil {
		doc = w.newDocument(ed)
		w.documents[ed] = doc
	}
	w.applyPresentationForEditor(ed, normalizeEditorMode(w.app.config.EditorMode))
	w.refreshDocumentStatus(doc)
	w.refreshGuidedBanner()
}

// SetMode swaps each tab's presentation in place. The DocTabs widget and its
// selected item never change, so mode changes cannot lose tab identity or
// document-local editor history.
func (w *AdvancedWorkspace) SetMode(mode string) {
	if w == nil || w.app == nil || w.app.docTabs == nil {
		return
	}
	mode = normalizeEditorMode(mode)
	for _, tab := range w.app.docTabs.Items {
		ed := w.app.editors[tab]
		if ed != nil {
			w.applyPresentationForEditor(ed, mode)
		}
	}
	w.app.docTabs.Refresh()
}

func (w *AdvancedWorkspace) applyPresentationForEditor(ed Editor, mode string) {
	if w == nil || ed == nil || w.app == nil || w.app.docTabs == nil {
		return
	}
	var tab *container.TabItem
	for candidate, candidateEditor := range w.app.editors {
		if candidateEditor == ed {
			tab = candidate
			break
		}
	}
	if tab == nil {
		return
	}
	doc := w.documents[ed]
	if doc == nil {
		doc = w.newDocument(ed)
		w.documents[ed] = doc
	}
	if doc.guidedContent == nil {
		doc.guidedContent = tab.Content
	}
	if mode == editorModeAdvanced {
		tab.Content = doc.content
	} else {
		tab.Content = doc.guidedContent
	}
}

func (w *AdvancedWorkspace) newDocument(ed Editor) *advancedDocumentView {
	doc := &advancedDocumentView{
		editor: ed,
		format: sourceFormatForEditor(ed),
		code:   NewSourceCodeEditor(),
	}
	doc.code.SetReadOnly(false)
	doc.cursorStatus = widget.NewLabel("")
	doc.cursorStatus.TextStyle = fyne.TextStyle{Monospace: true}
	doc.draftStatus = widget.NewLabel("")
	doc.draftStatus.TextStyle = fyne.TextStyle{Bold: true}
	doc.diagnosticBtn = widget.NewButton("No diagnostics", func() { w.showDiagnostics(doc) })
	doc.diagnosticBtn.Importance = widget.LowImportance
	doc.applyBtn = widget.NewButton("Apply", func() { w.applyDraft(doc) })
	doc.applyBtn.Importance = widget.HighImportance

	doc.code.OnChanged = func(text string) {
		analysis := w.analyze(doc, text)
		doc.analysis = analysis
		doc.analyzed = true
		doc.code.SetAnalysis(analysis)
		doc.publishing = true
		w.app.sourceDrafts.Set(ed, SourceDraft{
			Text:     text,
			ParseErr: sourceAnalysisError(analysis),
			EditMode: true,
		})
		doc.publishing = false
		w.refreshDocumentStatus(doc)
		w.refreshGuidedBanner()
	}
	doc.code.OnCursorChanged = func(int) { w.refreshDocumentStatus(doc) }
	doc.code.OnRequestCompletions = func(text string, caret int) []SourceCompletion {
		if doc.format == "" {
			return nil
		}
		return w.language.Complete(text, caret, doc.format)
	}
	doc.code.OnSearchRequested = func(replace bool) { w.showSearch(doc, replace) }
	doc.code.OnAssetRequested = func() {
		if doc.format != "" {
			w.app.showSourceAssetPicker(doc.code, doc.format)
		}
	}
	doc.code.OnSaveRequested = func(saveAs bool) {
		if saveAs {
			w.app.saveFileAs()
			return
		}
		w.app.saveFile()
	}
	doc.code.OnUndoFallback = func() bool { return w.undoSessionFallback(doc) }
	doc.code.OnRedoFallback = func() bool { return w.redoSessionFallback(doc) }
	doc.code.OnUnhandledShortcut = func(shortcut fyne.Shortcut) {
		w.app.DispatchShortcut(shortcut)
	}
	if w.hasPalette {
		doc.code.SetPalette(w.palette)
	}

	doc.searchBar = w.newSearchBar(doc)
	doc.searchBar.Hide()
	completionBtn := widget.NewButton("Complete", doc.code.RequestCompletion)
	assetBtn := widget.NewButton("Insert Asset", doc.code.OnAssetRequested)
	findBtn := widget.NewButton("Find", func() { w.showSearch(doc, false) })
	replaceBtn := widget.NewButton("Replace", func() { w.showSearch(doc, true) })
	revertBtn := widget.NewButton("Revert Draft", func() { w.confirmRevert(doc) })
	for _, button := range []*widget.Button{completionBtn, assetBtn, findBtn, replaceBtn, revertBtn} {
		button.Importance = widget.LowImportance
	}
	actions := container.NewHScroll(container.NewHBox(
		doc.draftStatus,
		doc.applyBtn,
		revertBtn,
		findBtn,
		replaceBtn,
		completionBtn,
		assetBtn,
	))
	status := container.NewHScroll(container.NewHBox(doc.cursorStatus, doc.diagnosticBtn))
	doc.content = container.NewBorder(
		container.NewVBox(actions, doc.searchBar),
		status,
		nil,
		nil,
		doc.code,
	)

	// Analyze once when the document view is created. Later presentation
	// switches reuse this cached result and never parse again.
	w.syncDocumentText(doc)
	doc.draftRemove = w.app.sourceDrafts.addListener(ed, func() {
		fyne.Do(func() {
			if w.documents[ed] != doc || doc.publishing {
				return
			}
			w.syncDocumentText(doc)
			w.refreshGuidedBanner()
		})
	})
	if source, ok := ed.(SourceListenerProvider); ok {
		doc.sourceRemove = source.AddSourceListener(func() {
			fyne.Do(func() {
				if w.documents[ed] != doc || w.app.sourceDrafts.Has(ed) {
					return
				}
				w.syncDocumentText(doc)
			})
		})
	}
	return doc
}

func (w *AdvancedWorkspace) newSearchBar(doc *advancedDocumentView) *fyne.Container {
	doc.findEntry = widget.NewEntry()
	doc.findEntry.SetPlaceHolder("Find")
	doc.replaceEntry = widget.NewEntry()
	doc.replaceEntry.SetPlaceHolder("Replace with")
	doc.matchCase = widget.NewCheck("Match case", nil)
	doc.searchStatus = widget.NewLabel("")

	find := func(backwards bool) {
		if doc.code.FindNext(doc.findEntry.Text, doc.matchCase.Checked, backwards) {
			doc.searchStatus.SetText("Match selected")
		} else {
			doc.searchStatus.SetText("No match")
		}
		w.focusCode(doc)
	}
	doc.findEntry.OnSubmitted = func(string) { find(false) }
	doc.replaceEntry.OnSubmitted = func(string) {
		if doc.code.ReplaceNext(doc.findEntry.Text, doc.replaceEntry.Text, doc.matchCase.Checked) {
			doc.searchStatus.SetText("Replaced one match")
		} else {
			doc.searchStatus.SetText("No match")
		}
	}
	doc.replaceRow = container.NewHBox(
		doc.replaceEntry,
		widget.NewButton("Replace", func() {
			if doc.code.ReplaceNext(doc.findEntry.Text, doc.replaceEntry.Text, doc.matchCase.Checked) {
				doc.searchStatus.SetText("Replaced one match")
			} else {
				doc.searchStatus.SetText("No match")
			}
		}),
		widget.NewButton("Replace All", func() {
			n := doc.code.ReplaceAll(doc.findEntry.Text, doc.replaceEntry.Text, doc.matchCase.Checked)
			doc.searchStatus.SetText(fmt.Sprintf("Replaced %d match(es)", n))
		}),
	)
	findRow := container.NewHBox(
		doc.findEntry,
		widget.NewButton("Previous", func() { find(true) }),
		widget.NewButton("Next", func() { find(false) }),
		doc.matchCase,
		widget.NewButton("Close", func() {
			doc.searchBar.Hide()
			w.focusCode(doc)
		}),
		doc.searchStatus,
	)
	return container.NewVBox(container.NewHScroll(findRow), container.NewHScroll(doc.replaceRow))
}

func (w *AdvancedWorkspace) showSearch(doc *advancedDocumentView, replace bool) {
	if doc == nil || doc.searchBar == nil {
		return
	}
	doc.searchBar.Show()
	if replace {
		doc.replaceRow.Show()
		if w.app != nil && w.app.mainWindow != nil {
			w.app.mainWindow.Canvas().Focus(doc.replaceEntry)
		}
	} else {
		doc.replaceRow.Hide()
		if w.app != nil && w.app.mainWindow != nil {
			w.app.mainWindow.Canvas().Focus(doc.findEntry)
		}
	}
}

func (w *AdvancedWorkspace) ShowSearch(replace bool) {
	if w == nil {
		return
	}
	w.showSearch(w.documents[w.active], replace)
}

func (w *AdvancedWorkspace) RequestAsset() {
	if doc := w.documents[w.active]; doc != nil && doc.code.OnAssetRequested != nil {
		doc.code.OnAssetRequested()
	}
}

func (w *AdvancedWorkspace) syncDocumentText(doc *advancedDocumentView) {
	if doc == nil {
		return
	}
	text := ""
	draft, hasDraft := w.app.sourceDrafts.Get(doc.editor)
	if hasDraft {
		text = draft.Text
	} else if se, ok := doc.editor.(SessionEditor); ok {
		text = se.CurrentSource()
	} else if provider, ok := doc.editor.(SourceProvider); ok {
		text = provider.GenerateSource()
	}
	textChanged := doc.code.Text() != text
	doc.code.SetText(text)
	if textChanged || !doc.analyzed {
		doc.analysis = w.analyze(doc, text)
		doc.analyzed = true
		doc.code.SetAnalysis(doc.analysis)
	}
	// One validator owns ParseErr at a time. Guided SourcePanel validates while
	// visible; rewriting its differently worded result here would ping-pong
	// through queued draft notifications on the GLFW driver.
	if hasDraft && normalizeEditorMode(w.app.config.EditorMode) == editorModeAdvanced {
		parseErr := sourceAnalysisError(doc.analysis)
		if draft.ParseErr != parseErr {
			draft.ParseErr = parseErr
			doc.publishing = true
			w.app.sourceDrafts.Set(doc.editor, draft)
			doc.publishing = false
		}
	}
	w.refreshDocumentStatus(doc)
}

func (w *AdvancedWorkspace) analyze(doc *advancedDocumentView, text string) SourceAnalysis {
	if doc.format == "" {
		return SourceAnalysis{Diagnostics: []SourceDiagnostic{{
			Range: SourceRange{Start: 0, End: 0}, Line: 1, Column: 1,
			Severity: SourceError, Message: fmt.Sprintf("advanced source editing is unsupported for %T", doc.editor),
		}}}
	}
	return w.language.Analyze(text, doc.format)
}

func sourceAnalysisError(analysis SourceAnalysis) string {
	var messages []string
	for _, diagnostic := range analysis.Diagnostics {
		if diagnostic.Severity == SourceError {
			messages = append(messages, diagnostic.Message)
		}
	}
	return strings.Join(messages, "; ")
}

func (w *AdvancedWorkspace) refreshDocumentStatus(doc *advancedDocumentView) {
	if doc == nil {
		return
	}
	text := doc.code.Text()
	offset := doc.code.CursorOffset()
	line, column := sourceLineColumn(text, offset)
	status := fmt.Sprintf("Line %d, Col %d  •  Byte %d / %d", line, column, clampSourceOffset(text, offset), len(text))
	if match, ok := matchingBracketStatus(doc.analysis.Brackets, offset, text); ok {
		status += "  •  " + match
	}
	doc.cursorStatus.SetText(status)

	errors, warnings := 0, 0
	for _, diagnostic := range doc.analysis.Diagnostics {
		if diagnostic.Severity == SourceError {
			errors++
		} else if diagnostic.Severity == SourceWarning {
			warnings++
		}
	}
	if errors == 0 && warnings == 0 {
		doc.diagnosticBtn.SetText("No diagnostics")
		doc.diagnosticBtn.Disable()
	} else {
		doc.diagnosticBtn.SetText(fmt.Sprintf("%d error(s), %d warning(s)", errors, warnings))
		doc.diagnosticBtn.Enable()
	}

	if draft, ok := w.app.sourceDrafts.Get(doc.editor); ok {
		doc.draftStatus.SetText("Draft pending — Guided is unchanged")
		if draft.ParseErr == "" && errors == 0 {
			doc.applyBtn.Enable()
		} else {
			doc.applyBtn.Disable()
		}
	} else {
		doc.draftStatus.SetText("Applied model")
		doc.applyBtn.Disable()
	}
}

func (w *AdvancedWorkspace) refreshGuidedBanner() {
	if w == nil || w.guidedBanner == nil || w.app == nil {
		return
	}
	draft, ok := w.app.sourceDrafts.Get(w.active)
	if !ok {
		w.guidedBanner.Hide()
		return
	}
	message := "Draft pending — click Apply to update Guided fields."
	if draft.ParseErr != "" {
		message = "Draft pending with errors — Guided fields remain at the last applied state."
	}
	w.guidedLabel.SetText(message)
	if draft.ParseErr == "" {
		w.guidedApply.Enable()
	} else {
		w.guidedApply.Disable()
	}
	w.guidedBanner.Show()
}

func (w *AdvancedWorkspace) applyActiveDraft() {
	w.applyDraft(w.documents[w.active])
}

func (w *AdvancedWorkspace) applyDraft(doc *advancedDocumentView) {
	if doc == nil {
		return
	}
	draft, ok := w.app.sourceDrafts.Get(doc.editor)
	if !ok {
		w.app.updateStatus("Apply: no pending source draft")
		return
	}
	if draft.ParseErr != "" {
		dialog.ShowError(fmt.Errorf("source didn't parse: %s", draft.ParseErr), w.app.mainWindow)
		return
	}
	se, ok := doc.editor.(SessionEditor)
	if !ok {
		err := fmt.Errorf("apply not supported for %T", doc.editor)
		w.retainApplyError(doc, draft, err)
		dialog.ShowError(err, w.app.mainWindow)
		return
	}
	if err := se.ApplySourceText(draft.Text); err != nil {
		w.retainApplyError(doc, draft, err)
		dialog.ShowError(err, w.app.mainWindow)
		return
	}
	if !w.app.sourceDrafts.DeleteIfMatch(doc.editor, draft) {
		w.syncDocumentText(doc)
	}
	w.app.updateStatus("Applied source edits to Guided fields")
}

func (w *AdvancedWorkspace) retainApplyError(doc *advancedDocumentView, draft SourceDraft, err error) {
	if doc == nil || err == nil {
		return
	}
	draft.ParseErr = err.Error()
	offset := doc.code.CursorOffset()
	line, column := sourceLineColumn(doc.code.Text(), offset)
	doc.analysis.Diagnostics = append(doc.analysis.Diagnostics, SourceDiagnostic{
		Range: SourceRange{Start: offset, End: offset}, Line: line, Column: column,
		Severity: SourceError, Message: err.Error(),
	})
	doc.analyzed = true
	doc.code.SetAnalysis(doc.analysis)
	doc.publishing = true
	w.app.sourceDrafts.Set(doc.editor, draft)
	doc.publishing = false
	w.refreshDocumentStatus(doc)
	w.refreshGuidedBanner()
}

func (w *AdvancedWorkspace) confirmRevert(doc *advancedDocumentView) {
	if doc == nil || !w.app.sourceDrafts.Has(doc.editor) {
		return
	}
	dialog.ShowConfirm("Discard source draft", "Discard this document's unapplied source text and return to the Guided model?", func(confirmed bool) {
		if confirmed {
			w.app.sourceDrafts.Delete(doc.editor)
			w.app.updateStatus("Discarded the pending source draft")
		}
	}, w.app.mainWindow)
}

func (w *AdvancedWorkspace) showDiagnostics(doc *advancedDocumentView) {
	if doc == nil || len(doc.analysis.Diagnostics) == 0 {
		return
	}
	items := make([]fyne.CanvasObject, 0, len(doc.analysis.Diagnostics))
	var d dialog.Dialog
	for i := range doc.analysis.Diagnostics {
		diagnostic := doc.analysis.Diagnostics[i]
		label := fmt.Sprintf("%s — line %d, col %d: %s", strings.ToUpper(string(diagnostic.Severity)), diagnostic.Line, diagnostic.Column, diagnostic.Message)
		button := widget.NewButton(label, func() {
			doc.code.SelectRange(diagnostic.Range.Start, diagnostic.Range.End)
			w.focusCode(doc)
			if d != nil {
				d.Hide()
			}
		})
		button.Alignment = widget.ButtonAlignLeading
		button.Importance = widget.LowImportance
		items = append(items, button)
	}
	scroll := container.NewVScroll(container.NewVBox(items...))
	scroll.SetMinSize(fyne.NewSize(680, 360))
	d = dialog.NewCustom("Source diagnostics", "Close", scroll, w.app.mainWindow)
	d.Show()
}

func (w *AdvancedWorkspace) focusCode(doc *advancedDocumentView) {
	if doc != nil && w.app != nil && w.app.mainWindow != nil {
		w.app.mainWindow.Canvas().Focus(doc.code)
	}
}

func (w *AdvancedWorkspace) codeIsFocused(doc *advancedDocumentView) bool {
	return doc != nil && w.app != nil && w.app.mainWindow != nil && w.app.mainWindow.Canvas().Focused() == doc.code
}

func (w *AdvancedWorkspace) UndoFocused() bool {
	doc := w.documents[w.active]
	if !w.codeIsFocused(doc) {
		return false
	}
	return doc.code.Undo()
}

func (w *AdvancedWorkspace) RedoFocused() bool {
	doc := w.documents[w.active]
	if !w.codeIsFocused(doc) {
		return false
	}
	return doc.code.Redo()
}

func (w *AdvancedWorkspace) undoSessionFallback(doc *advancedDocumentView) bool {
	if doc == nil || w.app.sourceDrafts.Has(doc.editor) {
		return false
	}
	se, ok := doc.editor.(SessionEditor)
	return ok && se.Undo()
}

func (w *AdvancedWorkspace) redoSessionFallback(doc *advancedDocumentView) bool {
	if doc == nil || w.app.sourceDrafts.Has(doc.editor) {
		return false
	}
	se, ok := doc.editor.(SessionEditor)
	return ok && se.Redo()
}

func (w *AdvancedWorkspace) RefreshPalette() error {
	if w == nil || w.app == nil {
		return nil
	}
	palette, err := configuredSourcePalette(w.app.config.SyntaxTheme, w.app.config.SyntaxColors)
	if err != nil {
		return err
	}
	w.palette = palette
	w.hasPalette = true
	for _, doc := range w.documents {
		doc.code.SetPalette(palette)
	}
	return nil
}

func configuredSourcePalette(name string, overrides map[string]string) (SourcePalette, error) {
	palettes := SourcePalettes()
	if len(palettes) == 0 {
		return SourcePalette{}, fmt.Errorf("no source palettes are available")
	}
	if name == "" {
		name = palettes[0].Name
	}
	base, ok := SourcePaletteByName(name)
	if !ok {
		return SourcePalette{}, fmt.Errorf("unknown source palette %q", name)
	}
	return SourcePaletteWithOverrides(base, overrides)
}

func sourceFormatForEditor(ed Editor) SourceFormat {
	switch ed.(type) {
	case *MBCHEditor:
		return SourceFormatMBCH
	case *SABEditor:
		return SourceFormatSAB
	case *VEHEditor:
		return SourceFormatVEH
	case *SiegeEditor:
		return SourceFormatSiege
	}
	if ed == nil {
		return ""
	}
	switch strings.ToLower(filepath.Ext(ed.GetCurrentPath())) {
	case string(SourceFormatMBCH):
		return SourceFormatMBCH
	case string(SourceFormatSAB):
		return SourceFormatSAB
	case string(SourceFormatVEH):
		return SourceFormatVEH
	case string(SourceFormatSiege):
		return SourceFormatSiege
	case string(SourceFormatMBTC):
		return SourceFormatMBTC
	default:
		return ""
	}
}

func matchingBracketStatus(pairs []SourceBracketPair, caret int, text string) (string, bool) {
	positions := []int{caret, caret - 1}
	for _, position := range positions {
		for _, pair := range pairs {
			other := -1
			switch position {
			case pair.Open:
				other = pair.Close
			case pair.Close:
				other = pair.Open
			}
			if other >= 0 {
				line, column := sourceLineColumn(text, other)
				return fmt.Sprintf("Bracket matches line %d, col %d", line, column), true
			}
		}
	}
	return "", false
}
