package main

import (
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fyneTest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func pickerContainmentObject(root fyne.CanvasObject, matches func(fyne.CanvasObject) bool) fyne.CanvasObject {
	if root == nil {
		return nil
	}
	if matches(root) {
		return root
	}
	for _, child := range childObjects(root) {
		if found := pickerContainmentObject(child, matches); found != nil {
			return found
		}
	}
	return nil
}

func pickerContainmentAncestor(root, first, second fyne.CanvasObject) fyne.CanvasObject {
	if root == nil || !containsObject(root, first) || !containsObject(root, second) {
		return nil
	}
	for _, child := range childObjects(root) {
		if found := pickerContainmentAncestor(child, first, second); found != nil {
			return found
		}
	}
	return root
}

func pickerContainmentAssertInside(t *testing.T, root, object fyne.CanvasObject) {
	t.Helper()
	position, found := objectPositionWithin(root, object)
	if !found || !object.Visible() || object.Size().Width <= 0 || object.Size().Height <= 0 ||
		position.X < -1 || position.Y < -1 ||
		position.X+object.Size().Width > root.Size().Width+1 ||
		position.Y+object.Size().Height > root.Size().Height+1 {
		t.Fatalf("%T escapes picker header: found=%v position=%v size=%v bounds=%v", object, found, position, object.Size(), root.Size())
	}
}

func pickerContainmentAssertNoOverlap(t *testing.T, root fyne.CanvasObject, objects []fyne.CanvasObject) {
	t.Helper()
	for index, object := range objects {
		position, _ := objectPositionWithin(root, object)
		for _, other := range objects[index+1:] {
			otherPosition, _ := objectPositionWithin(root, other)
			if position.X < otherPosition.X+other.Size().Width-1 &&
				otherPosition.X < position.X+object.Size().Width-1 &&
				position.Y < otherPosition.Y+other.Size().Height-1 &&
				otherPosition.Y < position.Y+object.Size().Height-1 {
				t.Fatalf("picker controls overlap: %T at %v/%v and %T at %v/%v", object, position, object.Size(), other, otherPosition, other.Size())
			}
		}
	}
}

func pickerContainmentCurrentCrumb(t *testing.T, picker *CustomFilePicker) *widget.Button {
	t.Helper()
	for index := len(picker.pathBar.Objects) - 1; index >= 0; index-- {
		if button, ok := picker.pathBar.Objects[index].(*widget.Button); ok {
			return button
		}
	}
	t.Fatal("picker has no current breadcrumb")
	return nil
}

func pickerContainmentAssertCurrentVisible(t *testing.T, picker *CustomFilePicker) {
	t.Helper()
	current := pickerContainmentCurrentCrumb(t, picker)
	position, found := objectPositionWithin(picker.pathBar, current)
	left := position.X - picker.pathScroll.Offset.X
	if !found || left < -1 || left+current.Size().Width > picker.pathScroll.Size().Width+1 {
		t.Fatalf("current breadcrumb is outside its viewport: found=%v left=%v size=%v viewport=%v offset=%v", found, left, current.Size(), picker.pathScroll.Size(), picker.pathScroll.Offset)
	}
}

func TestPickerHeaderContainsActionsAndKeepsCurrentPathReachable(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	deep := filepath.Join(root, "alpha-path", "beta-path", "gamma-path", "current-location")
	writeAuditAsset(t, deep, "selection.txt", "selected")

	browser := auditAssetBrowser(t, "", "")
	window := fyne.CurrentApp().NewWindow("picker containment")
	defer window.Close()
	picker := NewCustomFilePicker(window, browser)
	browser.loadFS(deep)
	picker.Show(nil)

	up := pickerContainmentObject(window.Content(), func(object fyne.CanvasObject) bool {
		button, ok := object.(*widget.Button)
		return ok && button.Text == "Up"
	})
	copyLocation := pickerContainmentObject(window.Content(), func(object fyne.CanvasObject) bool {
		button, ok := object.(*TooltipButton)
		return ok && button.Icon != nil && button.Icon.Name() == theme.ContentCopyIcon().Name() &&
			button.tooltipText == "Copy full picker location"
	})
	if up == nil || copyLocation == nil {
		t.Fatal("picker navigation or full-location copy action is unavailable")
	}
	header := pickerContainmentAncestor(window.Content(), up, copyLocation)
	if header == nil || !containsObject(header, picker.pathScroll) {
		t.Fatal("picker header does not contain the path viewport")
	}

	action := func(matches func(fyne.CanvasObject) bool) fyne.CanvasObject {
		return pickerContainmentObject(header, matches)
	}
	controls := []fyne.CanvasObject{
		up,
		action(func(object fyne.CanvasObject) bool {
			button, ok := object.(*widget.Button)
			return ok && button.Text == "Home"
		}),
		action(func(object fyne.CanvasObject) bool {
			button, ok := object.(*widget.Button)
			return ok && button.Icon != nil && button.Icon.Name() == theme.ViewRefreshIcon().Name()
		}),
		action(func(object fyne.CanvasObject) bool {
			_, ok := object.(*widget.Select)
			return ok
		}),
		action(func(object fyne.CanvasObject) bool {
			button, ok := object.(*widget.Button)
			return ok && (button.Text == "Favorite" || button.Text == "Unfavorite")
		}),
		action(func(object fyne.CanvasObject) bool {
			button, ok := object.(*widget.Button)
			return ok && (button.Text == "List" || button.Text == "Grid")
		}),
		copyLocation,
	}
	for _, control := range controls {
		if control == nil {
			t.Fatal("picker header lost an action")
		}
	}

	splitObject := pickerContainmentObject(window.Content(), func(object fyne.CanvasObject) bool {
		split, ok := object.(*container.Split)
		return ok && split.Horizontal && containsObject(split.Trailing, picker.pathScroll)
	})
	if splitObject == nil {
		t.Fatal("picker split allocation is unavailable")
	}
	split := splitObject.(*container.Split)
	for _, width := range []float32{900, 1200} {
		for _, offset := range []float64{0.25, 0.65} {
			window.Resize(fyne.NewSize(width, 812))
			split.SetOffset(offset)
			browser.loadFS(deep)
			window.Canvas().Capture()

			if header.Size().Width > split.Trailing.Size().Width+1 || header.MinSize().Width > split.Trailing.Size().Width+1 {
				t.Fatalf("picker header pins %.0fpx window at offset %.2f: header=%v/%v trailing=%v", width, offset, header.Size(), header.MinSize(), split.Trailing.Size())
			}
			for _, control := range controls {
				pickerContainmentAssertInside(t, header, control)
			}
			pickerContainmentAssertNoOverlap(t, header, controls)
			pickerContainmentAssertCurrentVisible(t, picker)

			fyne.CurrentApp().Clipboard().SetContent("")
			fyneTest.Tap(copyLocation.(*TooltipButton))
			want := "Files / " + deep
			if copied := fyne.CurrentApp().Clipboard().Content(); copied != want || browser.breadcrumbLabel.Text != want {
				t.Fatalf("full current path was not copied: copied=%q live=%q want=%q", copied, browser.breadcrumbLabel.Text, want)
			}
		}
	}

	auditPickerBreadcrumb(t, picker, "beta-path")
	wantAncestor := filepath.Join(root, "alpha-path", "beta-path")
	if browser.currentDir == nil || browser.currentDir.Path != wantAncestor {
		t.Fatalf("ancestor breadcrumb navigation = %#v, want %q", browser.currentDir, wantAncestor)
	}
	window.Canvas().Capture()
	pickerContainmentAssertCurrentVisible(t, picker)
	fyne.CurrentApp().Clipboard().SetContent("")
	fyneTest.Tap(copyLocation.(*TooltipButton))
	if copied := fyne.CurrentApp().Clipboard().Content(); copied != "Files / "+wantAncestor {
		t.Fatalf("ancestor location copy = %q, want %q", copied, "Files / "+wantAncestor)
	}
}

func TestGridCaptionPaintsEllipsisWithoutChangingSelectionValue(t *testing.T) {
	useEditorTestApp(t)
	root := t.TempDir()
	names := []string{
		"responsive-saber.sab",
		"responsive-typography-long-character-filename.mbch",
	}
	paths := make(map[string]string, len(names))
	for _, name := range names {
		paths[name] = writeAuditAsset(t, root, name, "ClassInfo\n{\n}\n")
	}
	browser := auditAssetBrowser(t, "", "")
	browser.loadFS(root)
	items := make([]*GridItem, 0, len(names))
	objects := make([]fyne.CanvasObject, 0, len(names))
	for _, name := range names {
		item := auditAssetItem(t, browser, paths[name])
		items = append(items, item)
		objects = append(objects, item)
	}

	window := fyne.CurrentApp().NewWindow("grid caption containment")
	window.SetPadded(false)
	window.SetContent(container.NewGridWrap(fyne.NewSize(100, 130), objects...))
	window.Resize(fyne.NewSize(204, 130))
	window.Show()
	defer window.Close()
	window.Canvas().Capture()

	var selected *AssetEntry
	browser.SetOnAssetSelected(func(entry *AssetEntry) { selected = entry })
	for index, item := range items {
		name := names[index]
		painted := pickerContainmentObject(item, func(object fyne.CanvasObject) bool {
			text, ok := object.(*canvas.Text)
			return ok && strings.HasSuffix(text.Text, "…") &&
				strings.HasPrefix(name, strings.TrimSuffix(text.Text, "…"))
		})
		if painted == nil {
			t.Fatalf("long grid filename %q did not paint a deliberate ellipsis", name)
		}
		pickerContainmentAssertInside(t, item, painted)
		if painted.MinSize().Width > painted.Size().Width+1 {
			t.Fatalf("ellipsized caption glyphs exceed their allocation: min=%v size=%v", painted.MinSize(), painted.Size())
		}

		fyneTest.Tap(item)
		if selected != item.Entry || browser.GetSelectedAsset() != item.Entry {
			t.Fatalf("caption tap did not retain the selected asset entry for %q", name)
		}
		if item.Text != name || item.Entry.Name != name || item.Entry.Path != paths[name] {
			t.Fatalf("caption truncation changed selection values: text=%q entry=%#v", item.Text, item.Entry)
		}
	}
}
