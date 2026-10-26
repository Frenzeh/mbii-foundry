package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func TestAuditFreshEditorsProjectInitializedDefaults(t *testing.T) {
	useEditorTestApp(t)

	t.Run("SAB", func(t *testing.T) {
		editor := NewSABEditor(&App{})
		got, err := parsers.ParseSAB(editor.CurrentSource())
		if err != nil {
			t.Fatal(err)
		}
		want := parsers.NewSaberData()
		if got.Name != "" || got.SaberType != want.SaberType || got.NumBlades != want.NumBlades || len(got.Blades) != len(want.Blades) {
			t.Fatalf("fresh Source changed saber identity/blades: %+v", got)
		}
		if got.Blades[0] != want.Blades[0] || got.MoveSpeedScale != want.MoveSpeedScale || got.AnimSpeedScale != want.AnimSpeedScale || got.DamageScale != want.DamageScale || got.KnockbackScale != want.KnockbackScale {
			t.Fatalf("fresh Source changed blade proportions or scales: %+v", got)
		}
		if editor.IsDirty() {
			t.Fatal("projecting a fresh saber must not dirty it")
		}
	})

	t.Run("VEH", func(t *testing.T) {
		editor := NewVEHEditor(&App{})
		got, err := parsers.ParseVEH(editor.CurrentSource())
		if err != nil {
			t.Fatal(err)
		}
		want := parsers.NewVehicleData()
		if got.Name != "" || got.Type != want.Type {
			t.Fatalf("fresh Source invented a name or changed vehicle type: %+v", got)
		}
		if editor.IsDirty() {
			t.Fatal("projecting a fresh vehicle must not dirty it")
		}
	})

	t.Run("Siege", func(t *testing.T) {
		editor := NewSiegeEditor(&App{})
		got, err := parsers.ParseSiege(editor.CurrentSource())
		if err != nil {
			t.Fatal(err)
		}
		if got.MissionName != "" || got.Team1 != nil || got.Team2 != nil || editor.IsDirty() {
			t.Fatalf("fresh Source changed the unnamed siege working state: %+v", got)
		}
	})
}

func TestAuditFreshEditorSingleEditUndoRedo(t *testing.T) {
	useEditorTestApp(t)
	for _, item := range []struct {
		name string
		make func() (SessionEditor, *widget.Entry)
	}{
		{name: "SAB", make: func() (SessionEditor, *widget.Entry) {
			editor := NewSABEditor(&App{})
			return editor, editor.fullNameEntry
		}},
		{name: "VEH", make: func() (SessionEditor, *widget.Entry) {
			editor := NewVEHEditor(&App{})
			return editor, editor.modelEntry
		}},
		{name: "Siege", make: func() (SessionEditor, *widget.Entry) {
			editor := NewSiegeEditor(&App{})
			return editor, editor.mapGraphicEntry
		}},
	} {
		t.Run(item.name, func(t *testing.T) {
			editor, entry := item.make()
			t.Cleanup(editor.Session().flushPending)
			// Do not observe source or session state before this first edit:
			// the constructor, not a later read, must establish the anchor.
			entry.SetText("working_value")
			if !editor.IsDirty() || !editor.CanUndo() {
				t.Fatal("the first form edit must immediately be dirty and undoable")
			}
			edited := editor.CurrentSource()
			if !editor.Undo() {
				t.Fatal("single edit could not restore the fresh unnamed working state")
			}
			if entry.Text != "" || editor.IsDirty() {
				t.Fatalf("undo did not restore the clean form: value=%q dirty=%v", entry.Text, editor.IsDirty())
			}
			if !editor.Redo() || entry.Text != "working_value" || !editor.IsDirty() || editor.CurrentSource() != edited {
				t.Fatal("redo lost the unnamed working form or its source")
			}
		})
	}
}

func TestAuditSABNewSaberIsGuardedAndUndoable(t *testing.T) {
	useEditorTestApp(t)
	editor := NewSABEditor(&App{})
	t.Cleanup(editor.Session().flushPending)
	if err := editor.ApplySourceText(multiSabFixture); err != nil {
		t.Fatal(err)
	}
	before := editor.CurrentSource()
	editor.NewSaber()
	fresh := editor.CurrentSource()
	got, err := parsers.ParseSAB(fresh)
	if err != nil {
		t.Fatal(err)
	}
	want := parsers.NewSaberData()
	if got.Name != "" || got.NumBlades != want.NumBlades || len(got.Blades) != len(want.Blades) || got.Blades[0] != want.Blades[0] {
		t.Fatalf("NewSaber lost fresh defaults: %+v", got)
	}
	if !editor.Undo() || editor.CurrentSource() != before {
		t.Fatal("NewSaber must restore the previous document on undo")
	}
	if !editor.Redo() || editor.CurrentSource() != fresh || editor.nameEntry.Text != "" {
		t.Fatal("redo must restore the fresh document without inventing a name")
	}
}

func TestAuditUnnamedSourceApplyRemainsRejected(t *testing.T) {
	useEditorTestApp(t)
	for _, editor := range []SessionEditor{NewSABEditor(&App{}), NewVEHEditor(&App{})} {
		fresh := editor.CurrentSource()
		if err := editor.ApplySourceText(fresh); err == nil {
			t.Fatalf("%T accepted an unnamed user Source Apply", editor)
		}
		if editor.CurrentSource() != fresh || editor.IsDirty() {
			t.Fatalf("%T changed after rejecting unnamed Source Apply", editor)
		}
	}
}

func TestAuditSABFractionalLoadSourceSave(t *testing.T) {
	useEditorTestApp(t)
	const source = `fractional_saber
{
	saberLength 32.1234567890123
	saberRadius 3.98765432109876
	moveSpeedScale 1.1234567890123457
	animSpeedScale 0.9876543210987654
	damageScale 2.3456789012345678
	knockbackScale 0.12345678901234566
}
`
	want, err := parsers.ParseSAB(source)
	if err != nil {
		t.Fatal(err)
	}
	editor := NewSABEditor(&App{})
	if err := editor.LoadFile(writeSab(t, source)); err != nil {
		t.Fatal(err)
	}
	projected := editor.CurrentSource()
	path := filepath.Join(t.TempDir(), "fractional.sab")
	if err := editor.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for stage, text := range map[string]string{"Source": projected, "saved": string(saved)} {
		got, err := parsers.ParseSAB(text)
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		if len(got.Blades) != len(want.Blades) {
			t.Fatalf("%s changed blade count", stage)
		}
		for field, pair := range map[string][2]float64{
			"length":    {got.Blades[0].Length, want.Blades[0].Length},
			"radius":    {got.Blades[0].Radius, want.Blades[0].Radius},
			"move":      {got.MoveSpeedScale, want.MoveSpeedScale},
			"animation": {got.AnimSpeedScale, want.AnimSpeedScale},
			"damage":    {got.DamageScale, want.DamageScale},
			"knockback": {got.KnockbackScale, want.KnockbackScale},
		} {
			if pair[0] != pair[1] {
				t.Errorf("%s rounded %s: got %.17g want %.17g", stage, field, pair[0], pair[1])
			}
		}
	}
}

func TestAuditVEHFractionalLoadSourceSave(t *testing.T) {
	useEditorTestApp(t)
	const source = `fractional_vehicle
{
	name fractional_vehicle
	type VH_SPEEDER
	speedMax 450.1234567890123
	turboSpeed 950.9876543210987
	acceleration 20.123456789012345
	decelIdle 10.987654321098765
	strafePerc 0.12345678901234566
	braking 10.234567890123456
}
`
	want, err := parsers.ParseVEH(source)
	if err != nil {
		t.Fatal(err)
	}
	editor := NewVEHEditor(&App{})
	if err := editor.LoadFile(writeReadOnlyVeh(t, source)); err != nil {
		t.Fatal(err)
	}
	projected := editor.CurrentSource()
	path := filepath.Join(t.TempDir(), "fractional.veh")
	if err := editor.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for stage, text := range map[string]string{"Source": projected, "saved": string(saved)} {
		got, err := parsers.ParseVEH(text)
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		for field, pair := range map[string][2]float64{
			"speed":        {got.SpeedMax, want.SpeedMax},
			"turbo":        {got.TurboSpeed, want.TurboSpeed},
			"acceleration": {got.Accel, want.Accel},
			"deceleration": {got.Decel, want.Decel},
			"strafe":       {got.StrafePerc, want.StrafePerc},
			"braking":      {got.Braking, want.Braking},
		} {
			if pair[0] != pair[1] {
				t.Errorf("%s rounded %s: got %.17g want %.17g", stage, field, pair[0], pair[1])
			}
		}
	}
}

func TestAuditSABRapidDefinitionSwitchRetainsPendingFormEdits(t *testing.T) {
	useEditorTestApp(t)
	editor := NewSABEditor(&App{})
	t.Cleanup(editor.Session().flushPending)
	if err := editor.LoadFile(writeSab(t, multiSabFixture)); err != nil {
		t.Fatal(err)
	}
	editor.fullNameEntry.SetText("Alpha working display")
	editor.blade1LengthEntry.SetText("43.123456789012345")
	if err := editor.SelectDefinition(1); err != nil {
		t.Fatal(err)
	}
	editor.fullNameEntry.SetText("Beta working display")
	editor.blade1RadiusEntry.SetText("4.234567890123456")
	if err := editor.SelectDefinition(0); err != nil {
		t.Fatal(err)
	}
	source := editor.CurrentSource()
	a, err := parsers.ParseSABDefinition(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parsers.ParseSABDefinition(source, 1)
	if err != nil {
		t.Fatal(err)
	}
	if a.FullName != "Alpha working display" || a.Blades[0].Length != 43.123456789012345 || b.FullName != "Beta working display" || b.Blades[0].Radius != 4.234567890123456 {
		t.Fatalf("rapid switches lost pending edits:\n%s", source)
	}
	if !editor.Undo() || editor.SelectedDefinition() != 1 || editor.fullNameEntry.Text != "Beta working display" {
		t.Fatal("undo must restore the previous selection and its edited form")
	}
}

func TestAuditVEHRapidDefinitionSwitchRetainsPendingFormEdits(t *testing.T) {
	useEditorTestApp(t)
	editor := NewVEHEditor(&App{})
	t.Cleanup(editor.Session().flushPending)
	if err := editor.LoadFile(writeReadOnlyVeh(t, realVEHHandlingFixture)); err != nil {
		t.Fatal(err)
	}
	editor.modelEntry.SetText("models/vehicles/alpha.glm")
	editor.speedEntry.SetText("450.1234567890123")
	if err := editor.SelectDefinition(1); err != nil {
		t.Fatal(err)
	}
	editor.modelEntry.SetText("models/vehicles/beta.glm")
	editor.accelEntry.SetText("15.9876543210123")
	if err := editor.SelectDefinition(0); err != nil {
		t.Fatal(err)
	}
	source := editor.CurrentSource()
	a, err := parsers.ParseVEHDefinition(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := parsers.ParseVEHDefinition(source, 1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Model != "models/vehicles/alpha.glm" || a.SpeedMax != 450.1234567890123 || b.Model != "models/vehicles/beta.glm" || b.Accel != 15.9876543210123 {
		t.Fatalf("rapid switches lost pending edits:\n%s", source)
	}
	if !editor.Undo() || editor.SelectedDefinition() != 1 || editor.modelEntry.Text != "models/vehicles/beta.glm" {
		t.Fatal("undo must restore the previous selection and its edited form")
	}
}

func auditWalkEditorObjects(root fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	seen := make(map[fyne.CanvasObject]bool)
	var walk func(fyne.CanvasObject)
	walk = func(object fyne.CanvasObject) {
		if object == nil || seen[object] {
			return
		}
		seen[object] = true
		visit(object)
		var children []fyne.CanvasObject
		switch value := object.(type) {
		case *fyne.Container:
			children = value.Objects
		case fyne.Widget:
			children = test.WidgetRenderer(value).Objects()
		}
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
}

func auditEditorTab(t *testing.T, editor Editor, name string) (*container.AppTabs, *container.TabItem) {
	t.Helper()
	var tabs *container.AppTabs
	var tab *container.TabItem
	auditWalkEditorObjects(editor.GetContent(), func(object fyne.CanvasObject) {
		if value, ok := object.(*container.AppTabs); ok {
			for _, item := range value.Items {
				if item.Text == name {
					tabs, tab = value, item
				}
			}
		}
	})
	if tab == nil {
		t.Fatalf("%T has no %q tab", editor, name)
	}
	return tabs, tab
}

func TestAuditLocalSourceTabsAreReadOnlyProjections(t *testing.T) {
	useEditorTestApp(t)
	for _, item := range []struct {
		name   string
		editor SessionEditor
	}{
		{name: "SAB", editor: NewSABEditor(&App{})},
		{name: "VEH", editor: NewVEHEditor(&App{})},
		{name: "Siege", editor: NewSiegeEditor(&App{})},
	} {
		t.Run(item.name, func(t *testing.T) {
			editor := item.editor
			tabs, tab := auditEditorTab(t, editor, "Source")
			tabs.Select(tab)
			var view *widget.RichText
			auditWalkEditorObjects(tab.Content, func(object fyne.CanvasObject) {
				switch value := object.(type) {
				case *widget.Entry:
					t.Errorf("%T Source tab still exposes untracked editable text", editor)
				case *widget.RichText:
					view = value
				}
			})
			if view == nil {
				t.Fatal("Source tab has no read-only text projection")
			}
			var displayed strings.Builder
			for _, segment := range view.Segments {
				if text, ok := segment.(*widget.TextSegment); ok {
					displayed.WriteString(text.Text)
				}
			}
			if displayed.String() != editor.CurrentSource() {
				t.Fatal("local Source projection differs from the working document")
			}
		})
	}
}
