package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func TestSourceAssetSuggestionsUseSyntheticVFSAndDeclaredShaders(t *testing.T) {
	usePublicCatalogs(t)
	application, browser := syntheticSourceAssetApp(t)

	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatMBCH, "model", "reb", 10), "rebel")
	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatMBCH, "skin", "def", 10), "default")
	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatMBCH, "soundOn", "sound/", 10), "sound/weapons/rebel/on.wav")
	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatMBCH, "soundset", "ky", 10), "kyle")
	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatMBCH, "useTeam", "reb", 10), "rebels.mbtc")
	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatVEH, "model", "models/vehicles/", 10), "models/vehicles/alpha.glm")
	assertSourceAssetSuggestion(t, application.sourceAssetSuggestions(SourceFormatVEH, "skin", "models/vehicles/", 10), "models/vehicles/alpha.skin")

	images := application.sourceAssetSuggestions(SourceFormatMBCH, "Icon", "gfx/hud/", 20)
	assertSourceAssetSuggestion(t, images, "gfx/hud/direct")
	assertSourceAssetSuggestion(t, images, "gfx/hud/declared")
	assertSourceAssetSuggestion(t, images, "gfx/hud/builtin")
	for _, suggestion := range images {
		if strings.HasSuffix(strings.ToLower(suggestion), ".shader") {
			t.Fatalf("shader source file leaked as a value suggestion: %q", suggestion)
		}
	}
	if browser.LoadIconResource("gfx/hud/declared") == nil {
		t.Fatal("declared shader did not resolve to a real synthetic image preview")
	}
	if preview := browser.shaderResolver.Resolve("gfx/hud/builtin"); preview != "" {
		t.Fatalf("built-in-only shader unexpectedly resolved a still preview: %q", preview)
	}
}

func TestSourceAssetReferenceConventionsAndSecurity(t *testing.T) {
	usePublicCatalogs(t)
	tests := []struct {
		name      string
		format    SourceFormat
		field     string
		candidate string
		shader    bool
		want      string
		valid     bool
	}{
		{name: "player model shorthand", format: SourceFormatMBCH, field: "model", candidate: "models/players/rebel/model.glm", want: "rebel", valid: true},
		{name: "player skin shorthand", format: SourceFormatMBCH, field: "skin", candidate: "models/players/rebel/model_blue.skin", want: "blue", valid: true},
		{name: "vehicle model full path", format: SourceFormatVEH, field: "model", candidate: "models/vehicles/alpha.glm", want: "models/vehicles/alpha.glm", valid: true},
		{name: "vehicle skin full path", format: SourceFormatVEH, field: "skin", candidate: "models/vehicles/alpha.skin", want: "models/vehicles/alpha.skin", valid: true},
		{name: "soundset folder identifier", format: SourceFormatMBCH, field: "soundset", candidate: "sound/chars/kyle/misc/hello.wav", want: "kyle", valid: true},
		{name: "soundset rejects unrelated sound", format: SourceFormatMBCH, field: "soundset", candidate: "sound/weapons/rebel/on.wav"},
		{name: "custom model path", format: SourceFormatMBCH, field: "NewWorldModel", candidate: "models/weapons2/blaster/model.glm", want: "models/weapons2/blaster/model.glm", valid: true},
		{name: "image extension omitted", format: SourceFormatMBCH, field: "Icon", candidate: `base\gfx\hud\direct.PNG`, want: "gfx/hud/direct", valid: true},
		{name: "declared shader", format: SourceFormatMBCH, field: "FriendlyShader", candidate: "gfx/hud/declared", shader: true, want: "gfx/hud/declared", valid: true},
		{name: "sound keeps extension", format: SourceFormatMBCH, field: "StartSound", candidate: "sound/weapons/rebel/on.wav", want: "sound/weapons/rebel/on.wav", valid: true},
		{name: "team uses portable basename", format: SourceFormatSiege, field: "useTeam", candidate: "ext_data/mb2/siegeteams/rebels.mbtc", want: "rebels.mbtc", valid: true},
		{name: "shader source is not an icon", format: SourceFormatMBCH, field: "Icon", candidate: "scripts/icons.shader"},
		{name: "wrong type", format: SourceFormatMBCH, field: "StartSound", candidate: "gfx/hud/direct.png"},
		{name: "absolute host path", format: SourceFormatMBCH, field: "Icon", candidate: "/private/assets/direct.png"},
		{name: "windows host path", format: SourceFormatMBCH, field: "Icon", candidate: `C:\private\direct.png`},
		{name: "traversal", format: SourceFormatMBCH, field: "Icon", candidate: "gfx/../private/direct.png"},
		{name: "quote cannot be represented", format: SourceFormatMBCH, field: "Icon", candidate: `gfx/hud/bad"name.png`},
		{name: "newline cannot be represented", format: SourceFormatMBCH, field: "Icon", candidate: "gfx/hud/bad\nname.png"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := sourceAssetReference(test.format, test.field, test.candidate, test.shader)
			if valid != test.valid || got != test.want {
				t.Fatalf("sourceAssetReference(%q, %q, %q, %v) = %q, %v; want %q, %v", test.format, test.field, test.candidate, test.shader, got, valid, test.want, test.valid)
			}
		})
	}
}

func TestSourceAssetInsertionRetainsRangeQuotesAndCancellation(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)

	unquoted := "WeaponInfo0 {\n Icon gfx/hud/old\n}\n"
	code := NewSourceCodeEditor()
	code.SetText(unquoted)
	caret := strings.Index(unquoted, "old") + 1
	code.SelectRange(caret, caret)
	target, ok := captureSourceAssetTarget(code, SourceFormatMBCH)
	if !ok {
		t.Fatal("failed to capture unquoted asset target")
	}
	code.SelectRange(0, 0) // Picker focus may move the live selection.
	if target.apply("gfx/hud/direct", false) || code.Text() != unquoted {
		t.Fatal("cancelled picker mutated the source")
	}
	if !target.apply("gfx/hud/direct", true) || code.Text() != "WeaponInfo0 {\n Icon \"gfx/hud/direct\"\n}\n" {
		t.Fatalf("unquoted insertion = %q", code.Text())
	}

	quoted := "WeaponInfo0 {\n Icon \"gfx/hud/old\"\n}\n"
	code.SetText(quoted)
	caret = strings.Index(quoted, "old") + 1
	code.SelectRange(caret, caret)
	target, ok = captureSourceAssetTarget(code, SourceFormatMBCH)
	if !ok || !target.apply(`gfx\hud\direct`, true) {
		t.Fatal("failed to replace quoted asset")
	}
	if code.Text() != "WeaponInfo0 {\n Icon \"gfx/hud/direct\"\n}\n" {
		t.Fatalf("quoted insertion escaped incorrectly: %q", code.Text())
	}

	for name, caretPrefix := range map[string]string{
		"inside empty quotes":  "Icon \"",
		"after empty quotes":   "Icon \"\"",
		"before opening quote": "Icon ",
	} {
		source := "WeaponInfo0 {\n Icon \"\"\n}\n"
		if name == "before opening quote" {
			source = quoted
		}
		code.SetText(source)
		caret := strings.Index(source, caretPrefix) + len(caretPrefix)
		code.SelectRange(caret, caret)
		target, ok := captureSourceAssetTarget(code, SourceFormatMBCH)
		if !ok || !target.apply("gfx/hud/direct", true) {
			t.Fatalf("%s: asset target rejected", name)
		}
		if got, want := code.Text(), "WeaponInfo0 {\n Icon \"gfx/hud/direct\"\n}\n"; got != want {
			t.Fatalf("%s: insertion = %q, want %q", name, got, want)
		}
	}

	code.SetText(quoted)
	code.SelectRange(caret, caret)
	target, ok = captureSourceAssetTarget(code, SourceFormatMBCH)
	if !ok {
		t.Fatal("failed to recapture asset target")
	}
	if target.apply(`gfx/hud/bad"name`, true) || code.Text() != quoted {
		t.Fatal("unrepresentable quote mutated the source")
	}

	code.SetText(quoted)
	code.SelectRange(caret, caret)
	target, ok = captureSourceAssetTarget(code, SourceFormatMBCH)
	if !ok {
		t.Fatal("failed to capture target before document deactivation")
	}
	code.SetReadOnly(true)
	if target.apply("gfx/hud/direct", true) || code.Text() != quoted {
		t.Fatal("deactivated document target mutated the source")
	}
	if _, ok := captureSourceAssetTarget(code, SourceFormatMBCH); ok {
		t.Fatal("read-only document accepted a new asset target")
	}
	code.SetReadOnly(false)

	code.SetText(unquoted)
	code.SelectRange(strings.Index(unquoted, "Icon")+1, strings.Index(unquoted, "Icon")+1)
	if _, ok := captureSourceAssetTarget(code, SourceFormatMBCH); ok {
		t.Fatal("key-position picker target was accepted")
	}
}

func TestVirtualFilePickerHasNoHostFilesystemSourcesAndCancelIsSilent(t *testing.T) {
	usePublicCatalogs(t)
	useEditorTestApp(t)
	application, _ := syntheticSourceAssetApp(t)
	browser := sourceAssetBrowserView(application.assetBrowser)
	if !browser.virtualOnly {
		t.Fatal("source asset browser was not isolated before UI construction")
	}
	if len(browser.sourceSelect.Options) != 1 || browser.sourceSelect.Options[0] != "All Game Data (Virtual)" {
		t.Fatalf("browser constructed host sources: %v", browser.sourceSelect.Options)
	}
	window := fyne.CurrentApp().NewWindow("virtual picker")
	t.Cleanup(func() {
		for _, open := range fyne.CurrentApp().Driver().AllWindows() {
			if open == window {
				window.Close()
				break
			}
		}
	})
	picker := NewVirtualFilePicker(window, browser)
	if len(picker.sources) != 1 || picker.sources[0] != "All Game Data (Virtual)" {
		t.Fatalf("virtual picker sources = %v", picker.sources)
	}
	picker.SetInitialPath("/private/host/path")
	selected := false
	picker.Show(func(*AssetEntry) { selected = true })
	if browser.currentPK3 != "VFS" {
		t.Fatalf("virtual picker backend = %q, want VFS", browser.currentPK3)
	}
	picker.cancelButton.Tapped(nil)
	if selected {
		t.Fatal("Cancel invoked the asset selection callback")
	}
}

func TestSourceAssetSuggestionLimitFollowsNormalizationAndDeduplication(t *testing.T) {
	usePublicCatalogs(t)
	root := t.TempDir()
	for i := 0; i < 425; i++ {
		writeSourceAssetFile(t, root, fmt.Sprintf("models/players/duplicate_%03d/model_default.skin", i), []byte("skin"))
	}
	writeSourceAssetFile(t, root, "models/players/zzzz/model_default_blue.skin", []byte("skin"))

	vfs := NewVirtualFileSystem("", root)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	application := &App{assetBrowser: &AssetBrowser{vfs: vfs}}
	got := application.sourceAssetSuggestions(SourceFormatMBCH, "skin", "def", 2)
	if len(got) != 2 || got[0] != "default" || got[1] != "default_blue" {
		t.Fatalf("normalized limited suggestions = %v, want [default default_blue]", got)
	}
}

func syntheticSourceAssetApp(t *testing.T) (*App, *AssetBrowser) {
	t.Helper()
	root := t.TempDir()
	writeSourceAssetFile(t, root, "models/players/rebel/model.glm", []byte("model"))
	writeSourceAssetFile(t, root, "models/players/rebel/model_default.skin", []byte("skin"))
	writeSourceAssetFile(t, root, "models/weapons2/blaster/model.glm", []byte("weapon model"))
	writeSourceAssetFile(t, root, "models/vehicles/alpha.glm", []byte("vehicle model"))
	writeSourceAssetFile(t, root, "models/vehicles/alpha.skin", []byte("vehicle skin"))
	writeSourceAssetFile(t, root, "sound/weapons/rebel/on.wav", []byte("wave"))
	writeSourceAssetFile(t, root, "sound/chars/kyle/misc/hello.wav", []byte("voice"))
	writeSourceAssetFile(t, root, "ext_data/mb2/siegeteams/rebels.mbtc", []byte("team"))

	imagePath := filepath.Join(root, "gfx", "hud", "direct.png")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imageData.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, imageData); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	writeSourceAssetFile(t, root, "scripts/source_assets.shader", []byte(`
gfx/hud/declared
{
    {
        map gfx/hud/direct.png
    }
}

gfx/hud/builtin
{
    {
        map $whiteimage
    }
}
`))

	vfs := NewVirtualFileSystem("", root)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	resolver := NewShaderResolver(vfs)
	resolver.Prebuild()
	browser := &AssetBrowser{
		assets:         make(map[string]*AssetEntry),
		rootEntries:    []*AssetEntry{},
		viewMode:       ViewModeGrid,
		sortMode:       SortNameAsc,
		iconSize:       100,
		textAssetsPath: root,
		vfs:            vfs,
		shaderResolver: resolver,
	}
	return &App{assetBrowser: browser}, browser
}

func writeSourceAssetFile(t *testing.T, root, relative string, data []byte) {
	t.Helper()
	filename := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSourceAssetSuggestion(t *testing.T, suggestions []string, want string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion == want {
			return
		}
	}
	t.Fatalf("suggestions %v do not include %q", suggestions, want)
}
