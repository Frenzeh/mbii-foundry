package main

import (
	"sort"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type sourceEditDelta struct {
	Start  int
	Before string
	After  string
}

type sourceEditTransaction struct {
	Deltas       []sourceEditDelta
	CaretBefore  int
	AnchorBefore int
	CaretAfter   int
	AnchorAfter  int
}

// sourceCodeBuffer keeps document history as replacement deltas. Text itself is
// copied only when Go must materialize a changed immutable string.
type sourceCodeBuffer struct {
	text   string
	caret  int
	anchor int
	undo   []sourceEditTransaction
	redo   []sourceEditTransaction
}

func (b *sourceCodeBuffer) reset(text string) bool {
	if b.text == text {
		return false
	}
	b.text = text
	b.caret = clampSourceOffset(text, b.caret)
	b.anchor = b.caret
	b.undo = nil
	b.redo = nil
	return true
}

func (b *sourceCodeBuffer) selection() (int, int) {
	if b.anchor < b.caret {
		return b.anchor, b.caret
	}
	return b.caret, b.anchor
}

func (b *sourceCodeBuffer) selectRange(start, end int) {
	b.anchor = clampSourceOffset(b.text, start)
	b.caret = clampSourceOffset(b.text, end)
}

func (b *sourceCodeBuffer) replace(start, end int, replacement string) bool {
	start, end = normalizedSourceRange(b.text, start, end)
	if b.text[start:end] == replacement {
		b.anchor = start + len(replacement)
		b.caret = b.anchor
		return false
	}
	tx := sourceEditTransaction{
		Deltas:       []sourceEditDelta{{Start: start, Before: b.text[start:end], After: replacement}},
		CaretBefore:  b.caret,
		AnchorBefore: b.anchor,
		CaretAfter:   start + len(replacement),
		AnchorAfter:  start + len(replacement),
	}
	return b.applyNew(tx)
}

func (b *sourceCodeBuffer) applyNew(tx sourceEditTransaction) bool {
	if len(tx.Deltas) == 0 {
		return false
	}
	for _, delta := range tx.Deltas {
		b.text = applySourceDelta(b.text, delta.Start, delta.Before, delta.After)
	}
	b.caret = clampSourceOffset(b.text, tx.CaretAfter)
	b.anchor = clampSourceOffset(b.text, tx.AnchorAfter)
	b.undo = append(b.undo, tx)
	b.redo = nil
	return true
}

func (b *sourceCodeBuffer) undoEdit() bool {
	if len(b.undo) == 0 {
		return false
	}
	last := len(b.undo) - 1
	tx := b.undo[last]
	b.undo = b.undo[:last]
	for i := len(tx.Deltas) - 1; i >= 0; i-- {
		delta := tx.Deltas[i]
		b.text = applySourceDelta(b.text, delta.Start, delta.After, delta.Before)
	}
	b.caret = clampSourceOffset(b.text, tx.CaretBefore)
	b.anchor = clampSourceOffset(b.text, tx.AnchorBefore)
	b.redo = append(b.redo, tx)
	return true
}

func (b *sourceCodeBuffer) redoEdit() bool {
	if len(b.redo) == 0 {
		return false
	}
	last := len(b.redo) - 1
	tx := b.redo[last]
	b.redo = b.redo[:last]
	for _, delta := range tx.Deltas {
		b.text = applySourceDelta(b.text, delta.Start, delta.Before, delta.After)
	}
	b.caret = clampSourceOffset(b.text, tx.CaretAfter)
	b.anchor = clampSourceOffset(b.text, tx.AnchorAfter)
	b.undo = append(b.undo, tx)
	return true
}

func applySourceDelta(text string, start int, before, after string) string {
	start = clampSourceOffset(text, start)
	end := start + len(before)
	if end > len(text) || text[start:end] != before {
		return text
	}
	return text[:start] + after + text[end:]
}

func normalizedSourceRange(text string, start, end int) (int, int) {
	start = clampSourceOffset(text, start)
	end = clampSourceOffset(text, end)
	if start > end {
		start, end = end, start
	}
	return start, end
}

func clampSourceOffset(text string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(text) {
		return len(text)
	}
	for offset > 0 && !utf8.RuneStart(text[offset]) {
		offset--
	}
	if offset > 0 && offset < len(text) && text[offset-1] == '\r' && text[offset] == '\n' {
		return offset - 1
	}
	return offset
}

func previousSourceOffset(text string, offset int) int {
	offset = clampSourceOffset(text, offset)
	if offset == 0 {
		return 0
	}
	if offset >= 2 && text[offset-2:offset] == "\r\n" {
		return offset - 2
	}
	_, size := utf8.DecodeLastRuneInString(text[:offset])
	return offset - size
}

func nextSourceOffset(text string, offset int) int {
	offset = clampSourceOffset(text, offset)
	if offset >= len(text) {
		return len(text)
	}
	if offset+1 < len(text) && text[offset:offset+2] == "\r\n" {
		return offset + 2
	}
	_, size := utf8.DecodeRuneInString(text[offset:])
	return offset + size
}

type sourceCell struct {
	Rune   rune
	Start  int
	End    int
	Column int
	Width  int
}

type sourceLine struct {
	Start      int
	ContentEnd int
	End        int
	Cells      []sourceCell
}

func sourceTextLines(text string) []sourceLine {
	lines := make([]sourceLine, 0, 1)
	lineStart := 0
	for lineStart <= len(text) {
		contentEnd := lineStart
		for contentEnd < len(text) && text[contentEnd] != '\n' && text[contentEnd] != '\r' {
			_, size := utf8.DecodeRuneInString(text[contentEnd:])
			contentEnd += size
		}
		end := contentEnd
		hasLineEnding := end < len(text)
		if hasLineEnding {
			if text[end] == '\r' && end+1 < len(text) && text[end+1] == '\n' {
				end += 2
			} else {
				end++
			}
		}
		line := sourceLine{Start: lineStart, ContentEnd: contentEnd, End: end}
		column := 0
		for offset := lineStart; offset < contentEnd; {
			r, size := utf8.DecodeRuneInString(text[offset:contentEnd])
			width := runewidth.RuneWidth(r)
			if r == '\t' {
				width = 4 - column%4
			} else if width < 1 {
				width = 1
			}
			line.Cells = append(line.Cells, sourceCell{Rune: r, Start: offset, End: offset + size, Column: column, Width: width})
			column += width
			offset += size
		}
		lines = append(lines, line)
		if !hasLineEnding {
			break
		}
		lineStart = end
	}
	return lines
}

func sourceLineForOffset(lines []sourceLine, offset int) int {
	if len(lines) == 0 {
		return 0
	}
	for i, line := range lines {
		if offset < line.End || i == len(lines)-1 {
			return i
		}
	}
	return len(lines) - 1
}

func sourceOffsetForColumn(line sourceLine, column int) int {
	if column <= 0 || len(line.Cells) == 0 {
		return line.Start
	}
	for _, cell := range line.Cells {
		if column < cell.Column+cell.Width {
			if column-cell.Column >= (cell.Width+1)/2 {
				return cell.End
			}
			return cell.Start
		}
	}
	return line.ContentEnd
}

func sourceColumnForOffset(line sourceLine, offset int) int {
	for _, cell := range line.Cells {
		if offset <= cell.Start {
			return cell.Column
		}
		if offset < cell.End {
			return cell.Column + cell.Width
		}
	}
	if len(line.Cells) == 0 {
		return 0
	}
	last := line.Cells[len(line.Cells)-1]
	return last.Column + last.Width
}

func sortedSourceDeltas(deltas []sourceEditDelta) {
	sort.SliceStable(deltas, func(i, j int) bool { return deltas[i].Start > deltas[j].Start })
}
