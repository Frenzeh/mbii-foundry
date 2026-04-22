package main

// Live source preview panel. Shows the file-as-it-would-be-saved for
// whichever editor tab is currently active, updating on every change.
// Inspired by kitsu's MBCH editor UX: edit via form fields on the left,
// see the actual .mbch bytes rendering on the right in real time.
//
// Togglable via the "source" button in the main toolbar. Hidden state
// persists across sessions (SourcePanelVisible in AppConfig).

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type SourcePanel struct {
	app *App

	header    *widget.Label
	byteCount *widget.Label
	content   *widget.RichText
	container *fyne.Container

	// Currently-tracked editor — nil when the active tab doesn't
	// implement SourceProvider (e.g. the Home/welcome tab).
	provider SourceProvider
}

func NewSourcePanel(a *App) *SourcePanel {
	sp := &SourcePanel{app: a}

	sp.header = widget.NewLabelWithStyle("Source", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	sp.byteCount = widget.NewLabel("")
	sp.byteCount.TextStyle = fyne.TextStyle{Italic: true, Monospace: true}

	sp.content = widget.NewRichTextFromMarkdown("*Select a file to see its live source here.*")
	sp.content.Wrapping = fyne.TextWrapOff

	// Copy-source-to-clipboard button. Useful for dropping the
	// current file into Discord pastes or quick-testing in a text
	// editor without saving.
	copyBtn := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		if sp.provider == nil {
			return
		}
		src := sp.provider.GenerateSource()
		if src == "" {
			return
		}
		a.mainWindow.Clipboard().SetContent(src)
	})
	copyBtn.Importance = widget.LowImportance

	topRow := container.NewBorder(nil, nil, sp.header, container.NewHBox(sp.byteCount, copyBtn))
	sp.container = container.NewBorder(topRow, nil, nil, nil, container.NewScroll(sp.content))
	return sp
}

// GetContent returns the Fyne object the main layout should place in
// the right pane.
func (sp *SourcePanel) GetContent() fyne.CanvasObject { return sp.container }

// SetActiveEditor tells the panel which editor to track. Safe to pass
// nil (e.g. Home tab); the panel just shows a placeholder until a real
// editor takes focus.
func (sp *SourcePanel) SetActiveEditor(ed Editor) {
	sp.provider = nil
	if provider, ok := ed.(SourceProvider); ok {
		sp.provider = provider
		provider.SetOnSourceChanged(func() {
			fyne.Do(sp.refresh)
		})
	}
	sp.refresh()
}

// refresh regenerates the displayed source from the current provider.
// Safe to call from any goroutine via fyne.Do.
func (sp *SourcePanel) refresh() {
	if sp.provider == nil {
		sp.content.ParseMarkdown("*Select a file to see its live source here.*")
		sp.byteCount.SetText("")
		return
	}
	src := sp.provider.GenerateSource()
	if src == "" {
		sp.content.ParseMarkdown("*(no source yet — the editor is empty)*")
		sp.byteCount.SetText("")
		return
	}
	// MBCH files have a hard 8192-byte limit. Flag it visibly.
	n := len(src)
	switch {
	case n > 8192:
		sp.byteCount.SetText(fmt.Sprintf("%d / 8192 ⚠ over limit", n))
	case n > 7500:
		sp.byteCount.SetText(fmt.Sprintf("%d / 8192 (near limit)", n))
	default:
		sp.byteCount.SetText(fmt.Sprintf("%d bytes", n))
	}
	sp.content.ParseMarkdown("```\n" + src + "\n```")
}
