package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
)

func writePortraitFixture(t *testing.T, root, path string, pixel color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.SetNRGBA(x, y, pixel)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func portraitBrowserFixture(t *testing.T, root string) *AssetBrowser {
	t.Helper()
	vfs := NewVirtualFileSystem("", root)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	return &AssetBrowser{vfs: vfs, shaderResolver: NewShaderResolver(vfs)}
}

func assertPortraitPixel(t *testing.T, res fyne.Resource, want color.NRGBA) {
	t.Helper()
	img := decodedImageFor(res)
	if img == nil {
		t.Fatal("portrait did not decode")
	}
	if got := color.NRGBAModel.Convert(img.At(0, 0)); got != want {
		t.Fatalf("portrait pixel = %v, want %v", got, want)
	}
}

func TestPortraitDistinctModelsAndSourceChange(t *testing.T) {
	root := t.TempDir()
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	first := "models/players/portrait_fixture_a/mb2_icon_default.png"
	second := "models/players/portrait_fixture_b/mb2_icon_default.png"
	writePortraitFixture(t, root, first, red)
	writePortraitFixture(t, root, second, blue)
	ab := portraitBrowserFixture(t, root)
	assertPortraitPixel(t, ab.LoadIconResource(first), red)
	assertPortraitPixel(t, ab.LoadIconResource(second), blue)
	// A second load goes through the cache and must retain the same identity.
	assertPortraitPixel(t, ab.LoadIconResource(second), blue)

	replacementRoot := t.TempDir()
	writePortraitFixture(t, replacementRoot, first, blue)
	replacement := portraitBrowserFixture(t, replacementRoot)
	assertPortraitPixel(t, replacement.LoadIconResource(first), blue)
	// A formerly cached image cannot make an absent asset appear to exist.
	missing := portraitBrowserFixture(t, t.TempDir())
	if missing.LoadIconResource(first) != nil {
		t.Fatal("missing source reused another installation's cached portrait")
	}
}

func TestPortraitCacheRefreshesReplacedFile(t *testing.T) {
	path := "models/players/portrait_source_fixture/mb2_icon_default.png"
	red := color.NRGBA{R: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	root := t.TempDir()
	writePortraitFixture(t, root, path, red)
	ab := portraitBrowserFixture(t, root)
	assertPortraitPixel(t, ab.LoadIconResource(path), red)
	writePortraitFixture(t, root, path, blue)
	modified := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(root, path), modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := ab.vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	assertPortraitPixel(t, ab.LoadIconResource(path), blue)
}

func TestPortraitProfileRendersAndClearsMissing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	root := t.TempDir()
	red := color.NRGBA{R: 255, A: 255}
	writePortraitFixture(t, root, "models/players/portrait_profile_fixture/mb2_icon_default.png", red)
	ab := portraitBrowserFixture(t, root)
	e := &MBCHEditor{
		assetBrowser: ab, iconResolver: NewIconResolver(ab.vfs),
		modelEntry:    NewValidatedEntry(func(string) error { return nil }),
		skinEntry:     NewValidatedEntry(func(string) error { return nil }),
		uiShaderEntry: NewValidatedEntry(func(string) error { return nil }),
		iconPreview:   canvas.NewImageFromImage(nil),
	}
	e.modelEntry.SetText("portrait_profile_fixture")
	e.skinEntry.SetText("default")
	window := test.NewWindow(e.iconPreview)
	defer window.Close()
	window.Resize(fyne.NewSize(64, 64))
	e.updateIconPreview()
	captured := window.Canvas().Capture()
	if got := color.NRGBAModel.Convert(captured.At(32, 32)); got != red {
		t.Fatalf("profile rendered %v, want portrait %v", got, red)
	}
	e.modelEntry.SetText("portrait_missing_fixture")
	e.updateIconPreview()
	captured = window.Canvas().Capture()
	if got := color.NRGBAModel.Convert(captured.At(32, 32)); got == red {
		t.Fatal("missing model retained the previous portrait")
	}
}

func TestPortraitEmbeddedMissDoesNotPoisonVFS(t *testing.T) {
	path := "models/players/portrait_lifecycle_fixture/mb2_icon_default"
	if _, ok := LoadGameIcon(nil, path); ok {
		t.Fatal("fixture unexpectedly embedded")
	}
	root := t.TempDir()
	vfs := NewVirtualFileSystem("", root)
	if _, ok := LoadGameIcon(vfs, path); ok {
		t.Fatal("unindexed VFS unexpectedly resolved portrait")
	}
	blue := color.NRGBA{B: 255, A: 255}
	writePortraitFixture(t, root, path+".png", blue)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	img, ok := LoadGameIcon(vfs, path)
	if !ok || color.NRGBAModel.Convert(img.At(0, 0)) != blue {
		t.Fatal("embedded/initial indexing miss poisoned the loaded portrait")
	}
	red := color.NRGBA{R: 255, A: 255}
	writePortraitFixture(t, root, path+".png", red)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	img, ok = LoadGameIcon(vfs, path)
	if !ok || color.NRGBAModel.Convert(img.At(0, 0)) != red {
		t.Fatal("refresh retained a decoded image from the old index")
	}
	if err := os.Remove(filepath.Join(root, path+".png")); err != nil {
		t.Fatal(err)
	}
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadGameIcon(vfs, path); ok {
		t.Fatal("refresh retained an image whose source was removed")
	}
}

func TestPortraitDefaultFallbackIsNotArbitraryTexture(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	root := t.TempDir()
	red := color.NRGBA{R: 255, A: 255}
	writePortraitFixture(t, root, "models/players/portrait_fallback_fixture/mb2_icon_default.png", red)
	ab := portraitBrowserFixture(t, root)
	box := container.NewStack()
	updateGalleryPreview(&ModelInfo{Name: "portrait_fallback_fixture"}, "no_portrait", box, ab)
	window := test.NewWindow(box)
	defer window.Close()
	window.Resize(fyne.NewSize(200, 200))
	hasRed := func() bool {
		img := window.Canvas().Capture()
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				if color.NRGBAModel.Convert(img.At(x, y)) == red {
					return true
				}
			}
		}
		return false
	}
	if !hasRed() {
		t.Fatal("documented default portrait fallback did not render")
	}
	updateGalleryPreview(&ModelInfo{Name: "missing_model", PortraitKey: "models/players/portrait_fallback_fixture/mb2_icon_default.png"}, "default", box, ab)
	if hasRed() {
		t.Fatal("missing model rendered an unrelated fallback")
	}
}
