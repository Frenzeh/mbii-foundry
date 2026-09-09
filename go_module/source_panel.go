package main

// Live source panel. Has two display modes:
//
//   * Highlighted view (default): widget.RichText with per-token
//     colors from syntax_highlight.go. Read-only, scrollable,
//     refreshes on form change so it always matches the would-save
//     bytes. The "notepad++ feel" for MBII data files.
//
//   * Edit mode (toggle via the Edit button): plain monospace
//     widget.Entry. Typing is captured here; a userDirty flag
//     pauses the auto-refresh so nothing clobbers in-progress edits.
//     Apply runs the edited text back through the editor's LoadFile
//     parser (reusing existing format logic); Revert discards.
//
// Fyne v2.7's Entry doesn't expose per-character coloring, so we
// can't do "edit + highlight simultaneously" inline — but swapping
// views on demand is a clean, understandable split.

import (
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

type SourcePanel struct {
	app *App

	header    *widget.Label
	byteCount *widget.Label

	// Dual display surfaces. Only one is visible at a time; content
	// area Stack swaps which renders. highlighted is the default.
	highlighted *widget.RichText
	editor      *widget.Entry
	viewHost    *fyne.Container // Stack swapping between highlighted & editor

	editToggle *TooltipButton // "Edit" / "View" toggle
	applyBtn   *TooltipButton
	revertBtn  *TooltipButton

	// Live-parse validation feedback — only visible in edit mode.
	// Updates on every keystroke with either a success tick or the
	// parser's error message. Apply is disabled when the parse fails.
	validationIcon *widget.Icon
	validationMsg  *widget.Label
	validationRow  *fyne.Container

	container *fyne.Container

	provider  SourceProvider
	editorRef Editor

	// drafts is the app-wide per-document draft store, shared with
	// every mirror panel. It retains each document's unapplied source
	// text + diagnostics across tab / mode / pop-out switches.
	drafts *SourceDraftStore

	// lastParseErr is the live parse diagnostics for the current edit
	// text; stored with the draft so re-selecting a tab restores the
	// exact validation state.
	lastParseErr string

	// srcRemove unsubscribes this panel from the current editor's
	// source-changed push notifications; srcSubEditor is the editor
	// the subscription belongs to.
	srcRemove    func()
	srcSubEditor Editor

	tickerStop chan struct{}
	tickerDone chan struct{}
	closeOnce  sync.Once
	closed     atomic.Bool

	// Legacy SourceProvider subscriptions have setter semantics instead
	// of a remove callback. Track the provider so Close can detach it.
	legacySource SourceProvider

	// True while the user has pending edits in the Entry. While true,
	// the auto-refresh from form → source is paused.
	userDirty bool

	// settingText distinguishes programmatic mirror/render synchronization
	// from user typing. Programmatic Entry.SetText callbacks must never
	// create, replace, or delete the shared document draft.
	settingText bool

	// onPopOut, if set, opens this panel's content in a new window
	// for dual-monitor workflows. The pop-out button next to the
	// collapse arrow invokes it; App wires this to a method that
	// creates a fresh SourcePanel mirror tracking the same active
	// editor as the primary.
	onPopOut func()

	// inEditMode reflects which view is on top of the Stack.
	inEditMode bool

	// Last source string rendered from the form. Used to detect
	// changes for ticker refreshes + to revert edits.
	lastRenderedSource string
}

func NewSourcePanel(a *App) *SourcePanel {
	sp := &SourcePanel{
		app:        a,
		tickerStop: make(chan struct{}),
		tickerDone: make(chan struct{}),
	}
	// Every panel owned by an App uses exactly the same draft store.
	// Initialize it here for lightweight/test Apps rather than silently
	// creating a private store that mirrors cannot see.
	if a != nil {
		if a.sourceDrafts == nil {
			a.sourceDrafts = NewSourceDraftStore()
		}
		sp.drafts = a.sourceDrafts
	} else {
		sp.drafts = NewSourceDraftStore()
	}
	// Safety-net ticker for editors that don't invoke
	// SetOnSourceChanged on every mutation. Must fyne.Do for UI
	// thread safety.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		defer close(sp.tickerDone)
		for {
			select {
			case <-sp.tickerStop:
				return
			case <-ticker.C:
				if sp.closed.Load() || sp.provider == nil {
					continue
				}
				fyne.Do(func() {
					if sp.closed.Load() || sp.provider == nil {
						return
					}
					sp.refreshFromProvider()
				})
			}
		}
	}()

	sp.header = widget.NewLabelWithStyle("SOURCE", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sp.byteCount = widget.NewLabel("")
	sp.byteCount.TextStyle = fyne.TextStyle{Monospace: true}

	// Highlighted view (default). TextWrapOff: long lines extend past
	// the right edge and the bi-directional Scroll wrapper provides
	// horizontal scrolling. MBCH source has pipe-separated mega-lists
	// (forcepowers, attributes) that read worse when wrapped — even
	// at pipe boundaries the eye loses the column. IDE convention is
	// no-wrap + scroll, so we follow it.
	sp.highlighted = widget.NewRichText()
	sp.highlighted.Wrapping = fyne.TextWrapOff
	sp.setPlaceholder("Select a file to see its live source here.")
	sp.editor = widget.NewMultiLineEntry()
	sp.editor.Wrapping = fyne.TextWrapOff
	sp.editor.OnChanged = func(s string) {
		if sp.provider == nil || sp.settingText {
			return
		}
		// Entry changes not initiated by render/mirror synchronization are
		// user work. Retain even text that happens to equal the rendered
		// source; only Apply, Revert, or confirmed Discard may consume it.
		sp.userDirty = true
		sp.updateByteCount(s)
		// Keep the colored preview live-in-sync with what the user types.
		sp.highlighted.Segments = highlightedSegments(s)
		sp.highlighted.Refresh()
		sp.validateEdits(s)
		if sp.editorRef != nil {
			sp.drafts.Set(sp.editorRef, SourceDraft{
				Text:     s,
				ParseErr: sp.lastParseErr,
				EditMode: sp.inEditMode,
			})
		}
	}

	// Stack wraps both views; Show/Hide flips which is visible.
	// SetMinSize on each inner Scroll enforces a floor of 240px so
	// the HSplit rail can't drag the source pane so narrow that
	// TextWrapBreak degrades to one-character-per-line (the "vertical
	// alphabet soup" case). 240px is about 30 monospace chars at
	// body size — enough for most tokens to lay out meaningfully.
	highlightScroll := container.NewScroll(sp.highlighted)
	highlightScroll.SetMinSize(fyne.NewSize(240, 0))
	editorScroll := container.NewScroll(sp.editor)
	editorScroll.SetMinSize(fyne.NewSize(240, 0))
	sp.viewHost = container.NewStack(highlightScroll, editorScroll)
	sp.viewHost.Objects[1].Hide() // editor hidden initially

	copyBtn := NewTooltipButton("", theme.ContentCopyIcon(), func() {
		text := sp.currentText()
		if text == "" {
			return
		}
		a.mainWindow.Clipboard().SetContent(text)
	}, "Copy source to clipboard")
	copyBtn.Importance = widget.LowImportance

	sp.editToggle = NewTooltipButton("Edit", theme.DocumentCreateIcon(), func() {
		sp.toggleEditMode()
	}, "Switch between read-only highlighted view and editable text")
	sp.editToggle.Importance = widget.LowImportance

	sp.applyBtn = NewTooltipButton("Apply", theme.ConfirmIcon(), func() {
		sp.applyEdits()
	}, "Parse the edited source and push it back to the form")
	sp.applyBtn.Importance = widget.HighImportance
	sp.applyBtn.Hide()

	sp.revertBtn = NewTooltipButton("Revert", theme.ContentUndoIcon(), func() {
		sp.revertEdits()
	}, "Discard in-progress edits; restore from form state")
	sp.revertBtn.Importance = widget.LowImportance
	sp.revertBtn.Hide()

	collapseBtn := NewTooltipButton("", PanelCollapseRightIcon(), func() {
		a.toggleSourcePanel()
	}, "Collapse source panel")
	collapseBtn.Importance = widget.LowImportance

	popOutBtn := NewTooltipButton("", theme.WindowMaximizeIcon(), func() {
		if sp.onPopOut != nil {
			sp.onPopOut()
		}
	}, "Pop out source panel into its own window (collapses rail)")
	popOutBtn.Importance = widget.LowImportance

	// Validation indicator — visible only while editing and only when
	// there is useful parser feedback.
	sp.validationIcon = widget.NewIcon(theme.ConfirmIcon())
	sp.validationMsg = widget.NewLabel("")
	sp.validationMsg.TextStyle = fyne.TextStyle{Monospace: true}
	sp.validationMsg.Wrapping = fyne.TextWrapWord
	validationRow := container.NewHBox(sp.validationIcon, sp.validationMsg)
	validationRow.Hide()
	sp.validationRow = validationRow

	headerActions := container.NewHBox(copyBtn, popOutBtn, collapseBtn)
	headerRow := container.NewBorder(nil, nil, sp.header, headerActions, sp.byteCount)
	actionRow := container.NewHBox(sp.editToggle, sp.applyBtn, sp.revertBtn)

	// Source is utility chrome, not a promotional card. A flat padded
	// header with one divider keeps it distinct without competing with
	// the editor or repeating the info panel's card treatment.
	chrome := container.NewPadded(container.NewVBox(headerRow, actionRow, validationRow))
	topBlock := container.NewVBox(chrome, NewAccentRule())

	sp.container = container.NewBorder(topBlock, nil, nil, nil, sp.viewHost)
	return sp
}

// GetContent returns the panel's root widget.
func (sp *SourcePanel) GetContent() fyne.CanvasObject { return sp.container }

// Close releases the panel's background refresh loop and source subscription.
// It is safe to call repeatedly (including both reattach and window-close
// paths); a disposed panel cannot be attached to another editor.
func (sp *SourcePanel) Close() {
	if sp == nil {
		return
	}
	sp.closeOnce.Do(func() {
		sp.closed.Store(true)
		close(sp.tickerStop)
		<-sp.tickerDone
		sp.storeDraftFor(sp.editorRef)
		sp.unsubscribeSource()
		sp.provider = nil
		sp.editorRef = nil
	})
}

// SetOnPopOut wires the pop-out button. Pass nil to disable (used
// by mirror panels so they don't spawn their own pop-outs).
func (sp *SourcePanel) SetOnPopOut(cb func()) { sp.onPopOut = cb }

// SetActiveEditor tells the panel which editor to track. Safe to
// pass nil.
func (sp *SourcePanel) SetActiveEditor(ed Editor) {
	if sp == nil || sp.closed.Load() {
		return
	}
	// 1. Park the outgoing editor's draft.
	sp.storeDraftFor(sp.editorRef)
	sp.unsubscribeSource()

	sp.provider = nil
	sp.editorRef = ed

	// 2. Assign the provider + subscribe BEFORE restoring the draft:
	// applyDraft and refreshFromProvider read provider-dependent state
	// (byte counter, placeholders) and need the surface wired up.
	if provider, ok := ed.(SourceProvider); ok {
		sp.provider = provider
		if lp, ok := ed.(SourceListenerProvider); ok {
			sp.srcSubEditor = ed
			sp.srcRemove = lp.AddSourceListener(func() {
				fyne.Do(func() {
					if !sp.closed.Load() {
						sp.refreshFromProvider()
					}
				})
			})
		} else {
			// Legacy fallback: retain the provider so Close can clear
			// its setter-based callback.
			sp.legacySource = provider
			provider.SetOnSourceChanged(func() {
				fyne.Do(func() {
					if !sp.closed.Load() {
						sp.refreshFromProvider()
					}
				})
			})
		}
	}

	// 3. Restore the incoming editor's draft, or reset document-local
	// mode/diagnostics so the outgoing document cannot leak into it.
	if d, ok := sp.drafts.Get(ed); ok {
		sp.applyDraft(d)
	} else {
		sp.userDirty = false
		sp.lastParseErr = ""
		sp.setEditMode(false)
	}
	sp.refreshFromProvider()
}

// storeDraftFor parks the current edit text (if any) into the shared
// store for ed. It only ever WRITES: passive surfaces (mirrors, tab
// switches with no pending edits) must never erase another surface's
// retained draft. Drafts are deleted exclusively by a successful
// Apply, an explicit Revert, or a confirmed Discard.
func (sp *SourcePanel) storeDraftFor(ed Editor) {
	if ed == nil || sp.editorRef != ed || !sp.userDirty {
		return
	}
	sp.drafts.Set(ed, SourceDraft{
		Text:     sp.editor.Text,
		ParseErr: sp.lastParseErr,
		EditMode: sp.inEditMode,
	})
}

// applyDraft restores a stored draft into the edit surface: text,
// live-parse diagnostics, byte count and edit mode (in BOTH
// directions — a doc saved without edit mode must not inherit the
// previous doc's editor view).
func (sp *SourcePanel) applyDraft(d SourceDraft) {
	sp.userDirty = true
	sp.lastParseErr = d.ParseErr
	sp.setEditorText(d.Text)
	sp.updateByteCount(d.Text)
	sp.highlighted.Segments = highlightedSegments(d.Text)
	sp.highlighted.Refresh()
	// Reassess through the active parser so recovered/older drafts never
	// become "valid" merely because their stored diagnostic was blank.
	sp.validateEdits(d.Text)
	if d.EditMode != sp.inEditMode {
		sp.setEditMode(d.EditMode)
	}
	d.ParseErr = sp.lastParseErr
	sp.drafts.Set(sp.editorRef, d)
}

func (sp *SourcePanel) setEditorText(text string) {
	sp.settingText = true
	sp.editor.SetText(text)
	sp.settingText = false
}

// unsubscribeSource drops this panel's push subscription, if any.
func (sp *SourcePanel) unsubscribeSource() {
	if sp.srcRemove != nil {
		sp.srcRemove()
		sp.srcRemove = nil
	}
	if sp.legacySource != nil {
		sp.legacySource.SetOnSourceChanged(nil)
		sp.legacySource = nil
	}
	sp.srcSubEditor = nil
}

// refreshFromProvider re-renders from the provider's current source,
// unless the user has pending edits.
func (sp *SourcePanel) refreshFromProvider() {
	if sp.provider == nil {
		sp.setPlaceholder("Select a file to see its live source here.")
		sp.setEditorText("")
		sp.lastRenderedSource = ""
		sp.userDirty = false
		sp.byteCount.SetText("")
		return
	}

	// The shared store is authoritative across the docked panel and all
	// pop-outs. Pull newer mirror edits, and notice when another surface
	// explicitly consumed the draft so this passive mirror can resume.
	if d, ok := sp.drafts.Get(sp.editorRef); ok {
		if !sp.userDirty || sp.editor.Text != d.Text ||
			sp.lastParseErr != d.ParseErr || sp.inEditMode != d.EditMode {
			sp.applyDraft(d)
		}
		return
	}
	if sp.userDirty {
		sp.userDirty = false
		sp.lastParseErr = ""
	}
	sp.renderFromProvider(sp.provider.GenerateSource())
}

// renderFromProvider pushes src into both views and updates
// bookkeeping. NOTE: never delete the stored draft here. Passive
// refreshes (ticker, form-driven pushes) must not erase a retained
// draft — only a successful Apply, an explicit Revert or a confirmed
// Discard may consume one.
func (sp *SourcePanel) renderFromProvider(src string) {
	if src == "" {
		sp.setPlaceholder("(no source yet — the editor is empty)")
		sp.setEditorText("")
	} else {
		// No-wrap rendering: source goes into the highlighter as-is
		// and overflows horizontally inside the bi-directional Scroll.
		sp.highlighted.Segments = highlightedSegments(src)
		sp.highlighted.Refresh()
		// Only overwrite the Entry if we're not in edit mode — editing
		// should feel uninterrupted even if the form is still ticking.
		if !sp.inEditMode {
			sp.setEditorText(src)
		}
	}
	sp.lastRenderedSource = src
	sp.userDirty = false
	sp.lastParseErr = ""
	sp.updateByteCount(src)
}

// setPlaceholder shows a muted italic message in the highlighted
// view (used when no editor is active).
func (sp *SourcePanel) setPlaceholder(msg string) {
	sp.highlighted.Segments = []widget.RichTextSegment{
		&widget.TextSegment{
			Text: msg,
			Style: widget.RichTextStyle{
				Inline:    true,
				ColorName: theme.ColorNamePlaceHolder,
				TextStyle: fyne.TextStyle{Italic: true},
			},
		},
	}
	sp.highlighted.Refresh()
}

// currentText returns whichever view's text is "live" — the Entry
// text if the user is editing, otherwise the last-rendered source.
func (sp *SourcePanel) currentText() string {
	if sp.userDirty || sp.inEditMode {
		return sp.editor.Text
	}
	return sp.lastRenderedSource
}

// toggleEditMode flips between highlighted view and edit mode.
func (sp *SourcePanel) toggleEditMode() {
	sp.setEditMode(!sp.inEditMode)
}

func (sp *SourcePanel) setEditMode(on bool) {
	entering := on && !sp.inEditMode
	sp.inEditMode = on
	if on {
		// Sync editor text from last-rendered source on entry — but
		// never clobber a restored draft that's already sitting in the
		// Entry (tab-switch restore path).
		if entering && !sp.userDirty {
			sp.setEditorText(sp.lastRenderedSource)
		}
		sp.viewHost.Objects[0].Hide()
		sp.viewHost.Objects[1].Show()
		sp.editToggle.SetIcon(theme.VisibilityIcon())
		sp.editToggle.SetText("View")
		sp.applyBtn.Show()
		sp.revertBtn.Show()
		sp.validationRow.Show()
		sp.validateEdits(sp.editor.Text)
	} else {
		sp.viewHost.Objects[1].Hide()
		sp.viewHost.Objects[0].Show()
		sp.editToggle.SetIcon(theme.DocumentCreateIcon())
		sp.editToggle.SetText("Edit")
		sp.applyBtn.Hide()
		sp.revertBtn.Hide()
		sp.validationRow.Hide()
	}
	// Edit/view is document-local state whenever a draft exists.
	if d, ok := sp.drafts.Get(sp.editorRef); ok {
		d.EditMode = on
		d.ParseErr = sp.lastParseErr
		sp.drafts.Set(sp.editorRef, d)
	}
	sp.viewHost.Refresh()
	if sp.validationRow != nil {
		sp.validationRow.Refresh()
	}
}

// validateEdits runs the appropriate parser against the current edit
// text and updates the validation indicator. Also toggles Apply's
// enabled state — we don't want the user committing unparseable text.
func (sp *SourcePanel) validateEdits(src string) {
	if sp.validationIcon == nil || sp.validationMsg == nil {
		return
	}
	err := sp.parseForActiveEditor(src)
	if err == nil {
		sp.lastParseErr = ""
		sp.validationIcon.SetResource(theme.ConfirmIcon())
		sp.validationMsg.SetText("Parses cleanly — Apply will update the form.")
		sp.applyBtn.Enable()
		return
	}
	// Clip overly long errors so the panel doesn't blow up vertically.
	msg := err.Error()
	if len(msg) > 240 {
		msg = msg[:240] + "…"
	}
	sp.lastParseErr = msg
	sp.validationIcon.SetResource(theme.ErrorIcon())
	sp.validationMsg.SetText("Parse error: " + msg)
	sp.applyBtn.Disable()
}

func validateSourceStructure(src string) error {
	if strings.TrimSpace(src) == "" {
		return fmt.Errorf("source is empty")
	}
	tokens, err := parsers.Lex(src)
	if err != nil {
		return err
	}
	depth := 0
	topLevelBlocks := 0
	for _, token := range tokens {
		switch token.Type {
		case parsers.TokenBraceOpen:
			if depth == 0 {
				topLevelBlocks++
			}
			depth++
		case parsers.TokenBraceClose:
			depth--
			if depth < 0 {
				return fmt.Errorf("unexpected closing brace")
			}
		}
	}
	if depth != 0 {
		return fmt.Errorf("unbalanced braces")
	}
	if topLevelBlocks == 0 {
		return fmt.Errorf("source contains no definition block")
	}
	if diagnostics := parsers.LexDiagnostics(src); len(diagnostics) > 0 {
		return fmt.Errorf("%s", strings.Join(diagnostics, "; "))
	}
	return nil
}

// parseSourceForEditor validates a draft against the actual editor
// contract. Unsupported/nil editors are errors: the absence of a
// parser is never evidence that arbitrary text is valid.
func parseSourceForEditor(ed Editor, src string) error {
	if err := validateSourceStructure(src); err != nil {
		return err
	}
	switch e := ed.(type) {
	case *MBCHEditor:
		_, err := parsers.ParseMBCH(src)
		return err
	case *SABEditor:
		_, err := parsers.ParseSABDefinition(src, e.SelectedDefinition())
		return err
	case *VEHEditor:
		_, err := parsers.ParseVEHDefinition(src, e.SelectedDefinition())
		return err
	case *SiegeEditor:
		_, err := parsers.ParseSiege(src)
		return err
	case nil:
		return fmt.Errorf("no active editor")
	default:
		return fmt.Errorf("source parsing is unsupported for %T", ed)
	}
}

func (sp *SourcePanel) parseForActiveEditor(src string) error {
	return parseSourceForEditor(sp.editorRef, src)
}

// updateByteCount writes the byte total. MBCH warnings use the same
// engine-faithful parser assessment as validation; other formats get a
// plain count and are never assigned MBCH's file or block capacities.
func (sp *SourcePanel) updateByteCount(src string) {
	n := len(src)
	if sp.provider == nil || n == 0 {
		sp.byteCount.SetText("")
		return
	}
	if !sp.editorIsMBCH() {
		sp.byteCount.SetText(fmt.Sprintf("%d bytes", n))
		return
	}

	bs, err := parsers.AssessMBCHSourceBuffers(src)
	if err != nil {
		sp.byteCount.SetText(fmt.Sprintf("%d bytes (capacity assessment failed)", n))
		return
	}
	switch {
	case bs.TotalFileBytes >= parsers.MBCHMaxFileBytes:
		sp.byteCount.SetText(fmt.Sprintf("%d / %d bytes — file limit reached", bs.TotalFileBytes, parsers.MBCHMaxFileBytes-1))
	case bs.ClassInfoBytes > parsers.ClassInfoMaxPayload:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes — ClassInfo %d / %d", n, bs.ClassInfoBytes, parsers.ClassInfoMaxPayload))
	case maxInt(bs.WeaponInfoBytes) > parsers.WeaponInfoMaxPayload:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes — WeaponInfo %d / %d", n, maxInt(bs.WeaponInfoBytes), parsers.WeaponInfoMaxPayload))
	case maxInt(bs.ForceInfoBytes) > parsers.ForceInfoMaxPayload:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes — ForceInfo %d / %d", n, maxInt(bs.ForceInfoBytes), parsers.ForceInfoMaxPayload))
	case bs.MaxPairedValueBytes > parsers.PairedValueMaxPayload:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes — value %d / %d", n, bs.MaxPairedValueBytes, parsers.PairedValueMaxPayload))
	case bs.TotalFileBytes > parsers.MBCHMaxFileBytes-500:
		sp.byteCount.SetText(fmt.Sprintf("%d / %d bytes (near file limit)", bs.TotalFileBytes, parsers.MBCHMaxFileBytes-1))
	default:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes", n))
	}
}

func maxInt(values []int) int {
	max := 0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

// editorIsMBCH reports whether the tracked document is a .mbch file
// (by path extension, falling back to the editor type for untitled
// documents).
func (sp *SourcePanel) editorIsMBCH() bool {
	if sp.editorRef == nil {
		return false
	}
	if p := sp.editorRef.GetCurrentPath(); p != "" {
		return strings.ToLower(filepath.Ext(p)) == ".mbch"
	}
	_, ok := sp.editorRef.(*MBCHEditor)
	return ok
}

// applyEdits parses the Entry's text and pushes the result back to
// the editor in-memory — no temp files, no LoadFile. This preserves
// document identity (path + immutable baseline) and a failed parse
// leaves the editor untouched, so the text stays editable and
// correctable. The applied state becomes one undo step.
func (sp *SourcePanel) applyEdits() {
	if sp.editorRef == nil {
		return
	}
	if !sp.userDirty {
		if sp.app != nil {
			sp.app.updateStatus("Apply: no source changes to push back to the form")
		}
		return
	}
	src := sp.editor.Text
	draft := SourceDraft{Text: src, ParseErr: sp.lastParseErr, EditMode: sp.inEditMode}
	// Validate first — applyBtn should already be disabled on parse
	// failure, but belt-and-suspenders. A failed parse retains the
	// text in the panel for correction.
	if err := sp.parseForActiveEditor(src); err != nil {
		dialog.ShowError(fmt.Errorf("source didn't parse: %w", err), sp.app.mainWindow)
		return
	}
	se, ok := sp.editorRef.(SessionEditor)
	if !ok {
		dialog.ShowError(fmt.Errorf("apply not supported for this editor type"), sp.app.mainWindow)
		return
	}
	if err := se.ApplySourceText(src); err != nil {
		dialog.ShowError(err, sp.app.mainWindow)
		return
	}
	// Applied — consume exactly the version this surface applied. If a
	// mirror published newer text, refreshFromProvider pulls that draft.
	sp.userDirty = false
	sp.lastParseErr = ""
	sp.drafts.DeleteIfMatch(sp.editorRef, draft)
	sp.refreshFromProvider()
	sp.app.updateStatus("Applied source edits to the form")
}

// revertEdits drops in-progress edits and re-syncs from the form.
// This is an explicit user-initiated discard of the draft.
func (sp *SourcePanel) revertEdits() {
	if !sp.userDirty {
		return
	}
	current := SourceDraft{
		Text:     sp.editor.Text,
		ParseErr: sp.lastParseErr,
		EditMode: sp.inEditMode,
	}
	sp.userDirty = false
	sp.lastParseErr = ""
	if sp.editorRef != nil {
		sp.drafts.DeleteIfMatch(sp.editorRef, current)
	}
	sp.refreshFromProvider()
}

// DraftPendingFor reports whether ed has unapplied source work —
// valid or not — retained in the panel's draft state. Close and quit
// guards count this as unsaved changes.
func (sp *SourcePanel) DraftPendingFor(ed Editor) bool {
	if sp == nil || ed == nil {
		return false
	}
	if ed == sp.editorRef && sp.userDirty {
		return true
	}
	return sp.drafts.Has(ed)
}

// DiscardDraft drops ed's retained draft (used on confirmed tab
// close after the user agreed to discard).
func (sp *SourcePanel) DiscardDraft(ed Editor) {
	if sp == nil || ed == nil {
		return
	}
	sp.drafts.Delete(ed)
	if sp.editorRef == ed {
		sp.userDirty = false
		sp.lastParseErr = ""
	}
}
