package main

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func sortedSourcePaletteNames() []string {
	palettes := SourcePalettes()
	names := make([]string, 0, len(palettes))
	for _, palette := range palettes {
		names = append(names, palette.Name)
	}
	sort.Strings(names)
	return names
}

var sourcePaletteOverrideKeys = []string{
	"background",
	"text",
	"comment",
	"string",
	"number",
	"constant",
	"header",
	"punctuation",
	"selection",
	"caret",
	"line_number",
	"error",
	"warning",
	"matching_bracket",
}

type sourcePalettePreferences struct {
	root    fyne.CanvasObject
	preset  *widget.Select
	entries map[string]*widget.Entry
	preview *SourceCodeEditor
	error   *widget.Label
}

func newSourcePalettePreferences(name string, overrides map[string]string) *sourcePalettePreferences {
	p := &sourcePalettePreferences{
		entries: make(map[string]*widget.Entry, len(sourcePaletteOverrideKeys)),
		preview: NewSourceCodeEditor(),
		error:   widget.NewLabel(""),
	}
	p.error.Wrapping = fyne.TextWrapWord
	p.preview.SetReadOnly(true)
	previewText := "ClassInfo {\n\tname \"Preview\"\n\tweapons WP_BLASTER\n}\n// Custom colours update here"
	p.preview.SetText(previewText)
	p.preview.SetAnalysis(sourcePalettePreviewAnalysis(previewText))

	p.preset = widget.NewSelect(sortedSourcePaletteNames(), func(string) { p.refreshPreview() })
	if name == "" && len(p.preset.Options) > 0 {
		name = p.preset.Options[0]
	}
	if _, ok := SourcePaletteByName(name); !ok && len(p.preset.Options) > 0 {
		name = p.preset.Options[0]
	}

	formItems := []*widget.FormItem{widget.NewFormItem("Preset", p.preset)}
	for _, key := range sourcePaletteOverrideKeys {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Preset default or #RRGGBB")
		entry.SetText(overrides[key])
		entry.OnChanged = func(string) { p.refreshPreview() }
		p.entries[key] = entry
		label := strings.ReplaceAll(strings.Title(strings.ReplaceAll(key, "_", " ")), "Rgba", "RGBA")
		formItems = append(formItems, widget.NewFormItem(label, entry))
	}
	p.preset.SetSelected(name)
	form := widget.NewForm(formItems...)
	preview := container.NewBorder(widget.NewLabel("Live syntax preview"), nil, nil, nil, p.preview)
	p.root = container.NewVBox(
		widget.NewLabel("Leave a colour blank to use the preset. Custom values accept #RRGGBB or #RRGGBBAA."),
		p.error,
		form,
		preview,
	)
	p.refreshPreview()
	return p
}

func (p *sourcePalettePreferences) Content() fyne.CanvasObject { return p.root }

func (p *sourcePalettePreferences) Value() (string, map[string]string, error) {
	if p == nil || p.preset.Selected == "" {
		return "", nil, fmt.Errorf("select a syntax colour preset")
	}
	overrides := make(map[string]string)
	for _, key := range sourcePaletteOverrideKeys {
		value := strings.TrimSpace(p.entries[key].Text)
		if value != "" {
			overrides[key] = value
		}
	}
	if _, err := configuredSourcePalette(p.preset.Selected, overrides); err != nil {
		return "", nil, err
	}
	return p.preset.Selected, overrides, nil
}

func (p *sourcePalettePreferences) refreshPreview() {
	if p == nil || p.preview == nil || p.preset == nil || p.preset.Selected == "" {
		return
	}
	name, overrides, err := p.Value()
	if err != nil {
		p.error.SetText("Syntax colours: " + err.Error())
		return
	}
	palette, err := configuredSourcePalette(name, overrides)
	if err != nil {
		p.error.SetText("Syntax colours: " + err.Error())
		return
	}
	p.error.SetText("")
	p.preview.SetPalette(palette)
}

func sourcePalettePreviewAnalysis(text string) SourceAnalysis {
	classes := []struct {
		text  string
		class SourceSyntaxClass
	}{
		{"ClassInfo", SourceSyntaxHeader},
		{"{", SourceSyntaxPunctuation},
		{"\"Preview\"", SourceSyntaxString},
		{"WP_BLASTER", SourceSyntaxConstant},
		{"}", SourceSyntaxPunctuation},
		{"// Custom colours update here", SourceSyntaxComment},
	}
	runs := make([]SourceSyntaxRun, 0, len(classes))
	searchFrom := 0
	for _, item := range classes {
		index := strings.Index(text[searchFrom:], item.text)
		if index < 0 {
			continue
		}
		start := searchFrom + index
		runs = append(runs, SourceSyntaxRun{Range: SourceRange{Start: start, End: start + len(item.text)}, Class: item.class})
		searchFrom = start + len(item.text)
	}
	return SourceAnalysis{Syntax: runs, Brackets: []SourceBracketPair{{Open: strings.Index(text, "{"), Close: strings.Index(text, "}")}}}
}
