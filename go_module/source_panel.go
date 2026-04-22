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
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

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

	editToggle *widget.Button // "Edit" / "View" toggle
	applyBtn   *widget.Button
	revertBtn  *widget.Button

	// Live-parse validation feedback — only visible in edit mode.
	// Updates on every keystroke with either a success tick or the
	// parser's error message. Apply is disabled when the parse fails.
	validationIcon *widget.Icon
	validationMsg  *widget.Label
	validationRow  *fyne.Container

	container *fyne.Container

	provider SourceProvider
	editorRef Editor

	// True while the user has pending edits in the Entry. While true,
	// the auto-refresh from form → source is paused.
	userDirty bool

	// inEditMode reflects which view is on top of the Stack.
	inEditMode bool

	// Last source string rendered from the form. Used to detect
	// changes for ticker refreshes + to revert edits.
	lastRenderedSource string
}

func NewSourcePanel(a *App) *SourcePanel {
	sp := &SourcePanel{app: a}

	// Safety-net ticker for editors that don't invoke
	// SetOnSourceChanged on every mutation. Must fyne.Do for UI
	// thread safety.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if sp.provider == nil {
				continue
			}
			fyne.Do(func() {
				if sp.provider == nil || sp.userDirty {
					return
				}
				cur := sp.provider.GenerateSource()
				if cur != sp.lastRenderedSource {
					sp.renderFromProvider(cur)
				}
			})
		}
	}()

	sp.header = widget.NewLabelWithStyle("Source", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sp.byteCount = widget.NewLabel("")
	sp.byteCount.TextStyle = fyne.TextStyle{Italic: true, Monospace: true}

	// Highlighted view (default).
	sp.highlighted = widget.NewRichText()
	sp.highlighted.Wrapping = fyne.TextWrapOff
	sp.setPlaceholder("Select a file to see its live source here.")

	// Edit view (hidden until toggled).
	sp.editor = widget.NewMultiLineEntry()
	sp.editor.TextStyle = fyne.TextStyle{Monospace: true}
	sp.editor.Wrapping = fyne.TextWrapOff
	sp.editor.OnChanged = func(s string) {
		if sp.provider == nil {
			return
		}
		sp.userDirty = s != sp.lastRenderedSource
		sp.updateByteCount(s)
		// Keep the colored preview live-in-sync with what the user
		// types — swapping back to View then shows the latest edits
		// already highlighted, and anyone peeking at the preview
		// during edit mode sees the right colors.
		sp.highlighted.Segments = highlightedSegments(s)
		sp.highlighted.Refresh()
		sp.validateEdits(s)
	}

	// Stack wraps both views; Show/Hide flips which is visible.
	sp.viewHost = container.NewStack(
		container.NewScroll(sp.highlighted),
		container.NewScroll(sp.editor),
	)
	sp.viewHost.Objects[1].Hide() // editor hidden initially

	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		text := sp.currentText()
		if text == "" {
			return
		}
		a.mainWindow.Clipboard().SetContent(text)
	})
	copyBtn.Importance = widget.LowImportance

	sp.editToggle = widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		sp.toggleEditMode()
	})
	sp.editToggle.Importance = widget.LowImportance

	sp.applyBtn = widget.NewButtonWithIcon("Apply", theme.ConfirmIcon(), func() {
		sp.applyEdits()
	})
	sp.applyBtn.Importance = widget.HighImportance
	sp.applyBtn.Hide()

	sp.revertBtn = widget.NewButtonWithIcon("Revert", theme.ContentUndoIcon(), func() {
		sp.revertEdits()
	})
	sp.revertBtn.Importance = widget.LowImportance
	sp.revertBtn.Hide()

	collapseBtn := widget.NewButtonWithIcon("", PanelCollapseRightIcon(), func() {
		a.toggleSourcePanel()
	})
	collapseBtn.Importance = widget.LowImportance

	rule := canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 90))
	rule.SetMinSize(fyne.NewSize(0, 2))

	// Validation indicator — confirm icon + message. Hidden outside
	// edit mode; visible only when there's something to say.
	sp.validationIcon = widget.NewIcon(theme.ConfirmIcon())
	sp.validationMsg = widget.NewLabel("")
	sp.validationMsg.TextStyle = fyne.TextStyle{Monospace: true}
	sp.validationMsg.Wrapping = fyne.TextWrapWord
	validationRow := container.NewHBox(sp.validationIcon, sp.validationMsg)
	validationRow.Hide()
	// Save a reference so we can toggle visibility with edit mode.
	sp.validationRow = validationRow

	headerRow := container.NewBorder(nil, nil, sp.header, container.NewHBox(sp.byteCount, copyBtn, collapseBtn))
	actionRow := container.NewHBox(sp.editToggle, sp.applyBtn, sp.revertBtn)
	topBlock := container.NewVBox(headerRow, actionRow, validationRow, rule)

	sp.container = container.NewBorder(topBlock, nil, nil, nil, sp.viewHost)
	return sp
}

// GetContent returns the panel's root widget.
func (sp *SourcePanel) GetContent() fyne.CanvasObject { return sp.container }

// SetActiveEditor tells the panel which editor to track. Safe to
// pass nil.
func (sp *SourcePanel) SetActiveEditor(ed Editor) {
	sp.provider = nil
	sp.editorRef = ed
	sp.userDirty = false
	// Force edit mode off whenever the active editor changes — we
	// don't want someone's half-typed edits to bleed into the next
	// file's session.
	if sp.inEditMode {
		sp.setEditMode(false)
	}
	if provider, ok := ed.(SourceProvider); ok {
		sp.provider = provider
		provider.SetOnSourceChanged(func() {
			fyne.Do(sp.refreshFromProvider)
		})
	}
	sp.refreshFromProvider()
}

// refreshFromProvider re-renders from the provider's current source,
// unless the user has pending edits.
func (sp *SourcePanel) refreshFromProvider() {
	if sp.provider == nil {
		sp.setPlaceholder("Select a file to see its live source here.")
		sp.editor.SetText("")
		sp.lastRenderedSource = ""
		sp.byteCount.SetText("")
		return
	}
	if sp.userDirty {
		return
	}
	sp.renderFromProvider(sp.provider.GenerateSource())
}

// renderFromProvider pushes src into both views and updates bookkeeping.
func (sp *SourcePanel) renderFromProvider(src string) {
	if src == "" {
		sp.setPlaceholder("(no source yet — the editor is empty)")
		sp.editor.SetText("")
	} else {
		sp.highlighted.Segments = highlightedSegments(src)
		sp.highlighted.Refresh()
		// Only overwrite the Entry if we're not in edit mode — editing
		// should feel uninterrupted even if the form is still ticking.
		if !sp.inEditMode {
			sp.editor.SetText(src)
		}
	}
	sp.lastRenderedSource = src
	sp.userDirty = false
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
	if sp.inEditMode {
		return sp.editor.Text
	}
	return sp.lastRenderedSource
}

// toggleEditMode flips between highlighted view and edit mode.
func (sp *SourcePanel) toggleEditMode() {
	sp.setEditMode(!sp.inEditMode)
}

func (sp *SourcePanel) setEditMode(on bool) {
	sp.inEditMode = on
	if on {
		// Sync editor text from last-rendered source on entry.
		sp.editor.SetText(sp.lastRenderedSource)
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
		sp.validationIcon.SetResource(theme.ConfirmIcon())
		sp.validationMsg.SetText("Parses cleanly — Apply will update the form.")
		sp.applyBtn.Enable()
		return
	}
	sp.validationIcon.SetResource(theme.ErrorIcon())
	// Clip overly long errors so the panel doesn't blow up vertically.
	msg := err.Error()
	if len(msg) > 240 {
		msg = msg[:240] + "…"
	}
	sp.validationMsg.SetText("Parse error: " + msg)
	sp.applyBtn.Disable()
}

// parseForActiveEditor dispatches to the correct parser based on the
// active editor's file extension. Parse functions are pure — they
// don't mutate any editor state, making them safe to run on every
// keystroke for live validation.
func (sp *SourcePanel) parseForActiveEditor(src string) error {
	if sp.editorRef == nil {
		return nil // nothing to parse against
	}
	ext := ""
	if p := sp.editorRef.GetCurrentPath(); p != "" {
		ext = strings.ToLower(filepath.Ext(p))
	}
	// Fall back to probing the editor type if no file path is set
	// yet (new unsaved file).
	if ext == "" {
		switch sp.editorRef.(type) {
		case *MBCHEditor:
			ext = ".mbch"
		case *SABEditor:
			ext = ".sab"
		case *VEHEditor:
			ext = ".veh"
		case *SiegeEditor:
			ext = ".siege"
		}
	}
	switch ext {
	case ".mbch":
		_, err := parsers.ParseMBCH(src)
		return err
	case ".sab":
		_, err := parsers.ParseSAB(src)
		return err
	case ".veh":
		_, err := parsers.ParseVEH(src)
		return err
	case ".siege":
		_, err := parsers.ParseSiege(src)
		return err
	}
	return nil
}

// updateByteCount writes the byte total with MBCH-cap warnings.
func (sp *SourcePanel) updateByteCount(src string) {
	n := len(src)
	switch {
	case sp.provider == nil, n == 0:
		sp.byteCount.SetText("")
	case n > 8192:
		sp.byteCount.SetText(fmt.Sprintf("%d / 8192 ⚠ over limit", n))
	case n > 7500:
		sp.byteCount.SetText(fmt.Sprintf("%d / 8192 (near limit)", n))
	default:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes", n))
	}
}

// applyEdits writes the Entry's text to a temp file and reuses the
// editor's LoadFile parser to push changes back to the form.
func (sp *SourcePanel) applyEdits() {
	if sp.editorRef == nil || !sp.userDirty {
		return
	}
	ext := ".txt"
	if p := sp.editorRef.GetCurrentPath(); p != "" {
		ext = filepath.Ext(p)
	}
	tmp, err := os.CreateTemp("", "foundry-apply-*"+ext)
	if err != nil {
		dialog.ShowError(fmt.Errorf("couldn't create temp file: %w", err), sp.app.mainWindow)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(sp.editor.Text); err != nil {
		tmp.Close()
		dialog.ShowError(fmt.Errorf("couldn't write temp file: %w", err), sp.app.mainWindow)
		return
	}
	tmp.Close()

	if err := sp.editorRef.LoadFile(tmpPath); err != nil {
		dialog.ShowError(fmt.Errorf("source didn't parse: %w", err), sp.app.mainWindow)
		return
	}
	if original := sp.app.currentEditorPath(); original != "" {
		sp.editorRef.SetCurrentPath(original)
	}
	sp.userDirty = false
	sp.refreshFromProvider()
	sp.app.updateStatus("Applied source edits to the form")
}

// revertEdits drops in-progress edits and re-syncs from the form.
func (sp *SourcePanel) revertEdits() {
	if !sp.userDirty {
		return
	}
	sp.userDirty = false
	sp.refreshFromProvider()
}
