package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestClassFlagRowsWrapWithoutOverlapAndToggle(t *testing.T) {
	useEditorTestApp(t)

	previous := LoadedClassFlags
	t.Cleanup(func() { LoadedClassFlags = previous })
	longID := "CFL_" + strings.Repeat("VERY_LONG_TECHNICAL_IDENTIFIER_", 8)
	otherID := "CFL_EXISTING_FLAG"
	longName := strings.Repeat("Readable friendly class flag name ", 9)
	longDescription := strings.Repeat("This complete description must remain visible and wrap into content-sized lines without colliding with the next flag row. ", 12)
	LoadedClassFlags = []ClassFlagDef{
		{ID: longID, Name: longName, Description: longDescription},
		{ID: otherID, Name: "Existing friendly flag", Description: strings.Repeat("A second long description verifies that adjacent cards remain separated. ", 8)},
	}

	for _, width := range []float32{320, 600, 900} {
		t.Run(fmt.Sprintf("width_%.0f", width), func(t *testing.T) {
			character := parsers.NewMBCHCharacter()
			character.ClassFlags = otherID
			editor := &MBCHEditor{character: character, sourceSubs: &sourceListeners{}}
			editor.session = NewDocumentSession(
				func() string { return character.ClassFlags },
				func(string) error { return nil },
			)
			editor.session.SetOnDirtyChange(editor.setDirtyState)
			editor.session.Reset()

			flagsEditor := NewWeaponFlagsEditor(editor)
			rows := findClassFlagRows(t, flagsEditor.GetContent())
			layoutClassFlagRows(rows, width)
			if len(rows.Objects) != len(LoadedClassFlags) {
				t.Fatalf("got %d flag rows, want %d", len(rows.Objects), len(LoadedClassFlags))
			}

			for i, row := range rows.Objects {
				if row.Size().Width != width {
					t.Fatalf("row %d width = %.1f, want %.1f", i, row.Size().Width, width)
				}
				if i > 0 {
					previousRow := rows.Objects[i-1]
					if previousRow.Position().Y+previousRow.Size().Height > row.Position().Y {
						t.Fatalf("rows %d and %d overlap: previous=%v/%v next=%v/%v", i-1, i,
							previousRow.Position(), previousRow.Size(), row.Position(), row.Size())
					}
				}
			}

			first := rows.Objects[0]
			name := findClassFlagLabel(t, first, longName)
			id := findClassFlagLabel(t, first, longID)
			description := findClassFlagLabel(t, first, longDescription)
			assertClassFlagLabelBelow(t, first, name, id, "friendly name", "technical ID")
			assertClassFlagLabelBelow(t, first, id, description, "technical ID", "description")
			assertClassFlagRenderedLinesFit(t, id, longID, width)
			assertClassFlagRenderedLinesFit(t, description, longDescription, width)

			firstTap, ok := first.(fyne.Tappable)
			if !ok {
				t.Fatalf("class flag row is not tappable: %T", first)
			}
			test.Tap(firstTap)
			wantFlags := []string{longID, otherID}
			sort.Strings(wantFlags)
			if got, want := character.ClassFlags, strings.Join(wantFlags, "|"); got != want {
				t.Fatalf("tap added flags %q, want %q", got, want)
			}
			if !editor.IsDirty() {
				t.Fatal("toggling a class flag did not mark the editor dirty")
			}

			rebuiltRows := findClassFlagRows(t, flagsEditor.GetContent())
			layoutClassFlagRows(rebuiltRows, width)
			rebuiltTap, ok := rebuiltRows.Objects[0].(fyne.Tappable)
			if !ok {
				t.Fatalf("rebuilt class flag row is not tappable: %T", rebuiltRows.Objects[0])
			}
			test.Tap(rebuiltTap)
			if character.ClassFlags != otherID {
				t.Fatalf("second tap left flags %q, want %q", character.ClassFlags, otherID)
			}
		})
	}
}

func findClassFlagRows(t *testing.T, root fyne.CanvasObject) *fyne.Container {
	t.Helper()
	if container, ok := root.(*fyne.Container); ok {
		if _, ok := container.Layout.(*classFlagRowsLayout); ok {
			return container
		}
	}
	for _, child := range classFlagChildren(root) {
		if rows := findClassFlagRowsOrNil(child); rows != nil {
			return rows
		}
	}
	t.Fatal("class flag rows not found")
	return nil
}

func findClassFlagRowsOrNil(root fyne.CanvasObject) *fyne.Container {
	if container, ok := root.(*fyne.Container); ok {
		if _, ok := container.Layout.(*classFlagRowsLayout); ok {
			return container
		}
	}
	for _, child := range classFlagChildren(root) {
		if rows := findClassFlagRowsOrNil(child); rows != nil {
			return rows
		}
	}
	return nil
}

func layoutClassFlagRows(rows *fyne.Container, width float32) {
	rows.Resize(fyne.NewSize(width, 10000))
	height := rows.MinSize().Height
	rows.Resize(fyne.NewSize(width, height))
}

func findClassFlagLabel(t *testing.T, root fyne.CanvasObject, text string) *widget.Label {
	t.Helper()
	if label, ok := root.(*widget.Label); ok && label.Text == text {
		return label
	}
	for _, child := range classFlagChildren(root) {
		if label := findClassFlagLabelOrNil(child, text); label != nil {
			return label
		}
	}
	t.Fatalf("label %q not found", text)
	return nil
}

func findClassFlagLabelOrNil(root fyne.CanvasObject, text string) *widget.Label {
	if label, ok := root.(*widget.Label); ok && label.Text == text {
		return label
	}
	for _, child := range classFlagChildren(root) {
		if label := findClassFlagLabelOrNil(child, text); label != nil {
			return label
		}
	}
	return nil
}

func assertClassFlagLabelBelow(t *testing.T, row fyne.CanvasObject, upper, lower *widget.Label, upperName, lowerName string) {
	t.Helper()
	upperPosition, ok := classFlagObjectPosition(row, upper)
	if !ok {
		t.Fatalf("%s is outside its flag row", upperName)
	}
	lowerPosition, ok := classFlagObjectPosition(row, lower)
	if !ok {
		t.Fatalf("%s is outside its flag row", lowerName)
	}
	if upperPosition.Y+upper.Size().Height > lowerPosition.Y {
		t.Fatalf("%s overlaps %s: upper=%v/%v lower=%v/%v", upperName, lowerName,
			upperPosition, upper.Size(), lowerPosition, lower.Size())
	}
	for name, bounds := range map[string]struct {
		position fyne.Position
		size     fyne.Size
	}{
		upperName: {position: upperPosition, size: upper.Size()},
		lowerName: {position: lowerPosition, size: lower.Size()},
	} {
		if bounds.position.X < 0 || bounds.position.X+bounds.size.Width > row.Size().Width+1 {
			t.Fatalf("%s overflows its flag row horizontally: position=%v size=%v row=%v",
				name, bounds.position, bounds.size, row.Size())
		}
	}
	if lowerPosition.Y+lower.Size().Height > row.Size().Height+1 {
		t.Fatalf("%s overflows its flag row: position=%v size=%v row=%v", lowerName,
			lowerPosition, lower.Size(), row.Size())
	}
}

func assertClassFlagRenderedLinesFit(t *testing.T, label *widget.Label, completeText string, width float32) {
	t.Helper()
	texts := classFlagCanvasTexts(label)
	if len(texts) < 2 {
		t.Fatalf("%.0fpx label did not wrap into rendered lines: size=%v", width, label.Size())
	}
	var rendered strings.Builder
	for _, text := range texts {
		position, ok := classFlagObjectPosition(label, text)
		if !ok {
			t.Fatal("rendered text is outside its label")
		}
		if position.X < 0 || position.Y < 0 ||
			position.X+text.Size().Width > label.Size().Width+1 ||
			position.Y+text.Size().Height > label.Size().Height+1 {
			t.Fatalf("%.0fpx rendered line overflows label: position=%v size=%v label=%v text=%q",
				width, position, text.Size(), label.Size(), text.Text)
		}
		rendered.WriteString(text.Text)
	}
	stripSpace := func(value string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, value)
	}
	if got, want := stripSpace(rendered.String()), stripSpace(completeText); got != want {
		t.Fatalf("rendered lines lost content: got %d non-space runes, want %d", len([]rune(got)), len([]rune(want)))
	}
}

func classFlagCanvasTexts(root fyne.CanvasObject) []*canvas.Text {
	if text, ok := root.(*canvas.Text); ok && text.Visible() && text.Text != "" {
		return []*canvas.Text{text}
	}
	var texts []*canvas.Text
	for _, child := range classFlagChildren(root) {
		texts = append(texts, classFlagCanvasTexts(child)...)
	}
	return texts
}

func classFlagObjectPosition(root, target fyne.CanvasObject) (fyne.Position, bool) {
	if root == target {
		return fyne.NewPos(0, 0), true
	}
	for _, child := range classFlagChildren(root) {
		position, ok := classFlagObjectPosition(child, target)
		if ok {
			return position.Add(child.Position()), true
		}
	}
	return fyne.Position{}, false
}

func classFlagChildren(root fyne.CanvasObject) []fyne.CanvasObject {
	switch object := root.(type) {
	case *fyne.Container:
		return object.Objects
	case fyne.Widget:
		return test.WidgetRenderer(object).Objects()
	default:
		return nil
	}
}
