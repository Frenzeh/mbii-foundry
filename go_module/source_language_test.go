package main

import (
	"strings"
	"testing"
)

func TestSourceAnalysisUsesByteRangesRuneColumnsAndRealSyntaxTokenizer(t *testing.T) {
	text := "ClassInfo\r\n{\r\n\tname \"écho\"\r\n\tMBClass MB_CLASS_SOLDIER\r\n\tlabel café\r\n\tnote \"é"
	analysis := NewSourceLanguage(nil).Analyze(text, SourceFormatMBCH)

	covered := 0
	for _, run := range analysis.Syntax {
		if run.Range.Start != covered || run.Range.End < run.Range.Start || run.Range.End > len(text) {
			t.Fatalf("non-contiguous syntax run %+v after byte %d", run, covered)
		}
		covered = run.Range.End
	}
	if covered != len(text) {
		t.Fatalf("syntax covered %d of %d UTF-8 bytes", covered, len(text))
	}

	var quote *SourceDiagnostic
	for i := range analysis.Diagnostics {
		if strings.Contains(analysis.Diagnostics[i].Message, "endquote") {
			quote = &analysis.Diagnostics[i]
			break
		}
	}
	if quote == nil {
		t.Fatal("unterminated quote diagnostic was not reported")
	}
	if quote.Line != 6 || quote.Column != 7 {
		t.Fatalf("quote location = %d:%d, want 6:7 (Unicode rune column)", quote.Line, quote.Column)
	}
	if quote.Range.Start != strings.LastIndex(text, `"`) {
		t.Fatalf("quote byte offset = %d, want %d", quote.Range.Start, strings.LastIndex(text, `"`))
	}
}

func TestSourceBracketPairsIgnoreCommentsAndQuotedBraces(t *testing.T) {
	text := "ClassInfo {\n name \"{not a block}\" // } comment\n nested (value)\n}\n"
	analysis := NewSourceLanguage(nil).Analyze(text, SourceFormatMBCH)
	if len(analysis.Brackets) != 2 {
		t.Fatalf("bracket pairs = %+v, want outer braces and parentheses only", analysis.Brackets)
	}
	for _, pair := range analysis.Brackets {
		if text[pair.Open] == '{' && text[pair.Close] != '}' || text[pair.Open] == '(' && text[pair.Close] != ')' {
			t.Fatalf("mismatched bracket pair %+v", pair)
		}
	}
}

func TestSourceContextAtKeepsUTF8ByteReplacementAndMultilineQuote(t *testing.T) {
	text := "ClassInfo\n{\n\tname \"café\nsecond line\"\n\tattributes \"MB_ATT_STAMINA,3|MB_ATT_PIS\"\n}\n"
	caret := strings.Index(text, "second") + len("second")
	context := SourceContextAt(text, caret, SourceFormatMBCH)
	if context.Block != "ClassInfo" || context.Field != "name" || !context.InString {
		t.Fatalf("multiline quoted context = %+v", context)
	}
	if got := text[context.Replace.Start:context.Replace.End]; got != "café\nsecond line" {
		t.Fatalf("multiline replacement = %q", got)
	}

	caret = strings.Index(text, "MB_ATT_PIS") + len("MB_ATT_PIS")
	context = SourceContextAt(text, caret, SourceFormatMBCH)
	if context.Field != "attributes" || context.Prefix != "MB_ATT_PIS" || !context.InString {
		t.Fatalf("ranked-list context = %+v", context)
	}
	if got := text[context.Replace.Start:context.Replace.End]; got != "MB_ATT_PIS" {
		t.Fatalf("list replacement widened to %q", got)
	}
}
