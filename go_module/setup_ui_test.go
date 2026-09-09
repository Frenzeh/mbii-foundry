package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestValidateSetupPathsIndependentOptionalFields(t *testing.T) {
	root := t.TempDir()
	gamedata := filepath.Join(root, "GameData")
	textAssets := filepath.Join(root, "TextAssets")
	invalid := filepath.Join(root, "invalid")
	for _, dir := range []string{
		filepath.Join(gamedata, "base"),
		filepath.Join(gamedata, "MBII"),
		filepath.Join(textAssets, "ext_data"),
		invalid,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name       string
		gamedata   string
		textAssets string
		wantCanUse bool
		wantGDErr  bool
		wantTAErr  bool
	}{
		{name: "empty uses skip", wantCanUse: false},
		{name: "GameData only", gamedata: gamedata, wantCanUse: true},
		{name: "TextAssets only", textAssets: textAssets, wantCanUse: true},
		{name: "both valid", gamedata: gamedata, textAssets: textAssets, wantCanUse: true},
		{name: "invalid GameData blocks valid TextAssets", gamedata: invalid, textAssets: textAssets, wantGDErr: true},
		{name: "valid GameData does not mask invalid TextAssets", gamedata: gamedata, textAssets: invalid, wantTAErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateSetupPaths(tt.gamedata, tt.textAssets)
			if got.CanUse != tt.wantCanUse {
				t.Fatalf("CanUse = %v, want %v", got.CanUse, tt.wantCanUse)
			}
			if (got.GameDataErr != nil) != tt.wantGDErr {
				t.Fatalf("GameDataErr = %v, want error %v", got.GameDataErr, tt.wantGDErr)
			}
			if (got.TextAssetsErr != nil) != tt.wantTAErr {
				t.Fatalf("TextAssetsErr = %v, want error %v", got.TextAssetsErr, tt.wantTAErr)
			}
		})
	}
}

func TestDetectSetupPathsFindsTextAssetsWithoutGameData(t *testing.T) {
	root := t.TempDir()
	textAssets := filepath.Join(root, "TextAssets")
	if err := os.MkdirAll(filepath.Join(textAssets, "models"), 0o755); err != nil {
		t.Fatal(err)
	}

	var effectiveGameData string
	detected := detectSetupPaths(
		"",
		"",
		func() string { return "" },
		func(gamedata string) string {
			effectiveGameData = gamedata
			return textAssets
		},
	)
	if effectiveGameData != "" {
		t.Fatalf("TextAssets detector received unexpected GameData %q", effectiveGameData)
	}
	if !detected.FoundAny || detected.GameDataPath != "" || detected.TextAssetsPath != textAssets {
		t.Fatalf("unexpected detection result: %+v", detected)
	}
	if state := validateSetupPaths(detected.GameDataPath, detected.TextAssetsPath); !state.CanUse {
		t.Fatalf("detected TextAssets-only result is not usable: %+v", state)
	}
}

func TestSetupWizardTextAssetsOnlyCanPersist(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	defer ui.Quit()

	root := t.TempDir()
	textAssets := filepath.Join(root, "TextAssets")
	if err := os.MkdirAll(filepath.Join(textAssets, "ext_data"), 0o755); err != nil {
		t.Fatal(err)
	}
	window := ui.NewWindow("setup")
	window.Resize(fyne.NewSize(1024, 720))
	window.Show()
	defer window.Close()

	configPath := filepath.Join(root, "config.json")
	app := &App{
		fyneApp:    ui,
		mainWindow: window,
		configPath: configPath,
		config: AppConfig{
			TextAssetsPath: textAssets,
		},
	}
	app.showSetupWizard()

	useButton := findButtonByText(window.Canvas().Overlays().Top(), "Use these folders")
	if useButton == nil {
		t.Fatal("Use these folders button not found")
	}
	if useButton.Disabled() {
		t.Fatal("TextAssets-only valid configuration left primary action disabled")
	}
	useButton.OnTapped()

	if !app.config.SetupWizardSeen || app.config.GamedataPath != "" || app.config.TextAssetsPath != textAssets {
		t.Fatalf("unexpected persisted app config: %+v", app.config)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	var persisted AppConfig
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode persisted config: %v", err)
	}
	if !persisted.SetupWizardSeen || persisted.GamedataPath != "" || persisted.TextAssetsPath != textAssets {
		t.Fatalf("unexpected config on disk: %+v", persisted)
	}
}

func TestSetupWizardHeadlessLayoutFitsWithoutScroll(t *testing.T) {
	for _, size := range []fyne.Size{
		fyne.NewSize(1400, 892),
		fyne.NewSize(1024, 720),
	} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size.Width, size.Height), func(t *testing.T) {
			ui := test.NewApp()
			ui.Settings().SetTheme(&FoundryTheme{})
			defer ui.Quit()
			window := ui.NewWindow("layout")
			window.Resize(size)
			window.Show()
			defer window.Close()

			app := &App{fyneApp: ui, mainWindow: window, configPath: filepath.Join(t.TempDir(), "config.json")}
			app.showSetupWizard()
			overlay := window.Canvas().Overlays().Top()
			modal, ok := overlay.(*widget.PopUp)
			if !ok {
				t.Fatalf("setup popup not shown, got %T", overlay)
			}
			if containsScroll(modal.Content) {
				t.Fatal("setup dialog contains a scroll container")
			}
			want := app.boundedDialogSize(fyne.NewSize(760, 520))
			min, got := modal.Content.MinSize(), modal.Content.Size()
			t.Logf("window=%v dialog=%v min=%v scroll=false", size, got, min)
			if min.Width > want.Width || min.Height > want.Height {
				t.Fatalf("setup min size %v exceeds bounded dialog %v", min, want)
			}
			if got.Width > want.Width || got.Height > want.Height {
				t.Fatalf("setup dialog %v exceeds bounded size %v", got, want)
			}
		})
	}
}

func TestSetupWizardLongValidationStatusStaysBounded(t *testing.T) {
	for _, size := range []fyne.Size{
		fyne.NewSize(760, 500),
		fyne.NewSize(1024, 720),
	} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size.Width, size.Height), func(t *testing.T) {
			ui := test.NewApp()
			ui.Settings().SetTheme(&FoundryTheme{})
			defer ui.Quit()

			window := ui.NewWindow("long validation")
			window.Resize(size)
			window.Show()
			defer window.Close()

			missing := filepath.Join(t.TempDir(), strings.Repeat("missing-segment-", 14))
			app := &App{
				fyneApp:    ui,
				mainWindow: window,
				configPath: filepath.Join(t.TempDir(), "config.json"),
				config:     AppConfig{TextAssetsPath: missing},
			}
			app.showSetupWizard()
			popup, ok := window.Canvas().Overlays().Top().(*widget.PopUp)
			if !ok {
				t.Fatalf("setup popup not shown, got %T", window.Canvas().Overlays().Top())
			}
			window.Canvas().Capture()

			status := findLabelByPrefix(popup.Content, "TextAssets:")
			if status == nil {
				t.Fatal("long validation status not found")
			}
			if status.Size().Width < 240 {
				t.Fatalf("validation status collapsed to %.1fpx wide", status.Size().Width)
			}
			if status.Size().Height > 80 {
				t.Fatalf("validation status grew to %.1fpx high", status.Size().Height)
			}
			position, ok := objectPositionWithin(popup.Content, status)
			if !ok {
				t.Fatal("validation status is outside popup content tree")
			}
			if position.X < 0 || position.Y < 0 ||
				position.X+status.Size().Width > popup.Content.Size().Width+1 ||
				position.Y+status.Size().Height > popup.Content.Size().Height+1 {
				t.Fatalf("validation status overflows popup: pos=%v size=%v popup=%v",
					position, status.Size(), popup.Content.Size())
			}
			useButton := findButtonByText(popup.Content, "Use these folders")
			if useButton == nil {
				t.Fatal("setup footer action not found")
			}
			buttonPosition, ok := objectPositionWithin(popup.Content, useButton)
			if !ok {
				t.Fatal("setup footer action is outside popup content tree")
			}
			if position.Y+status.Size().Height > buttonPosition.Y {
				t.Fatalf("validation status overlaps footer: status=%v/%v buttonY=%.1f",
					position, status.Size(), buttonPosition.Y)
			}
			if containsScroll(popup.Content) {
				t.Fatal("long validation introduced a scroll container")
			}
		})
	}
}

func TestCharacterSummaryRemainsInEditorComposition(t *testing.T) {
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	defer ui.Quit()

	editor := NewMBCHEditor(nil)
	if editor.summary == nil {
		t.Fatal("character summary was not constructed")
	}
	if !containsObject(editor.GetContent(), editor.summary) {
		t.Fatal("character summary is no longer part of the editor composition")
	}
	summaryMin := editor.summary.MinSize()
	t.Logf("summary min=%v composed=true", summaryMin)
	if summaryMin.Height > 120 {
		t.Fatalf("character summary is not compact: min size %v", summaryMin)
	}

	window := ui.NewWindow("editor layout")
	window.SetContent(editor.GetContent())
	window.Show()
	defer window.Close()
	for _, size := range []fyne.Size{fyne.NewSize(1400, 892), fyne.NewSize(1024, 720)} {
		window.Resize(size)
		t.Logf("window=%v summary=%v visible=%v", size, editor.summary.Size(), editor.summary.Visible())
		if !editor.summary.Visible() || editor.summary.Size().Height <= 0 {
			t.Fatalf("summary not laid out at window size %v: visible=%v size=%v", size, editor.summary.Visible(), editor.summary.Size())
		}
	}
}

func findButtonByText(root fyne.CanvasObject, text string) *widget.Button {
	if root == nil {
		return nil
	}
	if button, ok := root.(*widget.Button); ok && button.Text == text {
		return button
	}
	for _, child := range childObjects(root) {
		if button := findButtonByText(child, text); button != nil {
			return button
		}
	}
	return nil
}

func containsScroll(root fyne.CanvasObject) bool {
	if root == nil {
		return false
	}
	if fmt.Sprintf("%T", root) == "*widget.Scroll" {
		return true
	}
	for _, child := range childObjects(root) {
		if containsScroll(child) {
			return true
		}
	}
	return false
}

func containsObject(root, target fyne.CanvasObject) bool {
	if root == target {
		return true
	}
	for _, child := range childObjects(root) {
		if containsObject(child, target) {
			return true
		}
	}
	return false
}

func findLabelByPrefix(root fyne.CanvasObject, prefix string) *widget.Label {
	if root == nil {
		return nil
	}
	if label, ok := root.(*widget.Label); ok && strings.HasPrefix(label.Text, prefix) {
		return label
	}
	for _, child := range childObjects(root) {
		if label := findLabelByPrefix(child, prefix); label != nil {
			return label
		}
	}
	return nil
}

func objectPositionWithin(root, target fyne.CanvasObject) (fyne.Position, bool) {
	if root == target {
		return fyne.NewPos(0, 0), true
	}
	for _, child := range childObjects(root) {
		position, ok := objectPositionWithin(child, target)
		if !ok {
			continue
		}
		childPosition := child.Position()
		return fyne.NewPos(position.X+childPosition.X, position.Y+childPosition.Y), true
	}
	return fyne.Position{}, false
}

func childObjects(root fyne.CanvasObject) []fyne.CanvasObject {
	switch object := root.(type) {
	case *fyne.Container:
		return object.Objects
	case *widget.PopUp:
		return []fyne.CanvasObject{object.Content}
	default:
		return nil
	}
}
