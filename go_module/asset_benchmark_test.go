package main

import (
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// To run this measurement harness:
// 1. Set environment variables for the paths:
//    export MBII_GAMEDATA_PATH="path/to/MBII_GameData"
//    export MBII_TEXTASSETS_PATH="path/to/mbii/TextAssets"
// 2. Run the test:
//    go test -v -run=^$ -bench=BenchmarkVFS -benchmem
//
// Astra profiling notes:
// Do not hardcode local paths in this source file.
// Pass the paths via the environment variables above.

func reportBenchmarkDataset(b *testing.B, vfs *VirtualFileSystem) {
	b.Helper()
	vfs.mu.RLock()
	assetCount := len(vfs.Index)
	sourceCount := len(vfs.Sources)
	vfs.mu.RUnlock()
	b.ReportMetric(float64(assetCount), "assets")
	b.ReportMetric(float64(len(gatherModelsMap(vfs))), "models")
	b.ReportMetric(float64(sourceCount), "sources")
}

func benchmarkAssetBrowser(vfs *VirtualFileSystem) *AssetBrowser {
	return &AssetBrowser{vfs: vfs, shaderResolver: NewShaderResolver(vfs)}
}

func BenchmarkVFSCold(b *testing.B) {
	gameDataPath := os.Getenv("MBII_GAMEDATA_PATH")
	textAssetsPath := os.Getenv("MBII_TEXTASSETS_PATH")

	if gameDataPath == "" || textAssetsPath == "" {
		b.Skip("Skipping full-index measurement: MBII_GAMEDATA_PATH and MBII_TEXTASSETS_PATH env vars not set")
	}

	b.ResetTimer()
	for range b.N {
		vfs := NewVirtualFileSystem(gameDataPath, textAssetsPath)
		if err := vfs.Refresh(); err != nil {
			b.Fatalf("VFS Refresh failed: %v", err)
		}
	}
}

func BenchmarkVFSWarm(b *testing.B) {
	gameDataPath := os.Getenv("MBII_GAMEDATA_PATH")
	textAssetsPath := os.Getenv("MBII_TEXTASSETS_PATH")

	if gameDataPath == "" || textAssetsPath == "" {
		b.Skip("Skipping full-index measurement: MBII_GAMEDATA_PATH and MBII_TEXTASSETS_PATH env vars not set")
	}

	vfs := NewVirtualFileSystem(gameDataPath, textAssetsPath)
	if err := vfs.Refresh(); err != nil {
		b.Fatalf("Cold VFS Refresh failed: %v", err)
	}

	b.ResetTimer()
	for range b.N {
		if err := vfs.Refresh(); err != nil {
			b.Fatalf("Warm VFS Refresh failed: %v", err)
		}
	}
}

func BenchmarkGalleryOpening(b *testing.B) {
	gameDataPath := os.Getenv("MBII_GAMEDATA_PATH")
	textAssetsPath := os.Getenv("MBII_TEXTASSETS_PATH")

	if gameDataPath == "" || textAssetsPath == "" {
		b.Skip("Skipping full-index measurement: paths not set")
	}

	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Bench")

	vfs := NewVirtualFileSystem(gameDataPath, textAssetsPath)
	if err := vfs.Refresh(); err != nil {
		b.Fatalf("VFS Refresh failed: %v", err)
	}
	ab := benchmarkAssetBrowser(vfs)

	b.ResetTimer()
	for range b.N {
		ShowModelGalleryModal(win, vfs, ab, func(m, s string) {})
		if popup := win.Canvas().Overlays().Top(); popup != nil {
			popup.Hide()
		}
	}
	reportBenchmarkDataset(b, vfs)
}

func BenchmarkGalleryFiltering(b *testing.B) {
	gameDataPath := os.Getenv("MBII_GAMEDATA_PATH")
	textAssetsPath := os.Getenv("MBII_TEXTASSETS_PATH")

	if gameDataPath == "" || textAssetsPath == "" {
		b.Skip("Skipping full-index measurement: paths not set")
	}

	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Bench")

	vfs := NewVirtualFileSystem(gameDataPath, textAssetsPath)
	if err := vfs.Refresh(); err != nil {
		b.Fatalf("VFS Refresh failed: %v", err)
	}
	ab := benchmarkAssetBrowser(vfs)

	// Display the real modal
	ShowModelGalleryModal(win, vfs, ab, func(m, s string) {})

	// Find the Search Entry in the modal popup
	top := win.Canvas().Overlays().Top()
	if top == nil {
		b.Fatalf("No modal overlay found")
	}
	popup, ok := top.(*widget.PopUp)
	if !ok {
		b.Fatalf("Top overlay is not a PopUp")
	}
	var searchEntry *widget.Entry
	var findEntry func(fyne.CanvasObject) *widget.Entry
	findEntry = func(obj fyne.CanvasObject) *widget.Entry {
		if e, ok := obj.(*widget.Entry); ok {
			return e
		}
		if c, ok := obj.(*fyne.Container); ok {
			for _, child := range c.Objects {
				if e := findEntry(child); e != nil {
					return e
				}
			}
		}
		return nil
	}

	searchEntry = findEntry(popup.Content)
	if searchEntry == nil {
		b.Fatalf("Could not find search entry in model gallery")
	}

	b.ResetTimer()
	for range b.N {
		// Type to trigger the real renderCards callback
		searchEntry.SetText("jedi")
		searchEntry.SetText("")
	}
	reportBenchmarkDataset(b, vfs)
}

func BenchmarkEditorDirtyRefresh(b *testing.B) {
	gameDataPath := os.Getenv("MBII_GAMEDATA_PATH")
	textAssetsPath := os.Getenv("MBII_TEXTASSETS_PATH")

	if gameDataPath == "" || textAssetsPath == "" {
		b.Skip("Skipping full-index measurement: paths not set")
	}

	app := test.NewApp()
	app.Settings().SetTheme(theme.DefaultTheme())
	defer app.Quit()
	win := app.NewWindow("Bench")

	vfs := NewVirtualFileSystem(gameDataPath, textAssetsPath)
	if err := vfs.Refresh(); err != nil {
		b.Fatalf("VFS Refresh failed: %v", err)
	}
	ab := benchmarkAssetBrowser(vfs)

	foundryApp := &App{mainWindow: win, fileManager: NewFileManager(""), assetBrowser: ab}
	editor := NewMBCHEditor(foundryApp)

	// Real workload requires parsing/generating the source string and updating diagnostics
	editor.SetOnSourceChanged(func() {
		// In production this is called by SourcePanel.refreshFromProvider -> provider.GenerateSource()
		_ = editor.GenerateSource()
	})

	// Use bound controls so each dirty snapshot exercises a valid editor session.
	editor.classPicker.SetSelected("MB_CLASS_JEDI")
	editor.attributesEntry.SetText("MB_ATT_SABER_DEFENSE,3|MB_ATT_SABER_OFFENSE,3|MB_ATT_FORCEBLOCK,3")

	b.ResetTimer()
	for range b.N {
		editor.isDirty = false
		editor.markDirty()
	}
	reportBenchmarkDataset(b, vfs)
}
