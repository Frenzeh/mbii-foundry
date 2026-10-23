package main

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func attributeGridTiles(root fyne.CanvasObject) []*AttributeToggleWidget {
	if tile, ok := root.(*AttributeToggleWidget); ok {
		return []*AttributeToggleWidget{tile}
	}
	var tiles []*AttributeToggleWidget
	for _, child := range childObjects(root) {
		tiles = append(tiles, attributeGridTiles(child)...)
	}
	return tiles
}

func findAttributeGridTab(root fyne.CanvasObject) (*container.AppTabs, *container.TabItem) {
	if tabs, ok := root.(*container.AppTabs); ok {
		for _, item := range tabs.Items {
			if item.Text == "Attributes" {
				return tabs, item
			}
		}
	}
	for _, child := range childObjects(root) {
		if tabs, item := findAttributeGridTab(child); item != nil {
			return tabs, item
		}
	}
	return nil, nil
}

func attributeGridTileByID(tiles []*AttributeToggleWidget, id string) *AttributeToggleWidget {
	for _, tile := range tiles {
		if tile.ID == id {
			return tile
		}
	}
	return nil
}

func assertRenderedAttributeTile(t *testing.T, tile *AttributeToggleWidget) {
	t.Helper()
	if tile == nil {
		t.Fatal("attribute tile is missing")
	}
	if tile.Size().Width < 240 || tile.Size().Height < 40 {
		t.Fatalf("attribute tile is too small to use: %v", tile.Size())
	}

	if !tile.titleText.Visible() || tile.titleText.Text == "" || tile.titleText.Size().IsZero() {
		t.Fatalf("attribute title is not visibly laid out: text=%q size=%v visible=%v",
			tile.titleText.Text, tile.titleText.Size(), tile.titleText.Visible())
	}
	if _, ok := objectPositionWithin(tile, tile.titleText); !ok {
		t.Fatal("attribute title is absent from the rendered tile tree")
	}
	if len(tile.buttons) == 0 || !tile.buttons[0].Visible() || tile.buttons[0].Size().IsZero() {
		t.Fatal("attribute level controls are not visibly laid out")
	}
	if _, ok := objectPositionWithin(tile, tile.buttons[0]); !ok {
		t.Fatal("attribute level controls are absent from the rendered tile tree")
	}
}

func TestFullAttributeCatalogDrawsAfterSelectingAttributesTab(t *testing.T) {
	usePublicCatalogs(t)
	ui := test.NewApp()
	ui.Settings().SetTheme(&FoundryTheme{})
	defer ui.Quit()

	editor := NewMBCHEditor(nil)
	window := ui.NewWindow("full attribute catalog regression")
	window.Resize(fyne.NewSize(1200, 760))
	window.SetContent(editor.GetContent())
	window.Show()
	defer window.Close()
	window.Canvas().Capture() // Full editor initially lays out the Profile tab.

	tabs, attributesTab := findAttributeGridTab(editor.GetContent())
	if attributesTab == nil {
		t.Fatal("full editor has no Attributes tab")
	}
	tabs.Select(attributesTab)
	window.Canvas().Capture()

	tiles := attributeGridTiles(editor.attrGrid.content)
	pistol := attributeGridTileByID(tiles, "MB_ATT_PISTOL")
	heavy := attributeGridTileByID(tiles, "MB_ATT_CLONERIFLE")
	assertRenderedAttributeTile(t, pistol)
	assertRenderedAttributeTile(t, heavy)

	viewport := window.Canvas().Size()
	visibleDrawnTile := false
	for _, tile := range tiles {
		position, ok := objectPositionWithin(window.Content(), tile)
		if !ok || position.X < 0 || position.Y < 0 ||
			position.X+tile.Size().Width > viewport.Width+1 ||
			position.Y+tile.Size().Height > viewport.Height+1 {
			continue
		}
		if _, ok := objectPositionWithin(tile, tile.titleText); ok && !tile.titleText.Size().IsZero() {
			visibleDrawnTile = true
			break
		}
	}
	if !visibleDrawnTile {
		t.Fatalf("selected Attributes tab has no fully visible drawn tile in viewport %v", viewport)
	}
}

func TestAttributeGridTilesRemainVisibleAndEditableInsideTabScroll(t *testing.T) {
	usePublicCatalogs(t)
	for _, width := range []float32{360, 900} {
		t.Run(fmt.Sprintf("width_%.0f", width), func(t *testing.T) {
			ui := test.NewApp()
			ui.Settings().SetTheme(&FoundryTheme{})
			defer ui.Quit()

			character := parsers.NewMBCHCharacter()
			grid := NewAttributeGrid(character.Attributes, func(attributes string) {
				character.Attributes = attributes
			}, nil, nil)
			grid.search.SetText("Force Push")

			window := ui.NewWindow("attribute grid regression")
			window.Resize(fyne.NewSize(width, 560))
			window.SetContent(wrapForTab(grid.GetContent()))
			window.Show()
			defer window.Close()
			window.Canvas().Capture()

			tiles := attributeGridTiles(grid.content)
			if len(tiles) != 1 || tiles[0].ID != "MB_ATT_FP_PUSH" {
				t.Fatalf("filter exposed tiles %#v, want only MB_ATT_FP_PUSH", tiles)
			}
			tile := tiles[0]
			position, ok := objectPositionWithin(window.Content(), tile)
			if !ok {
				t.Fatal("filtered attribute tile is outside the tab content tree")
			}
			tileSize := tile.Size()
			viewport := window.Canvas().Size()
			if tileSize.Width < 240 || tileSize.Height < 40 {
				t.Fatalf("attribute tile is too small to use: %v", tileSize)
			}
			if position.X < 0 || position.Y < 0 ||
				position.X+tileSize.Width > viewport.Width+1 ||
				position.Y+tileSize.Height > viewport.Height+1 {
				t.Fatalf("attribute tile is clipped at compact tab bounds: pos=%v size=%v viewport=%v",
					position, tileSize, viewport)
			}

			if len(tile.buttons) < 3 {
				t.Fatal("Force Push does not expose its selectable levels")
			}
			test.Tap(tile.buttons[2])
			source, err := parsers.GenerateMBCH(character)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := parsers.ParseMBCH(source)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.Attributes != "MB_ATT_FP_PUSH,2" {
				t.Fatalf("level selection did not reach generated character source: %q", reopened.Attributes)
			}

			grid.search.SetText("no matching attribute")
			if got := len(attributeGridTiles(grid.content)); got != 0 {
				t.Fatalf("no-result filter left %d selectable tiles", got)
			}
			grid.search.SetText("MB_ATT_FP_PUSH")
			tiles = attributeGridTiles(grid.content)
			if len(tiles) != 1 {
				t.Fatalf("ID filter restored %d tiles, want 1", len(tiles))
			}
			test.Tap(tiles[0].buttons[0])
			source, err = parsers.GenerateMBCH(character)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err = parsers.ParseMBCH(source)
			if err != nil {
				t.Fatal(err)
			}
			if reopened.Attributes != "" {
				t.Fatalf("turning the visible level off left generated attributes %q", reopened.Attributes)
			}
		})
	}
}
