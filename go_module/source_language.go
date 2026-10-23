package main

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// SourceLanguage provides positioned analysis and context-aware completion for
// the source formats edited by Foundry. It owns no document state: callers may
// safely keep one instance and analyze several buffers.
type SourceLanguage struct {
	assetSuggest func(format SourceFormat, field, prefix string, limit int) []string
}

func NewSourceLanguage(assetSuggest func(format SourceFormat, field, prefix string, limit int) []string) *SourceLanguage {
	return &SourceLanguage{assetSuggest: assetSuggest}
}

func (l *SourceLanguage) Analyze(text string, format SourceFormat) SourceAnalysis {
	analysis := SourceAnalysis{
		Syntax:   sourceSyntaxRuns(text),
		Brackets: sourceBracketPairs(text),
	}
	analysis.Diagnostics = sourceDiagnostics(text, format)
	return analysis
}

func (l *SourceLanguage) Complete(text string, caret int, format SourceFormat) []SourceCompletion {
	return sourceCompletions(l, text, caret, format)
}

type sourceLexKind uint8

const (
	sourceLexWhitespace sourceLexKind = iota
	sourceLexComment
	sourceLexWord
	sourceLexString
	sourceLexOpen
	sourceLexClose
)

type sourceLexToken struct {
	kind   sourceLexKind
	text   string
	start  int
	end    int
	line   int
	column int
}

func sourceLex(text string) []sourceLexToken {
	tokens := make([]sourceLexToken, 0, 128)
	line, column := 1, 1
	advance := func(segment string) {
		for len(segment) > 0 {
			r, size := utf8.DecodeRuneInString(segment)
			segment = segment[size:]
			if r == '\n' {
				line, column = line+1, 1
			} else {
				column++
			}
		}
	}
	appendToken := func(kind sourceLexKind, start, end, tokenLine, tokenColumn int) {
		tokens = append(tokens, sourceLexToken{
			kind: kind, text: text[start:end], start: start, end: end,
			line: tokenLine, column: tokenColumn,
		})
		advance(text[start:end])
	}

	for i := 0; i < len(text); {
		start, tokenLine, tokenColumn := i, line, column
		switch {
		case isSourceSpace(text[i]):
			for i < len(text) && isSourceSpace(text[i]) {
				i++
			}
			appendToken(sourceLexWhitespace, start, i, tokenLine, tokenColumn)
		case text[i] == '/' && i+1 < len(text) && text[i+1] == '/':
			i += 2
			for i < len(text) && text[i] != '\n' {
				i++
			}
			appendToken(sourceLexComment, start, i, tokenLine, tokenColumn)
		case text[i] == '/' && i+1 < len(text) && text[i+1] == '*':
			i += 2
			for i+1 < len(text) && (text[i] != '*' || text[i+1] != '/') {
				i++
			}
			if i+1 < len(text) {
				i += 2
			} else {
				i = len(text)
			}
			appendToken(sourceLexComment, start, i, tokenLine, tokenColumn)
		case text[i] == '"':
			i++
			// SGPV has no escape syntax: the next bare quote closes the token.
			for i < len(text) && text[i] != '"' {
				i++
			}
			if i < len(text) {
				i++
			}
			appendToken(sourceLexString, start, i, tokenLine, tokenColumn)
		case text[i] == '{' || text[i] == '(' || text[i] == '[':
			i++
			appendToken(sourceLexOpen, start, i, tokenLine, tokenColumn)
		case text[i] == '}' || text[i] == ')' || text[i] == ']':
			i++
			appendToken(sourceLexClose, start, i, tokenLine, tokenColumn)
		default:
			for i < len(text) && !isSourceSpace(text[i]) && !strings.ContainsRune("{}()[]\"", rune(text[i])) {
				if text[i] == '/' && i+1 < len(text) && (text[i+1] == '/' || text[i+1] == '*') {
					break
				}
				i++
			}
			if i == start {
				i++
			}
			appendToken(sourceLexWord, start, i, tokenLine, tokenColumn)
		}
	}
	return tokens
}

func isSourceSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

func sourceSyntaxRuns(text string) []SourceSyntaxRun {
	tokens := tokenizeMBII(text)
	runs := make([]SourceSyntaxRun, 0, len(tokens))
	offset := 0
	for _, token := range tokens {
		class := SourceSyntaxText
		switch token.kind {
		case tkComment:
			class = SourceSyntaxComment
		case tkString:
			class = SourceSyntaxString
		case tkNumber:
			class = SourceSyntaxNumber
		case tkConst:
			class = SourceSyntaxConstant
		case tkHeader:
			class = SourceSyntaxHeader
		case tkPunct:
			class = SourceSyntaxPunctuation
		}
		width := len(token.text)
		if offset < len(text) && !strings.HasPrefix(text[offset:], token.text) {
			// tokenizeMBII's defensive byte fallback renders a non-ASCII
			// byte with string(byte), which re-encodes it as UTF-8. The
			// source token still consumed exactly one original byte.
			width = 1
		}
		end := offset + width
		if end > len(text) {
			end = len(text)
		}
		if end > offset {
			if n := len(runs); n > 0 && runs[n-1].Class == class && runs[n-1].Range.End == offset {
				runs[n-1].Range.End = end
			} else {
				runs = append(runs, SourceSyntaxRun{Range: SourceRange{Start: offset, End: end}, Class: class})
			}
		}
		offset = end
	}
	if offset < len(text) {
		runs = append(runs, SourceSyntaxRun{Range: SourceRange{Start: offset, End: len(text)}, Class: SourceSyntaxText})
	}
	return runs
}

func sourceBracketPairs(text string) []SourceBracketPair {
	tokens := sourceLex(text)
	type bracket struct {
		char byte
		at   int
	}
	stack := make([]bracket, 0, 16)
	pairs := make([]SourceBracketPair, 0, 16)
	for _, token := range tokens {
		if token.kind == sourceLexOpen {
			stack = append(stack, bracket{char: text[token.start], at: token.start})
			continue
		}
		if token.kind != sourceLexClose {
			continue
		}
		close := text[token.start]
		for i := len(stack) - 1; i >= 0; i-- {
			if sourceBracketsMatch(stack[i].char, close) {
				pairs = append(pairs, SourceBracketPair{Open: stack[i].at, Close: token.start})
				stack = stack[:i]
				break
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Open < pairs[j].Open })
	return pairs
}

func sourceBracketsMatch(open, close byte) bool {
	return open == '{' && close == '}' || open == '(' && close == ')' || open == '[' && close == ']'
}

func SourceContextAt(text string, caret int, format SourceFormat) SourceContext {
	caret = sourceClampOffset(text, caret)
	tokens := sourceLex(text)
	context := SourceContext{Replace: SourceRange{Start: caret, End: caret}}

	for _, token := range tokens {
		if caret < token.start || caret > token.end {
			continue
		}
		if token.kind == sourceLexComment && caret >= token.start && caret < token.end {
			return context
		}
	}

	context.Block = sourceBlockAt(tokens, caret)
	for i, token := range tokens {
		if !sourceCaretInsideString(text, token, caret) {
			continue
		}
		for previous := i - 1; previous >= 0; previous-- {
			candidate := tokens[previous]
			if candidate.kind == sourceLexWhitespace || candidate.kind == sourceLexComment {
				continue
			}
			if candidate.line == token.line && candidate.kind == sourceLexWord {
				context.Field = sourceTokenValue(candidate)
				context.InString = true
				start, end := sourceValueInterior(token)
				fragmentCaret := caret
				if fragmentCaret > end {
					fragmentCaret = end
				}
				start, end = sourceValueFragment(text, start, end, fragmentCaret, context.Field)
				context.Replace = SourceRange{Start: start, End: end}
				context.Prefix = text[start:fragmentCaret]
				return context
			}
			break
		}
	}
	lineStart := 0
	if i := strings.LastIndex(text[:caret], "\n"); i >= 0 {
		lineStart = i + 1
	}
	lineEnd := len(text)
	if i := strings.Index(text[caret:], "\n"); i >= 0 {
		lineEnd = caret + i
	}

	lineTokens := make([]sourceLexToken, 0, 4)
	for _, token := range tokens {
		if token.end <= lineStart || token.start >= lineEnd || token.kind == sourceLexWhitespace || token.kind == sourceLexComment {
			continue
		}
		lineTokens = append(lineTokens, token)
	}
	if len(lineTokens) == 0 {
		context.AtKey = true
		return context
	}
	if len(lineTokens) > 1 && lineTokens[1].kind == sourceLexOpen {
		return context // A block header is neither a key nor a value.
	}

	first := lineTokens[0]
	if caret <= first.end {
		context.AtKey = true
		context.Prefix = sourceTokenInterior(text, first, caret)
		context.Replace = sourceTokenReplace(first)
		return context
	}

	context.Field = sourceTokenValue(first)
	context.AtKey = false
	var value *sourceLexToken
	for i := 1; i < len(lineTokens); i++ {
		token := &lineTokens[i]
		if token.kind == sourceLexOpen || token.kind == sourceLexClose {
			continue
		}
		if caret >= token.start && caret <= token.end {
			value = token
			break
		}
		if token.start <= caret {
			value = token
		}
	}
	if value == nil {
		return context
	}
	if value.kind == sourceLexString && caret == value.end && strings.HasSuffix(value.text, "\"") {
		start, end := sourceValueInterior(*value)
		context.InString = true
		context.Replace = SourceRange{Start: start, End: end}
		context.Prefix = text[start:end]
		return context
	}
	if caret > value.end {
		context.Field = ""
		return context
	}
	// Any replacement inside an existing quoted token, including a caret just
	// before its opening quote, must not add another pair of quotes.
	context.InString = value.kind == sourceLexString && strings.HasPrefix(value.text, "\"")
	start, end := sourceValueInterior(*value)
	if caret < start {
		caret = start
	}
	if caret > end {
		caret = end
	}
	start, end = sourceValueFragment(text, start, end, caret, context.Field)
	context.Replace = SourceRange{Start: start, End: end}
	context.Prefix = text[start:caret]
	return context
}

func sourceCaretInsideString(text string, token sourceLexToken, caret int) bool {
	if token.kind != sourceLexString || caret <= token.start {
		return false
	}
	closed := len(token.text) > 1 && strings.HasSuffix(token.text, "\"")
	if closed {
		return caret < token.end
	}
	return caret <= token.end && token.end == len(text)
}

func sourceBlockAt(tokens []sourceLexToken, caret int) string {
	type block struct {
		name  string
		close byte
	}
	stack := make([]block, 0, 8)
	previous := ""
	for _, token := range tokens {
		if token.start >= caret {
			break
		}
		switch token.kind {
		case sourceLexWhitespace, sourceLexComment:
			continue
		case sourceLexWord, sourceLexString:
			previous = sourceTokenValue(token)
		case sourceLexOpen:
			open := token.text[0]
			if open == '{' {
				stack = append(stack, block{name: previous, close: '}'})
			} else if open == '(' {
				stack = append(stack, block{close: ')'})
			} else {
				stack = append(stack, block{close: ']'})
			}
			previous = ""
		case sourceLexClose:
			if len(stack) > 0 && token.text[0] == stack[len(stack)-1].close {
				stack = stack[:len(stack)-1]
			}
			previous = ""
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].name != "" {
			return stack[i].name
		}
	}
	return ""
}

func sourceTokenValue(token sourceLexToken) string {
	if token.kind == sourceLexString && len(token.text) > 0 {
		value := strings.TrimPrefix(token.text, "\"")
		return strings.TrimSuffix(value, "\"")
	}
	return token.text
}

func sourceTokenInterior(text string, token sourceLexToken, caret int) string {
	start, end := sourceValueInterior(token)
	if caret < start {
		return ""
	}
	if caret > end {
		caret = end
	}
	return text[start:caret]
}

func sourceTokenReplace(token sourceLexToken) SourceRange {
	start, end := sourceValueInterior(token)
	return SourceRange{Start: start, End: end}
}

func sourceValueInterior(token sourceLexToken) (int, int) {
	start, end := token.start, token.end
	if token.kind == sourceLexString {
		start++
		if len(token.text) > 1 && strings.HasSuffix(token.text, "\"") {
			end--
		}
	}
	if end < start {
		end = start
	}
	return start, end
}

func sourceValueFragment(text string, start, end, caret int, field string) (int, int) {
	left, right := start, end
	if i := strings.LastIndex(text[start:caret], "|"); i >= 0 {
		left = start + i + 1
	}
	if sourceRankedField(field) {
		if i := strings.LastIndex(text[left:caret], ","); i >= 0 {
			left += i + 1
		}
	}
	for right > caret {
		segment := text[caret:right]
		i := strings.IndexAny(segment, "|")
		if sourceRankedField(field) {
			if comma := strings.Index(segment, ","); comma >= 0 && (i < 0 || comma < i) {
				i = comma
			}
		}
		if i < 0 {
			break
		}
		right = caret + i
		break
	}
	return left, right
}

func sourceRankedField(field string) bool {
	switch strings.ToLower(field) {
	case "attributes", "forcepowers":
		return true
	}
	return false
}

func sourceClampOffset(text string, offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	for offset > 0 && offset < len(text) && !utf8.RuneStart(text[offset]) {
		offset--
	}
	return offset
}

func sourceLineColumn(text string, offset int) (int, int) {
	offset = sourceClampOffset(text, offset)
	line, column := 1, 1
	for len(text[:offset]) > 0 {
		r, size := utf8.DecodeRuneInString(text[:offset])
		text = text[size:]
		offset -= size
		if r == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return line, column
}

func sourceFormatSupported(format SourceFormat) bool {
	switch format {
	case SourceFormatMBCH, SourceFormatSAB, SourceFormatVEH, SourceFormatSiege, SourceFormatMBTC:
		return true
	}
	return false
}
