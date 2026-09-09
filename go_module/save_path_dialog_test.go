package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/safeio"
)

func TestSavePathDialogRejectedSavePreservesDestination(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(theme.DefaultTheme())
	window := app.NewWindow("Save destination")
	window.Resize(fyne.NewSize(900, 700))
	window.Show()
	path := filepath.Join(t.TempDir(), "existing.mbch")
	original := []byte("existing document must survive rejection")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	replacement := []byte("corrected document")
	accepted := false
	ShowSavePathDialog(window, "Save As", path, ".mbch", func(destination string) error {
		called = true
		if destination != path {
			t.Fatalf("unexpected destination: %s", destination)
		}
		if !accepted {
			return errors.New("generated document rejected")
		}
		return safeio.WriteFile(destination, replacement, 0600)
	})
	test.Tap(saveDialogButton(t, window.Canvas().Overlays().Top(), "Save"))
	if called {
		t.Fatal("existing destination was submitted before overwrite confirmation")
	}
	test.Tap(saveDialogButton(t, window.Canvas().Overlays().Top(), "Yes"))
	if !called {
		t.Fatal("confirmed save did not reach the editor")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("destination was changed before the editor accepted new contents")
	}
	if window.Canvas().Overlays().Top() == nil {
		t.Fatal("failed save discarded the destination dialog instead of allowing correction")
	}
	accepted = true
	test.Tap(saveDialogButton(t, window.Canvas().Overlays().Top(), "Save"))
	test.Tap(saveDialogButton(t, window.Canvas().Overlays().Top(), "Yes"))
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(replacement) {
		t.Fatal("retry did not replace the existing file with accepted contents")
	}
	if window.Canvas().Overlays().Top() != nil {
		t.Fatal("successful retry left the destination chooser open")
	}
}

func TestSavePathDialogCancelPreservesDestination(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("Cancel destination")
	window.Resize(fyne.NewSize(900, 700))
	window.Show()
	path := filepath.Join(t.TempDir(), "existing.mbch")
	original := []byte("unchanged after cancel")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	ShowSavePathDialog(window, "Save As", path, ".mbch", func(string) error {
		t.Fatal("cancel submitted a save")
		return nil
	})
	test.Tap(saveDialogButton(t, window.Canvas().Overlays().Top(), "Cancel"))
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("cancel changed the existing destination")
	}
	if window.Canvas().Overlays().Top() != nil {
		t.Fatal("cancel left the destination chooser open")
	}
}

func saveDialogButton(t *testing.T, root fyne.CanvasObject, label string) *widget.Button {
	t.Helper()
	seen := make(map[fyne.CanvasObject]bool)
	var visit func(fyne.CanvasObject) *widget.Button
	visit = func(object fyne.CanvasObject) *widget.Button {
		if object == nil || seen[object] {
			return nil
		}
		seen[object] = true
		if button, ok := object.(*widget.Button); ok && button.Text == label {
			return button
		}
		var children []fyne.CanvasObject
		switch value := object.(type) {
		case *fyne.Container:
			children = value.Objects
		case fyne.Widget:
			children = test.WidgetRenderer(value).Objects()
		}
		for _, child := range children {
			if found := visit(child); found != nil {
				return found
			}
		}
		return nil
	}
	button := visit(root)
	if button == nil {
		t.Fatalf("dialog action %q not found", label)
	}
	return button
}
