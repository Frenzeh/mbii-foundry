package main

import (
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type SyntaxHighlighter struct{}

func NewSyntaxHighlighter() *SyntaxHighlighter {
	return &SyntaxHighlighter{}
}

func (sh *SyntaxHighlighter) Highlight(content string) *widget.RichText {
	rt := widget.NewRichText()

	lines := strings.Split(content, "\n")

	for i, line := range lines {
		// Check for comment
		commentIdx := strings.Index(line, "//")
		if commentIdx != -1 {
			// Process part before comment
			if commentIdx > 0 {
				sh.appendLineSegments(rt, line[:commentIdx])
			}
			// Process comment
			rt.Segments = append(rt.Segments, &widget.TextSegment{
				Text: line[commentIdx:],
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNamePlaceHolder,
					Inline:    true,
					TextStyle: fyne.TextStyle{Italic: true, Monospace: true},
				},
			})
		} else {
			sh.appendLineSegments(rt, line)
		}

		// Only add newline if this is not the last segment from Split
		// strings.Split("A\nB", "\n") -> ["A", "B"] -> Add \n after A.
		if i < len(lines)-1 {
			rt.Segments = append(rt.Segments, &widget.TextSegment{
				Text:  "\n",
				Style: widget.RichTextStyle{TextStyle: fyne.TextStyle{Monospace: true}},
			})
		}
	}

	return rt
}

func (sh *SyntaxHighlighter) appendLineSegments(rt *widget.RichText, text string) {
	if strings.TrimSpace(text) == "" {
		rt.Segments = append(rt.Segments, &widget.TextSegment{Text: text, Style: widget.RichTextStyle{TextStyle: fyne.TextStyle{Monospace: true}}})
		return
	}

	// Regex to find strings
	reStr := regexp.MustCompile("\"([^\"]*)\"")

	lastIdx := 0
	matches := reStr.FindAllStringIndex(text, -1)

	for _, match := range matches {
		start, end := match[0], match[1]

		// Process text before string
		if start > lastIdx {
			pre := text[lastIdx:start]
			sh.processNonString(rt, pre)
		}

		// Process string
		rt.Segments = append(rt.Segments, &widget.TextSegment{
			Text: text[start:end],
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNameWarning, // Orange for strings
				Inline:    true,
				TextStyle: fyne.TextStyle{Monospace: true},
			},
		})

		lastIdx = end
	}

	// Process remaining
	if lastIdx < len(text) {
		sh.processNonString(rt, text[lastIdx:])
	}
}

func (sh *SyntaxHighlighter) processNonString(rt *widget.RichText, text string) {
	// Simple heuristic: Key usually comes first
	// But splitting inside a segment is hard without context.
	// Just plain text for now, or simple keyword detection.

	color := theme.ColorNameForeground

	// If it looks like a Key (start of line, no space before it in original line context... hard here)
	// Let's just highlight specific keywords
	if strings.Contains(text, "ClassInfo") || strings.Contains(text, "WeaponInfo") || strings.Contains(text, "ForceInfo") {
		color = theme.ColorNamePrimary
	}

	rt.Segments = append(rt.Segments, &widget.TextSegment{
		Text: text,
		Style: widget.RichTextStyle{
			ColorName: color,
			Inline:    true,
			TextStyle: fyne.TextStyle{Monospace: true},
		},
	})
}
