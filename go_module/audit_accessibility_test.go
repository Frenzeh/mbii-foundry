package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func auditFocusable(t *testing.T, object fyne.CanvasObject) fyne.Focusable {
	t.Helper()
	focusable, ok := object.(fyne.Focusable)
	if !ok {
		t.Fatalf("%T cannot receive keyboard focus", object)
	}
	return focusable
}

func auditStroke(rect *canvas.Rectangle) [5]uint32 {
	var result [5]uint32
	if rect.StrokeColor != nil {
		result[0], result[1], result[2], result[3] = rect.StrokeColor.RGBA()
	}
	result[4] = uint32(rect.StrokeWidth * 100)
	return result
}

var auditActivations = []struct {
	name string
	run  func(fyne.Focusable)
}{
	{"Return", func(f fyne.Focusable) { f.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn}) }},
	{"Enter", func(f fyne.Focusable) { f.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnter}) }},
	{"Space", func(f fyne.Focusable) { f.TypedRune(' ') }},
}

func TestAuditActivityKeyboardActivation(t *testing.T) {
	useEditorTestApp(t)
	for _, activation := range auditActivations {
		t.Run(activation.name, func(t *testing.T) {
			selected, toggled := 0, 0
			item := &ActivityItem{ID: "files", Label: "Files", Tooltip: "Browse files", Icon: theme.FolderIcon()}
			header := NewSidebarHeader([]*ActivityItem{item}, func(got *ActivityItem) {
				if got != item {
					t.Fatalf("activated activity = %p, want %p", got, item)
				}
				selected++
			}, func() { toggled++ })
			fyneTest.WidgetRenderer(header)
			pill := header.pills[item.ID]
			focusable := auditFocusable(t, pill)
			focusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			focusable.TypedRune('x')
			if selected != 0 || header.ActiveID() != "" {
				t.Fatal("unrelated keyboard input selected an activity")
			}
			activation.run(focusable)
			if header.ActiveID() != item.ID || selected != 1 || toggled != 0 {
				t.Fatalf("activity activation: active=%q, selected=%d, toggled=%d", header.ActiveID(), selected, toggled)
			}
			activation.run(focusable)
			if selected != 1 {
				t.Fatal("reselecting the active activity fired a duplicate callback")
			}
			activation.run(auditFocusable(t, header.collapseBtn))
			if toggled != 1 || selected != 1 || header.ActiveID() != item.ID {
				t.Fatal("collapse activation did not preserve the active activity")
			}
		})
	}
}

func TestAuditActivityVisibleFocusPreservesLayoutAndSelection(t *testing.T) {
	useEditorTestApp(t)
	calls := 0
	header := NewSidebarHeader([]*ActivityItem{{ID: "files", Label: "Files", Tooltip: "Browse files", Icon: theme.FolderIcon()}},
		func(*ActivityItem) { calls++ }, func() { calls++ })
	fyneTest.WidgetRenderer(header)
	pill := header.pills["files"]
	for _, target := range []*activityPill{pill, header.collapseBtn} {
		fyneTest.WidgetRenderer(target)
		focusable := auditFocusable(t, target)
		for _, active := range []bool{false, true} {
			target.setActive(active)
			resting := auditStroke(target.bg)
			focusable.FocusGained()
			if got := auditStroke(target.bg); got == resting || got[3] == 0 || got[4] == 0 {
				t.Fatalf("active=%v: %T has no visible focus stroke", active, target)
			}
			focusable.FocusLost()
			if auditStroke(target.bg) != resting {
				t.Fatal("blur did not restore the existing activity styling")
			}
		}
	}
	if calls != 0 || header.ActiveID() != "" {
		t.Fatal("focus changed activity selection or toggled the sidebar")
	}
	layout := &activityPillLayout{pill: pill}
	layout.Layout(nil, fyne.NewSize(64, 36))
	if pill.label.Visible() {
		t.Fatal("compact activity unexpectedly expanded its label")
	}
	layout.Layout(nil, fyne.NewSize(200, 36))
	if !pill.label.Visible() || pill.item.Tooltip == "" {
		t.Fatal("expanded label or existing tooltip was lost")
	}
}

func TestAuditClassKeyboardActivationAndVisibleFocus(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	for _, activation := range auditActivations {
		t.Run(activation.name, func(t *testing.T) {
			changes := 0
			changedID := ""
			picker := NewClassIconPicker(func(id string) { changes++; changedID = id })
			if len(picker.cards) < 2 {
				t.Fatal("class picker needs two selectable classes")
			}
			card := picker.cards[1]
			fyneTest.WidgetRenderer(card)
			focusable := auditFocusable(t, card)
			for _, selected := range []string{picker.cards[0].def.ID, card.def.ID} {
				picker.SetSelected(selected)
				resting := auditStroke(card.border)
				focusable.FocusGained()
				if got := auditStroke(card.border); got == resting || got[3] == 0 || got[4] == 0 {
					t.Fatalf("selected=%q: class card has no distinct visible focus", selected)
				}
				focusable.FocusLost()
				if auditStroke(card.border) != resting || picker.Selected() != selected || changes != 0 {
					t.Fatal("focus changed the selected class or failed to restore its styling")
				}
			}
			picker.SetSelected(picker.cards[0].def.ID)
			focusable.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
			focusable.TypedRune('x')
			if changes != 0 || picker.Selected() != picker.cards[0].def.ID {
				t.Fatal("unrelated keyboard input selected a class")
			}
			activation.run(focusable)
			if changes != 1 || changedID != card.def.ID || picker.Selected() != card.def.ID {
				t.Fatal("keyboard activation did not publish the selected class")
			}
			activation.run(focusable)
			fyneTest.Tap(card)
			if changes != 1 {
				t.Fatal("keyboard/pointer reselection fired duplicate class mutations")
			}
		})
	}
}

func auditFormItems(object fyne.CanvasObject) []*widget.FormItem {
	var items []*widget.FormItem
	switch object := object.(type) {
	case *fyne.Container:
		for _, child := range object.Objects {
			items = append(items, auditFormItems(child)...)
		}
	case *widget.Card:
		items = append(items, auditFormItems(object.Content)...)
	case *widget.Form:
		items = append(items, object.Items...)
	}
	return items
}

func TestAuditSkinRGBLabelledChannelBindings(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	app := &App{mainWindow: fyne.CurrentApp().NewWindow("skin channel regression")}
	editor := NewMBCHEditor(app)
	editor.character.ExtraFields = map[string]string{
		"model_2": "synthetic", "userRGB_2": "1",
		"customred_2": "0.11", "customgreen_2": "0.22", "customblue_2": "0.33",
		"customred_1": "0.99",
	}
	skins := NewSkinVariantsEditor(editor)
	skins.Refresh()
	items := auditFormItems(skins.GetContent())
	channels := map[string]string{"customred_2": "0.11", "customgreen_2": "0.22", "customblue_2": "0.33"}
	labels := map[string]bool{}
	for key, initial := range channels {
		var entry *widget.Entry
		for _, item := range items {
			candidate, ok := item.Widget.(*widget.Entry)
			if !ok || candidate.Text != initial {
				continue
			}
			label := strings.TrimSpace(item.Text)
			if label == "" || labels[label] {
				t.Fatalf("channel %s lacks its own retained form label", key)
			}
			labels[label] = true
			entry = candidate
			break
		}
		if entry == nil {
			t.Fatalf("channel %s is not exposed through a labelled form item", key)
		}
		entry.SetText("0.45")
		for otherKey, want := range channels {
			if otherKey == key {
				want = "0.45"
			}
			if got := editor.character.ExtraFields[otherKey]; got != want {
				t.Fatalf("editing %s changed %s to %q, want %q", key, otherKey, got, want)
			}
		}
		entry.SetText(initial)
	}
	if editor.character.ExtraFields["customred_1"] != "0.99" || editor.character.ExtraFields["userRGB_2"] != "1" {
		t.Fatal("editing a channel changed another variant or its RGB enablement")
	}
}
