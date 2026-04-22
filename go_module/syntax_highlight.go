package main

// Syntax highlighter for MBII data files (.mbch, .sab, .veh, .siege).
// These formats share a lineage with Quake 3 / Jedi Academy script
// files: named blocks with brace bodies, whitespace-delimited
// key/value pairs, // line comments, /* */ block comments, double-
// quoted strings, and ALL_CAPS_CONSTANTS (MB_CLASS_*, WP_*, FP_*,
// CFL_*, MB_ATT_*, SET_* etc.).
//
// Tokens recognized:
//   * tkComment   — "// ..."   "/* ... */"
//   * tkString    — "..."
//   * tkNumber    — 123, -1.5
//   * tkConst     — ALL_CAPS_WITH_UNDERSCORES identifiers (game enums)
//   * tkHeader    — identifier immediately followed by "{" (ClassInfo,
//                   WeaponInfo0, Team1 etc.) — rendered bold
//   * tkIdent     — every other identifier (mostly field names)
//   * tkPunct     — { } , | ( ) : ;
//   * tkWhitespace
//   * tkPlain     — fallback
//
// These are matched against theme color names registered in
// FoundryTheme.Color (see main.go) so the palette scales with the
// active accent theme.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type synTokenKind int

const (
	tkPlain synTokenKind = iota
	tkWhitespace
	tkComment
	tkString
	tkNumber
	tkIdent
	tkConst
	tkHeader
	tkPunct
)

// Custom theme color names for syntax highlighting. Registered in
// FoundryTheme.Color so switching the primary accent scales them.
const (
	ColorNameSyntaxComment fyne.ThemeColorName = "foundrySyntaxComment"
	ColorNameSyntaxString  fyne.ThemeColorName = "foundrySyntaxString"
	ColorNameSyntaxNumber  fyne.ThemeColorName = "foundrySyntaxNumber"
	ColorNameSyntaxConst   fyne.ThemeColorName = "foundrySyntaxConst"
	ColorNameSyntaxHeader  fyne.ThemeColorName = "foundrySyntaxHeader"
	ColorNameSyntaxPunct   fyne.ThemeColorName = "foundrySyntaxPunct"
)

type synToken struct {
	kind synTokenKind
	text string
}

// tokenizeMBII splits the input into syntactic tokens for
// highlighting. Defensive — any byte it can't classify gets emitted
// as tkPlain so we never lose content.
func tokenizeMBII(src string) []synToken {
	out := make([]synToken, 0, 256)
	i := 0
	n := len(src)
	for i < n {
		c := src[i]
		switch {
		case c == '/' && i+1 < n && src[i+1] == '/':
			// Line comment to EOL.
			j := i + 2
			for j < n && src[j] != '\n' {
				j++
			}
			out = append(out, synToken{tkComment, src[i:j]})
			i = j
		case c == '/' && i+1 < n && src[i+1] == '*':
			// Block comment. Hunt for closing */; if absent, swallow
			// to EOF so we don't drop text.
			j := i + 2
			for j+1 < n && !(src[j] == '*' && src[j+1] == '/') {
				j++
			}
			if j+1 < n {
				j += 2
			} else {
				j = n
			}
			out = append(out, synToken{tkComment, src[i:j]})
			i = j
		case c == '"':
			j := i + 1
			for j < n && src[j] != '"' && src[j] != '\n' {
				// Naive backslash-escape skip; strings in these
				// formats rarely contain escapes but the parser is
				// tolerant either way.
				if src[j] == '\\' && j+1 < n {
					j++
				}
				j++
			}
			if j < n && src[j] == '"' {
				j++
			}
			out = append(out, synToken{tkString, src[i:j]})
			i = j
		case isSynDigit(c) || (c == '-' && i+1 < n && isSynDigit(src[i+1])):
			j := i
			if c == '-' {
				j++
			}
			for j < n && (isSynDigit(src[j]) || src[j] == '.') {
				j++
			}
			out = append(out, synToken{tkNumber, src[i:j]})
			i = j
		case isSynIdentStart(c):
			j := i
			for j < n && isSynIdentPart(src[j]) {
				j++
			}
			word := src[i:j]
			kind := tkIdent
			if isSynAllCaps(word) {
				kind = tkConst
			}
			out = append(out, synToken{kind, word})
			i = j
		case c == '{' || c == '}' || c == ',' || c == '|' || c == '(' || c == ')' || c == ':' || c == ';':
			out = append(out, synToken{tkPunct, string(c)})
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			j := i
			for j < n && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			out = append(out, synToken{tkWhitespace, src[i:j]})
			i = j
		default:
			out = append(out, synToken{tkPlain, string(c)})
			i++
		}
	}

	// Second pass: promote any tkIdent that's followed (across
	// whitespace) by "{" to tkHeader. That catches ClassInfo,
	// WeaponInfo0, Team1, Objective3, etc.
	for idx := range out {
		if out[idx].kind != tkIdent && out[idx].kind != tkConst {
			continue
		}
		for j := idx + 1; j < len(out); j++ {
			if out[j].kind == tkWhitespace {
				continue
			}
			if out[j].kind == tkPunct && out[j].text == "{" {
				out[idx].kind = tkHeader
			}
			break
		}
	}
	return out
}

// highlightedSegments renders tokens as Fyne RichText segments with
// per-kind theme color names + bold for headers. Whitespace is kept
// inline so newlines break lines naturally.
func highlightedSegments(src string) []widget.RichTextSegment {
	tokens := tokenizeMBII(src)
	segs := make([]widget.RichTextSegment, 0, len(tokens))
	for _, t := range tokens {
		style := widget.RichTextStyle{
			Inline:    true,
			TextStyle: fyne.TextStyle{Monospace: true},
		}
		switch t.kind {
		case tkComment:
			style.ColorName = ColorNameSyntaxComment
			style.TextStyle.Italic = true
		case tkString:
			style.ColorName = ColorNameSyntaxString
		case tkNumber:
			style.ColorName = ColorNameSyntaxNumber
		case tkConst:
			style.ColorName = ColorNameSyntaxConst
		case tkHeader:
			style.ColorName = ColorNameSyntaxHeader
			style.TextStyle.Bold = true
		case tkPunct:
			style.ColorName = ColorNameSyntaxPunct
		}
		segs = append(segs, &widget.TextSegment{Text: t.text, Style: style})
	}
	return segs
}

func isSynDigit(c byte) bool     { return c >= '0' && c <= '9' }
func isSynLetter(c byte) bool    { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isSynIdentStart(c byte) bool { return isSynLetter(c) || c == '_' }
func isSynIdentPart(c byte) bool {
	return isSynIdentStart(c) || isSynDigit(c)
}

// isSynAllCaps reports whether word is an ALL_CAPS_WITH_UNDERSCORES
// style identifier — indicating an MBII enum constant. Requires at
// least one letter + underscore so lone "X" or "Y" don't get flagged.
func isSynAllCaps(word string) bool {
	if len(word) < 2 {
		return false
	}
	hasUnderscore := false
	hasLetter := false
	for i := 0; i < len(word); i++ {
		c := word[i]
		switch {
		case c == '_':
			hasUnderscore = true
		case c >= 'A' && c <= 'Z':
			hasLetter = true
		case c >= '0' && c <= '9':
			// digits allowed inside a constant (e.g. MB_CTF2)
		default:
			return false
		}
	}
	return hasUnderscore && hasLetter
}
