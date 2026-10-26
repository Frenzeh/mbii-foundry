package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestSlotEntryLongValueKeepsCaretInViewport(t *testing.T) {
	useEditorTestApp(t)
	entry := NewSlotEntry(&wheelRoute{})
	entry.SetText("models/short.skin")
	shortMinWidth := entry.MinSize().Width
	original := "models/players/" + strings.Repeat("café_β-gyjp.[variant]/", 40) + "model_default.skin"
	entry.SetText(original)
	end := utf8.RuneCountInString(original)

	window := fyne.CurrentApp().NewWindow("slot entry containment")
	window.SetPadded(false)
	window.SetContent(entry)
	window.Show()
	defer window.Close()
	window.Canvas().Focus(entry)

	for _, width := range []float32{900, 360, 120, 600, 360} {
		window.Resize(fyne.NewSize(width, 80))
		window.Canvas().Capture()
		if entry.Text != original {
			t.Fatalf("resize to %.0f changed the original entry text", width)
		}
		if entry.MinSize().Width > shortMinWidth+1 {
			t.Fatalf("long text expanded the minimum width from %v to %v", shortMinWidth, entry.MinSize().Width)
		}
		if entry.Size().Width > width+1 {
			t.Fatalf("entry escaped its %.0fpx allocation: %v", width, entry.Size())
		}

		entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
		assertSlotEntryState(t, entry, original, end)
		entry.TypedRune('界')
		assertSlotEntryState(t, entry, original+"界", end+1)
		entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
		assertSlotEntryState(t, entry, original, end)
		entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyHome})
		assertSlotEntryState(t, entry, original, 0)
		entry.TypedRune('Ω')
		assertSlotEntryState(t, entry, "Ω"+original, 1)
		entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
		assertSlotEntryState(t, entry, original, 0)
	}
}

func TestFormSectionsReflowWithoutLosingEntryText(t *testing.T) {
	useEditorTestApp(t)
	originals := []string{"001.2300", "models/players/café_β-gyjp/model_default.skin", "-000.0400"}
	wheel := &wheelRoute{}
	var sections []fyne.CanvasObject
	var fields []struct {
		entry *slotEntry
		text  string
	}
	for i := range 5 {
		form := widget.NewForm()
		form.Orientation = widget.Vertical
		for j := range i%len(originals) + 1 {
			entry := NewSlotEntry(wheel)
			entry.SetText(originals[j])
			form.Append("Stored value", entry)
			fields = append(fields, struct {
				entry *slotEntry
				text  string
			}{entry, originals[j]})
		}
		sections = append(sections, NewFormSection(
			"Movement and model paths — glyphs (gyjp), punctuation, and precision",
			strings.Repeat("Preserve spaces, punctuation, Unicode café/β, and every decimal digit. ", i%2+1),
			form,
		))
	}
	columns := newFormColumns(3, sections...)
	viewport := container.NewVScroll(columns)
	wheel.target = viewport
	window := fyne.CurrentApp().NewWindow("form section containment")
	window.SetPadded(false)
	window.SetContent(viewport)
	window.Show()
	defer window.Close()

	for _, state := range []struct {
		width   float32
		columns int
	}{{360, 1}, {600, 2}, {900, 3}, {600, 2}, {360, 1}} {
		window.Resize(fyne.NewSize(state.width, 720))
		window.Canvas().Capture()
		if columns.Size().Width > state.width+1 || columns.MinSize().Width > state.width+1 {
			t.Fatalf("form groups pin a %.0fpx viewport wider: size=%v minimum=%v", state.width, columns.Size(), columns.MinSize())
		}
		assertFormObjectsContained(t, columns, sections)
		firstRow := 0
		for _, section := range sections {
			if section.Position().Y < 1 {
				firstRow++
			}
			// Inspect painted heading/hint lines and form captions, but stop at
			// entries: their long text intentionally lives in a clipped viewport.
			objects := formContainmentObjects(section, func(object fyne.CanvasObject) bool {
				if _, ok := object.(*slotEntry); ok {
					return true
				}
				text, ok := object.(*canvas.Text)
				return ok && text.Text != ""
			})
			assertFormObjectsContained(t, section, objects)
		}
		if firstRow != state.columns {
			t.Fatalf("%.0fpx viewport has %d groups in the first row, want %d", state.width, firstRow, state.columns)
		}
		for _, field := range fields {
			if field.entry.Text != field.text {
				t.Fatalf("reflow changed stored text: got %q, want %q", field.entry.Text, field.text)
			}
		}

		entry := fields[0].entry
		window.Canvas().Focus(entry)
		entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
		entry.TypedRune('5')
		assertSlotEntryState(t, entry, originals[0]+"5", len(originals[0])+1)
		entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
		assertSlotEntryState(t, entry, originals[0], len(originals[0]))
	}
}

func TestCharacterSummaryReservesWrappedHeightBeforeProfile(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	editor := NewMBCHEditor(nil)
	t.Cleanup(editor.Session().flushPending)
	name := strings.Repeat("Long character name, g j p q / punctuation. ", 5)
	if err := editor.ApplySourceText("ClassInfo\n{\nname \"" + name + "\"\nMBClass MB_CLASS_JEDI\nmaxhealth 100\napMultiplier 1.125\n}\n"); err != nil {
		t.Fatal(err)
	}
	window := fyne.CurrentApp().NewWindow("character header containment")
	window.SetPadded(false)
	window.SetContent(editor.GetContent())
	window.Show()
	defer window.Close()

	for _, width := range []float32{900, 360, 600, 360} {
		before, dirty := editor.CurrentSource(), editor.IsDirty()
		window.Resize(fyne.NewSize(width, 720))
		window.Canvas().Capture()
		if editor.GetContent().MinSize().Width > width+1 {
			t.Fatalf("character tabs prevent shrinking to %.0f: minimum=%v", width, editor.GetContent().MinSize())
		}
		text := formContainmentObjects(editor.summary, func(object fyne.CanvasObject) bool {
			line, ok := object.(*canvas.Text)
			return ok && line.Text != ""
		})
		assertFormObjectsContained(t, editor.summary, text)
		assertFormObjectsContained(t, editor.GetContent(), []fyne.CanvasObject{editor.summary, editor.nameEntry})
		if editor.CurrentSource() != before || editor.IsDirty() != dirty {
			t.Fatal("header reflow changed the document")
		}

		window.Canvas().Focus(editor.nameEntry)
		editor.nameEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
		editor.nameEntry.TypedRune('界')
		if !strings.Contains(editor.CurrentSource(), name+"界") {
			t.Fatal("profile input beneath the wrapped summary is not editable")
		}
		editor.nameEntry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
		if editor.CurrentSource() != before {
			t.Fatal("editing the name changed unrelated character content")
		}
	}
}

func TestDeveloperFieldsReflowWithUnknownValues(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	character := parsers.NewMBCHCharacter()
	value := strings.Repeat("EAS_FUTURE_", 20) + "界"
	character.ExtraFields = map[string]string{"special1": value, "futureField": "keep"}
	editor := &MBCHEditor{
		character:       character,
		loading:         true,
		devFieldEntries: make(map[string]*widget.Entry),
	}
	section := editor.buildDevFieldsCard()
	editor.populateDevFieldsFromCharacter()
	window := fyne.CurrentApp().NewWindow("developer field containment")
	window.SetPadded(false)
	window.SetContent(container.NewVScroll(section))
	window.Show()
	defer window.Close()

	for _, width := range []float32{900, 360, 600, 360} {
		window.Resize(fyne.NewSize(width, 720))
		window.Canvas().Capture()
		if section.MinSize().Width > width+1 || section.Size().Width > width+1 {
			t.Fatalf("developer fields exceed %.0f: size=%v minimum=%v", width, section.Size(), section.MinSize())
		}
		objects := formContainmentObjects(section, func(object fyne.CanvasObject) bool {
			switch object.(type) {
			case *widget.Entry, *widget.SelectEntry:
				return true
			}
			text, ok := object.(*canvas.Text)
			return ok && text.Text != ""
		})
		assertFormObjectsContained(t, section, objects)
		if editor.devFieldEntries["special1"].Text != value ||
			character.ExtraFields["special1"] != value || character.ExtraFields["futureField"] != "keep" {
			t.Fatal("developer form reflow changed unknown stored values")
		}
	}
}

func TestDescriptionColorToolbarWrapsAndPreservesTextActions(t *testing.T) {
	useEditorTestApp(t)
	entry := NewValidatedEntry(func(string) error { return nil })
	const colored = " \t^1café^2 / ^3β ^4[gyjp]^5 .001.2300^6\n^7^8^9^0literal ^x  "
	const plain = " \tcafé / β [gyjp] .001.2300\n^0literal ^x  "
	generated := 0
	toolbar := NewQ3ColorToolbar(entry, func() { generated++ })
	actions := formContainmentObjects(toolbar, func(object fyne.CanvasObject) bool {
		switch object.(type) {
		case *TooltipButton, *widget.Button:
			return true
		}
		return false
	})
	window := fyne.CurrentApp().NewWindow("description toolbar containment")
	window.SetPadded(false)
	window.SetContent(toolbar)
	window.Show()
	defer window.Close()

	for _, width := range []float32{360, 900, 360} {
		window.Resize(fyne.NewSize(width, 240))
		window.Canvas().Capture()
		assertFormObjectsContained(t, toolbar, actions)
		swatches := make(map[string]bool)
		clears, autoActions := 0, 0
		for i, action := range actions {
			entry.SetText(colored)
			before := generated
			position, _ := objectPositionWithin(toolbar, action)
			center := position.Add(fyne.NewPos(action.Size().Width/2, action.Size().Height/2))
			test.TapCanvas(window.Canvas(), center)
			if i == len(actions)-1 && generated != before+1 {
				t.Fatal("last toolbar action is not callable at its visible position")
			}
			switch {
			case generated == before+1:
				autoActions++
				if entry.Text != colored {
					t.Fatal("auto-generate dispatch changed text outside its callback")
				}
			case generated != before:
				t.Fatal("one tap dispatched auto-generate more than once")
			case entry.Text == plain:
				clears++
			default:
				matched := false
				for _, color := range Q3ColorLabels {
					if entry.Text == colored+"^"+color.Code {
						swatches[color.Code] = true
						matched = true
						break
					}
				}
				if !matched {
					t.Fatalf("toolbar tap lost or rewrote description text: %q", entry.Text)
				}
			}
		}
		if len(swatches) != len(Q3ColorLabels) || clears != 1 || autoActions != 1 {
			t.Fatalf("%.0fpx toolbar actions are unreachable: swatches=%d clear=%d auto=%d", width, len(swatches), clears, autoActions)
		}
	}
}

func assertSlotEntryState(t *testing.T, entry *slotEntry, text string, offset int) {
	t.Helper()
	if entry.Text != text || entry.CursorTextOffset() != offset {
		t.Fatalf("entry state: text=%q cursor=%d; want text=%q cursor=%d", entry.Text, entry.CursorTextOffset(), text, offset)
	}
	var viewport *container.Scroll
	for _, object := range test.WidgetRenderer(entry).Objects() {
		if scroll, ok := object.(*container.Scroll); ok {
			viewport = scroll
			break
		}
	}
	if viewport == nil || viewport.Content == nil {
		t.Fatal("entry has no internal text viewport")
	}
	assertFormObjectsContained(t, entry, []fyne.CanvasObject{viewport})
	for _, object := range childObjects(viewport.Content) {
		if caret, ok := object.(*canvas.Rectangle); ok {
			assertFormObjectsContained(t, viewport, []fyne.CanvasObject{caret})
			return
		}
	}
	t.Fatal("focused entry has no rendered caret")
}

func formContainmentObjects(root fyne.CanvasObject, accept func(fyne.CanvasObject) bool) []fyne.CanvasObject {
	var objects []fyne.CanvasObject
	var visit func(fyne.CanvasObject)
	visit = func(object fyne.CanvasObject) {
		if !object.Visible() {
			return
		}
		if accept(object) {
			objects = append(objects, object)
			return
		}
		for _, child := range childObjects(object) {
			visit(child)
		}
	}
	visit(root)
	return objects
}

func assertFormObjectsContained(t *testing.T, root fyne.CanvasObject, objects []fyne.CanvasObject) {
	t.Helper()
	if len(objects) == 0 {
		t.Fatalf("no visible controls or text in %T", root)
	}
	for i, object := range objects {
		position, found := objectPositionWithin(root, object)
		size, bounds := object.Size(), root.Size()
		if !found || !object.Visible() || size.Width <= 0 || size.Height <= 0 ||
			position.X < -1 || position.Y < -1 ||
			position.X+size.Width > bounds.Width+1 || position.Y+size.Height > bounds.Height+1 {
			t.Fatalf("%T escapes %T: position=%v size=%v bounds=%v visible=%v", object, root, position, size, bounds, object.Visible())
		}
		for _, previous := range objects[:i] {
			other, _ := objectPositionWithin(root, previous)
			overlapX := fyne.Min(position.X+size.Width, other.X+previous.Size().Width) - fyne.Max(position.X, other.X)
			overlapY := fyne.Min(position.Y+size.Height, other.Y+previous.Size().Height) - fyne.Max(position.Y, other.Y)
			if overlapX > 1 && overlapY > 1 {
				t.Fatalf("%T at %v overlaps %T at %v inside %T", object, position, previous, other, root)
			}
		}
	}
}
