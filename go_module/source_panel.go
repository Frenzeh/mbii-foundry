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
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
	// Fallback refresh timer — catches editors that don't call
	// SetOnSourceChanged from every UI field. Event-driven refresh
	// (via markDirty() hooks) gets priority for instant updates;
	// this ticker is the safety net for editors that lack those
	// hooks (500ms matches kitsu's MBCH editor cadence).
	//
	// All the actual work must happen inside fyne.Do: GenerateSource
	// reads Fyne widgets which is only safe on the UI thread. The
	// previous version called GenerateSource from the goroutine
	// directly and crashed the app on first MBCH open once the
	// ticker fired.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		var last string
		for range ticker.C {
			if sp.provider == nil {
				continue
			}
			fyne.Do(func() {
				if sp.provider == nil {
					return
				}
				cur := sp.provider.GenerateSource()
				if cur != last {
					last = cur
					sp.refresh()
				}
			})
		}
	}()

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

	// Collapse button — mirrors the sidebar's push-arrow design so
	// both panels use the same visual language for "minimize me."
	// Panel-collapse-right says "push to the right edge."
	collapseBtn := widget.NewButtonWithIcon("", PanelCollapseRightIcon(), func() {
		a.toggleSourcePanel()
	})
	collapseBtn.Importance = widget.LowImportance

	// Thin accent rule under the header, matching the sidebar header
	// treatment so both panels read as part of the same design.
	rule := canvas.NewRectangle(tintWithAlpha(CurrentThemeColor, 90))
	rule.SetMinSize(fyne.NewSize(0, 2))

	headerRow := container.NewBorder(nil, nil, sp.header, container.NewHBox(sp.byteCount, copyBtn, collapseBtn))
	topBlock := container.NewVBox(headerRow, rule)
	sp.container = container.NewBorder(topBlock, nil, nil, nil, container.NewScroll(sp.content))
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
