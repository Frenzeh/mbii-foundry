package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// RGBPreset defines a canonical MBII engine skin color preset.
type RGBPreset struct {
	Name     string
	R, G, B  float64
	HexColor string
	Color    color.Color
}

// MBIIRGBPresets contains the exact floats from the MBII engine & Wiki guide.
var MBIIRGBPresets = []RGBPreset{
	{Name: "Red", R: 0.709, G: 0.120, B: 0.120, HexColor: "#B51E1E", Color: color.RGBA{R: 181, G: 30, B: 30, A: 255}},
	{Name: "Blue", R: 0.180, G: 0.355, B: 0.550, HexColor: "#2E5B8C", Color: color.RGBA{R: 46, G: 91, B: 140, A: 255}},
	{Name: "Gold", R: 0.785, G: 0.630, B: 0.314, HexColor: "#C8A150", Color: color.RGBA{R: 200, G: 161, B: 80, A: 255}},
	{Name: "Orange", R: 0.945, G: 0.510, B: 0.099, HexColor: "#F18219", Color: color.RGBA{R: 241, G: 130, B: 25, A: 255}},
	{Name: "Purple", R: 0.471, G: 0.239, B: 0.353, HexColor: "#783D5A", Color: color.RGBA{R: 120, G: 61, B: 90, A: 255}},
	{Name: "Brown", R: 0.471, G: 0.237, B: 0.080, HexColor: "#783C14", Color: color.RGBA{R: 120, G: 60, B: 20, A: 255}},
	{Name: "Grey", R: 0.502, G: 0.502, B: 0.502, HexColor: "#808080", Color: color.RGBA{R: 128, G: 128, B: 128, A: 255}},
	{Name: "White", R: 1.000, G: 1.000, B: 1.000, HexColor: "#FFFFFF", Color: color.RGBA{R: 255, G: 255, B: 255, A: 255}},
	{Name: "Black", R: 0.000, G: 0.000, B: 0.000, HexColor: "#101010", Color: color.RGBA{R: 20, G: 20, B: 20, A: 255}},
}

// NewRGBPresetBar creates a visual swatch bar for fast MBII skin color selection.
func NewRGBPresetBar(onSelect func(r, g, b float64)) fyne.CanvasObject {
	swatches := make([]fyne.CanvasObject, 0, len(MBIIRGBPresets))

	for _, p := range MBIIRGBPresets {
		preset := p // capture loop variable
		rect := canvas.NewRectangle(preset.Color)
		rect.SetMinSize(fyne.NewSize(16, 16))
		rect.CornerRadius = 3

		btn := widget.NewButton(preset.Name, func() {
			if onSelect != nil {
				onSelect(preset.R, preset.G, preset.B)
			}
		})
		btn.Importance = widget.LowImportance

		swatch := container.NewHBox(rect, btn)
		swatches = append(swatches, swatch)
	}

	header := widget.NewLabel("Engine Color Presets:")
	header.TextStyle = fyne.TextStyle{Bold: true}

	grid := container.NewGridWithColumns(3, swatches...)
	return container.NewVBox(
		header,
		grid,
	)
}

// FormatRGBFloat formats float values into standard 3-decimal MBII format.
func FormatRGBFloat(val float64) string {
	return fmt.Sprintf("%.3f", val)
}
