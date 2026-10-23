package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func writeWeaponIconFixture(t *testing.T, root, path string, pixel color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.SetNRGBA(x, y, pixel)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, encoded.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

func weaponIconBrowserFixture(t *testing.T, root string) *AssetBrowser {
	t.Helper()
	vfs := NewVirtualFileSystem("", root)
	if err := vfs.Refresh(); err != nil {
		t.Fatal(err)
	}
	return &AssetBrowser{vfs: vfs, shaderResolver: NewShaderResolver(vfs)}
}

func resourcePixel(t *testing.T, resource fyne.Resource) color.NRGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(resource.Content()))
	if err != nil {
		t.Fatalf("decode resolved icon: %v", err)
	}
	return color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
}

func countRenderedColor(img image.Image, want color.NRGBA) int {
	count := 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if color.NRGBAModel.Convert(img.At(x, y)) == want {
				count++
			}
		}
	}
	return count
}

func renderedRegionHasContrast(img image.Image, position fyne.Position, size fyne.Size) bool {
	colors := make(map[color.NRGBA]struct{})
	for y := int(position.Y); y < int(position.Y+size.Height); y++ {
		for x := int(position.X); x < int(position.X+size.Width); x++ {
			if image.Pt(x, y).In(img.Bounds()) {
				colors[color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)] = struct{}{}
			}
		}
	}
	return len(colors) > 1
}

func cleanupWeaponIconCache(t *testing.T, resource fyne.Resource) {
	t.Helper()
	cachePath := filepath.Join(os.TempDir(), "mbii-fa-cache", resource.Name())
	t.Cleanup(func() { _ = os.Remove(cachePath) })
}

func TestWeaponOverrideRowIconIdentity(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(&FoundryTheme{})
	defer app.Quit()

	root := t.TempDir()
	targetColor := color.NRGBA{R: 229, G: 17, B: 53, A: 255}
	baseColor := color.NRGBA{R: 17, G: 231, B: 91, A: 255}
	customColor := color.NRGBA{R: 41, G: 83, B: 227, A: 255}
	writeWeaponIconFixture(t, root, "gfx/hud/w_icon_target_fixture.png", targetColor)
	writeWeaponIconFixture(t, root, "gfx/hud/w_icon_base_fixture.png", baseColor)
	writeWeaponIconFixture(t, root, "gfx/hud/custom_override_fixture.png", customColor)
	browser := weaponIconBrowserFixture(t, root)

	tests := []struct {
		name      string
		override  parsers.WeaponInfo
		want      *color.NRGBA
		forbidden color.NRGBA
	}{
		{
			name: "explicit icon wins",
			override: parsers.WeaponInfo{
				Icon: "gfx/hud/custom_override_fixture.png", WeaponToReplace: "WP_TARGET_FIXTURE", WeaponBasedOff: "WP_BASE_FIXTURE",
			},
			want: &customColor,
		},
		{
			name: "replacement target wins over base",
			override: parsers.WeaponInfo{
				WeaponToReplace: "WP_TARGET_FIXTURE", WeaponBasedOff: "WP_BASE_FIXTURE",
			},
			want: &targetColor,
		},
		{
			name: "base is authoring fallback without target",
			override: parsers.WeaponInfo{
				WeaponBasedOff: "WP_BASE_FIXTURE",
			},
			want: &baseColor,
		},
		{
			name: "missing target never borrows base identity",
			override: parsers.WeaponInfo{
				WeaponToReplace: "WP_MISSING_TARGET_FIXTURE", WeaponBasedOff: "WP_BASE_FIXTURE",
			},
			forbidden: baseColor,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			editor := &MBCHEditor{
				character:    &parsers.MBCHCharacter{WeaponOverrides: []parsers.WeaponInfo{tc.override}},
				assetBrowser: browser,
				iconResolver: NewIconResolver(browser.vfs),
			}
			ui := NewWeaponInfoUI(editor)
			row := ui.weaponList.CreateItem()
			ui.weaponList.UpdateItem(0, row)

			window := test.NewWindow(row)
			rowHeight := row.MinSize().Height
			if rowHeight < 44 {
				rowHeight = 44
			}
			window.Resize(fyne.NewSize(280, rowHeight))
			captured := window.Canvas().Capture()
			window.Close()

			border := row.(*fyne.Container)
			leftCell := border.Objects[1].(*fyne.Container)
			iconTile := leftCell.Objects[0].(*canvas.Image)
			if iconTile.Resource == nil {
				t.Fatal("row icon has no renderable resource")
			}

			if tc.want != nil {
				cleanupWeaponIconCache(t, iconTile.Resource)
				if got := resourcePixel(t, iconTile.Resource); got != *tc.want {
					t.Fatalf("resolved icon pixel = %v, want %v", got, *tc.want)
				}
				if countRenderedColor(captured, *tc.want) == 0 {
					t.Fatalf("resolved %v icon did not render in the row", *tc.want)
				}
				return
			}

			if countRenderedColor(captured, tc.forbidden) != 0 {
				t.Fatal("missing target visibly borrowed WeaponBasedOff artwork")
			}
			iconPosition := leftCell.Position().Add(iconTile.Position())
			if !renderedRegionHasContrast(captured, iconPosition, iconTile.Size()) {
				t.Fatal("missing target did not visibly render a generic icon")
			}
		})
	}
}
