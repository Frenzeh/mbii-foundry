package main

import (
	"fmt"
	"image/color"
	"strconv"
)

var sourcePalettePresets = []SourcePalette{
	{
		Name: "Foundry Dark",
		Colors: [sourceSyntaxClassCount]color.NRGBA{
			{R: 0xe6, G: 0xe9, B: 0xef, A: 0xff},
			{R: 0x7f, G: 0x9a, B: 0x76, A: 0xff},
			{R: 0xe6, G: 0xb4, B: 0x73, A: 0xff},
			{R: 0x8d, G: 0xca, B: 0xff, A: 0xff},
			{R: 0xd7, G: 0x9d, B: 0xff, A: 0xff},
			{R: 0xff, G: 0xd1, B: 0x66, A: 0xff},
			{R: 0xa9, G: 0xb2, B: 0xc3, A: 0xff},
		},
		Background:      color.NRGBA{R: 0x17, G: 0x1a, B: 0x21, A: 0xff},
		Selection:       color.NRGBA{R: 0x35, G: 0x5f, B: 0x9a, A: 0xff},
		Caret:           color.NRGBA{R: 0xf5, G: 0xf7, B: 0xfa, A: 0xff},
		LineNumber:      color.NRGBA{R: 0x7f, G: 0x88, B: 0x99, A: 0xff},
		Error:           color.NRGBA{R: 0xff, G: 0x5c, B: 0x68, A: 0xff},
		Warning:         color.NRGBA{R: 0xff, G: 0xc8, B: 0x57, A: 0xff},
		MatchingBracket: color.NRGBA{R: 0x46, G: 0x6a, B: 0x47, A: 0xff},
	},
	{
		Name: "Monokai",
		Colors: [sourceSyntaxClassCount]color.NRGBA{
			{R: 0xf8, G: 0xf8, B: 0xf2, A: 0xff},
			{R: 0x75, G: 0x71, B: 0x5e, A: 0xff},
			{R: 0xe6, G: 0xdb, B: 0x74, A: 0xff},
			{R: 0xae, G: 0x81, B: 0xff, A: 0xff},
			{R: 0x66, G: 0xd9, B: 0xef, A: 0xff},
			{R: 0xa6, G: 0xe2, B: 0x2e, A: 0xff},
			{R: 0xf9, G: 0x26, B: 0x72, A: 0xff},
		},
		Background:      color.NRGBA{R: 0x27, G: 0x28, B: 0x22, A: 0xff},
		Selection:       color.NRGBA{R: 0x49, G: 0x4b, B: 0x40, A: 0xff},
		Caret:           color.NRGBA{R: 0xf8, G: 0xf8, B: 0xf0, A: 0xff},
		LineNumber:      color.NRGBA{R: 0x90, G: 0x8f, B: 0x7e, A: 0xff},
		Error:           color.NRGBA{R: 0xf9, G: 0x26, B: 0x72, A: 0xff},
		Warning:         color.NRGBA{R: 0xfd, G: 0x97, B: 0x1f, A: 0xff},
		MatchingBracket: color.NRGBA{R: 0x55, G: 0x60, B: 0x3a, A: 0xff},
	},
	{
		Name: "Light",
		Colors: [sourceSyntaxClassCount]color.NRGBA{
			{R: 0x22, G: 0x27, B: 0x2e, A: 0xff},
			{R: 0x55, G: 0x77, B: 0x55, A: 0xff},
			{R: 0x9a, G: 0x3e, B: 0x20, A: 0xff},
			{R: 0x0b, G: 0x62, B: 0x9c, A: 0xff},
			{R: 0x6f, G: 0x42, B: 0xc1, A: 0xff},
			{R: 0x75, G: 0x52, B: 0x00, A: 0xff},
			{R: 0x4a, G: 0x55, B: 0x63, A: 0xff},
		},
		Background:      color.NRGBA{R: 0xfa, G: 0xfb, B: 0xfc, A: 0xff},
		Selection:       color.NRGBA{R: 0xb7, G: 0xd7, B: 0xff, A: 0xff},
		Caret:           color.NRGBA{R: 0x11, G: 0x18, B: 0x20, A: 0xff},
		LineNumber:      color.NRGBA{R: 0x6a, G: 0x73, B: 0x7d, A: 0xff},
		Error:           color.NRGBA{R: 0xb7, G: 0x1c, B: 0x1c, A: 0xff},
		Warning:         color.NRGBA{R: 0x9a, G: 0x61, B: 0x00, A: 0xff},
		MatchingBracket: color.NRGBA{R: 0xc7, G: 0xe8, B: 0xbf, A: 0xff},
	},
}

// SourcePalettes returns independent copies of the built-in, theme-free palettes.
func SourcePalettes() []SourcePalette {
	palettes := make([]SourcePalette, len(sourcePalettePresets))
	copy(palettes, sourcePalettePresets)
	return palettes
}

func SourcePaletteByName(name string) (SourcePalette, bool) {
	for _, palette := range sourcePalettePresets {
		if palette.Name == name {
			return palette, true
		}
	}
	return SourcePalette{}, false
}

func SourcePaletteWithOverrides(base SourcePalette, overrides map[string]string) (SourcePalette, error) {
	result := base
	for key, value := range overrides {
		parsed, err := parseSourceColor(value)
		if err != nil {
			return base, fmt.Errorf("%s: %w", key, err)
		}
		switch key {
		case "background":
			result.Background = parsed
		case "text":
			result.Colors[SourceSyntaxText] = parsed
		case "comment":
			result.Colors[SourceSyntaxComment] = parsed
		case "string":
			result.Colors[SourceSyntaxString] = parsed
		case "number":
			result.Colors[SourceSyntaxNumber] = parsed
		case "constant":
			result.Colors[SourceSyntaxConstant] = parsed
		case "header":
			result.Colors[SourceSyntaxHeader] = parsed
		case "punctuation":
			result.Colors[SourceSyntaxPunctuation] = parsed
		case "selection":
			result.Selection = parsed
		case "caret":
			result.Caret = parsed
		case "line_number":
			result.LineNumber = parsed
		case "error":
			result.Error = parsed
		case "warning":
			result.Warning = parsed
		case "matching_bracket":
			result.MatchingBracket = parsed
		default:
			return base, fmt.Errorf("unknown source palette override %q", key)
		}
	}
	return result, nil
}

func parseSourceColor(value string) (color.NRGBA, error) {
	if len(value) != 7 && len(value) != 9 || len(value) == 0 || value[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("color must be #RRGGBB or #RRGGBBAA")
	}
	n, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("invalid hexadecimal color %q", value)
	}
	if len(value) == 7 {
		return color.NRGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 0xff}, nil
	}
	return color.NRGBA{R: uint8(n >> 24), G: uint8(n >> 16), B: uint8(n >> 8), A: uint8(n)}, nil
}
