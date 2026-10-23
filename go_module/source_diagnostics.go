package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Frenzeh/mbii-foundry/parsers"
)

func sourceDiagnostics(text string, format SourceFormat) []SourceDiagnostic {
	diagnostics := sourceStructuralDiagnostics(text)
	if !sourceFormatSupported(format) {
		return append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError,
			fmt.Sprintf("unsupported source format %q", format)))
	}
	if strings.TrimSpace(text) == "" {
		return append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError, "source is empty"))
	}

	blocks := sourceBlocks(text)
	if len(sourceTopBlocks(blocks)) == 0 {
		diagnostics = append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError,
			"source contains no definition block"))
	}

	switch format {
	case SourceFormatMBCH:
		diagnostics = append(diagnostics, sourceMBCHDiagnostics(text, blocks)...)
	case SourceFormatSAB:
		diagnostics = append(diagnostics, sourceSABDiagnostics(text, blocks)...)
	case SourceFormatVEH:
		diagnostics = append(diagnostics, sourceVEHDiagnostics(text, blocks)...)
	case SourceFormatSiege:
		diagnostics = append(diagnostics, sourceSiegeDiagnostics(text)...)
	case SourceFormatMBTC:
		diagnostics = append(diagnostics, sourceMBTCDiagnostics(text)...)
	}
	diagnostics = append(diagnostics, sourceSemanticWarnings(text, format)...)
	return sourceUniqueDiagnostics(diagnostics)
}

func sourceStructuralDiagnostics(text string) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	stack := make([]int, 0, 16)
	for i := 0; i < len(text); {
		switch {
		case text[i] == '/' && i+1 < len(text) && text[i+1] == '/':
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case text[i] == '"':
			start := i
			i++
			for i < len(text) && text[i] != '"' {
				i++
			}
			if i == len(text) {
				diagnostics = append(diagnostics, sourceDiagnostic(text,
					SourceRange{Start: start, End: len(text)}, SourceError,
					"unexpected EOF while looking for endquote"))
				continue
			}
			i++
		case text[i] == '{':
			stack = append(stack, i)
			i++
		case text[i] == '}':
			if len(stack) == 0 {
				diagnostics = append(diagnostics, sourceDiagnostic(text,
					SourceRange{Start: i, End: i + 1}, SourceError, "unexpected closing brace"))
			} else {
				stack = stack[:len(stack)-1]
			}
			i++
		default:
			i++
		}
	}
	for _, start := range stack {
		diagnostics = append(diagnostics, sourceDiagnostic(text,
			SourceRange{Start: start, End: start + 1}, SourceError, "unclosed opening brace"))
	}
	return diagnostics
}

func sourceMBCHDiagnostics(text string, blocks []sourceBlock) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	classInfo := sourceFindBlock(blocks, "classinfo")
	if classInfo == nil {
		diagnostics = append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError,
			"missing ClassInfo definition block"))
	}

	character, parseErr := parsers.ParseMBCH(text)
	if parseErr != nil {
		diagnostics = append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError,
			"MBCH parser: "+parseErr.Error()))
	} else if classInfo != nil {
		if character.Name == "" {
			diagnostics = append(diagnostics, sourceDiagnostic(text, classInfo.nameRange, SourceError,
				"ClassInfo is missing required name"))
		}
		if character.MBClass == "" || strings.EqualFold(character.MBClass, "MB_CLASS_NOCLASS") {
			diagnostics = append(diagnostics, sourceDiagnostic(text, classInfo.nameRange, SourceError,
				"ClassInfo is missing a selectable MBClass"))
		}
		for _, finding := range character.Diags {
			rangeHint := sourceRangeForMention(text, finding)
			diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceWarning, finding))
		}
	}

	assessment, assessErr := parsers.AssessMBCHSourceBuffers(text)
	if assessErr != nil {
		diagnostics = append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError,
			"MBCH capacity assessment failed: "+assessErr.Error()))
		return diagnostics
	}
	for _, finding := range assessment.Diagnostics {
		lower := strings.ToLower(finding)
		if strings.Contains(lower, "unbalanced braces") || strings.Contains(lower, "unterminated quote") {
			continue
		}
		rangeHint := sourceRangeForMention(text, finding)
		if strings.Contains(lower, "escaped quotes") {
			if at := strings.Index(text, `\"`); at >= 0 {
				rangeHint = SourceRange{Start: at, End: at + 2}
			}
		}
		if strings.Contains(lower, "quoted //") {
			if at := strings.Index(text, "//"); at >= 0 {
				rangeHint = SourceRange{Start: at, End: at + 2}
			}
		}
		diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceError, finding))
	}
	if assessment.TotalFileBytes >= parsers.MBCHMaxFileBytes {
		diagnostics = append(diagnostics, sourceDiagnostic(text, sourceEOFRange(text), SourceError,
			fmt.Sprintf("file exceeds engine payload limit (%d/%d bytes)", assessment.TotalFileBytes, parsers.MBCHMaxFileBytes-1)))
	} else if assessment.TotalFileBytes > parsers.MBCHMaxFileBytes-500 {
		diagnostics = append(diagnostics, sourceDiagnostic(text, sourceEOFRange(text), SourceWarning,
			fmt.Sprintf("file is near the engine payload limit (%d/%d bytes)", assessment.TotalFileBytes, parsers.MBCHMaxFileBytes-1)))
	}
	if assessment.ClassInfoBytes > parsers.ClassInfoMaxPayload {
		diagnostics = append(diagnostics, sourceDiagnostic(text, sourceBlockRange(classInfo), SourceError,
			fmt.Sprintf("ClassInfo exceeds engine payload limit (%d/%d bytes)", assessment.ClassInfoBytes, parsers.ClassInfoMaxPayload)))
	}
	diagnostics = append(diagnostics, sourceBlockCapacityDiagnostics(text, blocks, "weaponinfo", assessment.WeaponInfoBytes, parsers.WeaponInfoMaxPayload)...)
	diagnostics = append(diagnostics, sourceBlockCapacityDiagnostics(text, blocks, "forceinfo", assessment.ForceInfoBytes, parsers.ForceInfoMaxPayload)...)
	if assessment.MaxPairedValueBytes > parsers.PairedValueMaxPayload {
		diagnostics = append(diagnostics, sourceDiagnostic(text, sourceLongestValueRange(text), SourceError,
			fmt.Sprintf("paired value exceeds engine payload limit (%d/%d bytes)", assessment.MaxPairedValueBytes, parsers.PairedValueMaxPayload)))
	}
	return diagnostics
}

func sourceSABDiagnostics(text string, blocks []sourceBlock) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	topBlocks := sourceTopBlocks(blocks)
	for index, block := range topBlocks {
		saber, err := parsers.ParseSABDefinition(text, index)
		if err != nil {
			diagnostics = append(diagnostics, sourceDiagnostic(text, block.nameRange, SourceError,
				fmt.Sprintf("saber definition %d: %v", index+1, err)))
			continue
		}
		if strings.TrimSpace(saber.Name) == "" {
			diagnostics = append(diagnostics, sourceDiagnostic(text, block.nameRange, SourceError,
				"saber definition has no name"))
		}
		if semanticErr := parsers.ValidateSABSemantics(saber); semanticErr != nil {
			rangeHint := block.nameRange
			definitionEnd := len(text)
			if index+1 < len(topBlocks) {
				definitionEnd = topBlocks[index+1].nameRange.Start
			}
			if local := sourceRangeForMention(text[block.nameRange.Start:definitionEnd], semanticErr.Error()); local.End > local.Start {
				rangeHint = SourceRange{Start: block.nameRange.Start + local.Start, End: block.nameRange.Start + local.End}
			}
			diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceError,
				fmt.Sprintf("saber definition %d: %v", index+1, semanticErr)))
		}
	}
	return diagnostics
}

func sourceVEHDiagnostics(text string, blocks []sourceBlock) []SourceDiagnostic {
	var diagnostics []SourceDiagnostic
	for index, block := range sourceTopBlocks(blocks) {
		vehicle, err := parsers.ParseVEHDefinition(text, index)
		if err != nil {
			diagnostics = append(diagnostics, sourceDiagnostic(text, block.nameRange, SourceError,
				fmt.Sprintf("vehicle definition %d: %v", index+1, err)))
			continue
		}
		if strings.TrimSpace(vehicle.Name) == "" {
			diagnostics = append(diagnostics, sourceDiagnostic(text, block.nameRange, SourceError,
				"vehicle definition has no name"))
		}
		if strings.TrimSpace(vehicle.Type) == "" {
			diagnostics = append(diagnostics, sourceDiagnostic(text, block.nameRange, SourceError,
				"vehicle definition is missing type"))
		}
	}
	return diagnostics
}

func sourceSiegeDiagnostics(text string) []SourceDiagnostic {
	siege, err := parsers.ParseSiege(text)
	if err != nil {
		return []SourceDiagnostic{sourceDiagnostic(text, SourceRange{}, SourceError, "siege parser: "+err.Error())}
	}
	var diagnostics []SourceDiagnostic
	if siege.Team1 == nil && siege.Team2 == nil {
		diagnostics = append(diagnostics, sourceDiagnostic(text, SourceRange{}, SourceError,
			"siege requires at least one team"))
	}
	for _, team := range []*parsers.SiegeTeam{siege.Team1, siege.Team2} {
		if team == nil {
			continue
		}
		rangeHint := sourceFindInsensitiveRange(text, team.Name)
		if strings.TrimSpace(team.UseTeam) == "" {
			diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceError,
				fmt.Sprintf("team %q is missing useTeam", team.Name)))
		}
		if team.Attackers != 0 && len(team.Objectives) > 0 {
			hasFinal := false
			for _, objective := range team.Objectives {
				hasFinal = hasFinal || objective.Final != 0
			}
			if !hasFinal {
				diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceWarning,
					fmt.Sprintf("attacking team %q has objectives but no final objective", team.Name)))
			}
		}
	}
	return diagnostics
}

func sourceMBTCDiagnostics(text string) []SourceDiagnostic {
	team := parsers.ParseMBTC(text)
	var diagnostics []SourceDiagnostic
	for _, finding := range team.Diagnostics {
		severity := SourceWarning
		if strings.Contains(strings.ToLower(finding), "engine errors out") {
			severity = SourceError
		}
		diagnostics = append(diagnostics, sourceDiagnostic(text, sourceRangeForMention(text, finding), severity, finding))
	}
	if len(text) >= parsers.MBTCMaxFileBytes {
		diagnostics = append(diagnostics, sourceDiagnostic(text, sourceEOFRange(text), SourceError,
			fmt.Sprintf("team file exceeds engine payload limit (%d/%d bytes)", len(text), parsers.MBTCMaxFileBytes-1)))
	}
	return diagnostics
}

func sourceDiagnostic(text string, rangeHint SourceRange, severity SourceSeverity, message string) SourceDiagnostic {
	rangeHint.Start = sourceClampOffset(text, rangeHint.Start)
	rangeHint.End = sourceClampOffset(text, rangeHint.End)
	if rangeHint.End < rangeHint.Start {
		rangeHint.End = rangeHint.Start
	}
	line, column := sourceLineColumn(text, rangeHint.Start)
	return SourceDiagnostic{Range: rangeHint, Line: line, Column: column, Severity: severity, Message: message}
}

func sourceEOFRange(text string) SourceRange {
	if len(text) == 0 {
		return SourceRange{}
	}
	start := sourceClampOffset(text, len(text)-1)
	return SourceRange{Start: start, End: len(text)}
}

func sourceUniqueDiagnostics(in []SourceDiagnostic) []SourceDiagnostic {
	seen := make(map[string]bool, len(in))
	out := make([]SourceDiagnostic, 0, len(in))
	for _, diagnostic := range in {
		key := fmt.Sprintf("%s:%d:%d:%s", diagnostic.Severity, diagnostic.Range.Start, diagnostic.Range.End, diagnostic.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, diagnostic)
	}
	return out
}

type sourceBlock struct {
	name      string
	nameRange SourceRange
	depth     int
}

func sourceBlocks(text string) []sourceBlock {
	tokens := sourceLex(text)
	blocks := make([]sourceBlock, 0, 16)
	stack := make([]int, 0, 16)
	previous := -1
	for i, token := range tokens {
		switch token.kind {
		case sourceLexWhitespace, sourceLexComment:
			continue
		case sourceLexWord, sourceLexString:
			previous = i
		case sourceLexOpen:
			if token.text != "{" {
				continue
			}
			block := sourceBlock{depth: len(stack)}
			if previous >= 0 {
				block.name = sourceTokenValue(tokens[previous])
				block.nameRange = SourceRange{Start: tokens[previous].start, End: tokens[previous].end}
			}
			blocks = append(blocks, block)
			stack = append(stack, len(blocks)-1)
			previous = -1
		case sourceLexClose:
			if token.text == "}" && len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			previous = -1
		}
	}
	return blocks
}

func sourceTopBlocks(blocks []sourceBlock) []sourceBlock {
	out := make([]sourceBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.depth == 0 {
			out = append(out, block)
		}
	}
	return out
}

func sourceFindBlock(blocks []sourceBlock, name string) *sourceBlock {
	for i := range blocks {
		if strings.EqualFold(blocks[i].name, name) {
			return &blocks[i]
		}
	}
	return nil
}

func sourceBlockRange(block *sourceBlock) SourceRange {
	if block == nil {
		return SourceRange{}
	}
	return block.nameRange
}

func sourceBlockCapacityDiagnostics(text string, blocks []sourceBlock, prefix string, sizes []int, limit int) []SourceDiagnostic {
	matching := make([]sourceBlock, 0, len(sizes))
	for _, block := range blocks {
		if block.depth == 0 && strings.HasPrefix(strings.ToLower(block.name), prefix) {
			matching = append(matching, block)
		}
	}
	var diagnostics []SourceDiagnostic
	for i, size := range sizes {
		if size <= limit {
			continue
		}
		rangeHint := SourceRange{}
		if i < len(matching) {
			rangeHint = matching[i].nameRange
		}
		diagnostics = append(diagnostics, sourceDiagnostic(text, rangeHint, SourceError,
			fmt.Sprintf("%s block exceeds engine payload limit (%d/%d bytes)", prefix, size, limit)))
	}
	return diagnostics
}

func sourceRangeForMention(text, message string) SourceRange {
	fields := strings.FieldsFunc(message, func(r rune) bool {
		return !(r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z')
	})
	for _, field := range fields {
		if len(field) < 3 {
			continue
		}
		if rangeHint := sourceFindInsensitiveRange(text, field); rangeHint.End > rangeHint.Start {
			return rangeHint
		}
	}
	return SourceRange{}
}

func sourceFindInsensitiveRange(text, needle string) SourceRange {
	if needle == "" {
		return SourceRange{}
	}
	foldedText, originalOffsets := sourceFoldedByteOffsets(text)
	foldedNeedle := strings.ToLower(needle)
	at := strings.Index(foldedText, foldedNeedle)
	if at < 0 {
		return SourceRange{}
	}
	end := at + len(foldedNeedle)
	if at >= len(originalOffsets) || end >= len(originalOffsets) {
		return SourceRange{}
	}
	return SourceRange{Start: originalOffsets[at], End: originalOffsets[end]}
}

func sourceFoldedByteOffsets(text string) (string, []int) {
	var folded strings.Builder
	offsets := make([]int, 1, len(text)+1)
	for start := 0; start < len(text); {
		_, size := utf8.DecodeRuneInString(text[start:])
		lower := strings.ToLower(text[start : start+size])
		folded.WriteString(lower)
		for range len(lower) {
			offsets = append(offsets, start)
		}
		start += size
		offsets[len(offsets)-1] = start
	}
	return folded.String(), offsets
}

func sourceLongestValueRange(text string) SourceRange {
	longest := SourceRange{}
	for _, occurrence := range sourceFieldOccurrences(text, SourceFormatMBCH) {
		if occurrence.valueRange.End-occurrence.valueRange.Start > longest.End-longest.Start {
			longest = occurrence.valueRange
		}
	}
	return longest
}

func sourceParseRank(raw string) (string, *int) {
	parts := strings.SplitN(strings.TrimSpace(raw), ",", 2)
	if len(parts) != 2 {
		return parts[0], nil
	}
	rank, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return parts[0], nil
	}
	return parts[0], &rank
}
