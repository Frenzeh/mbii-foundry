package main

import "image/color"

// SourceRange uses zero-based UTF-8 byte offsets; End is exclusive.
type SourceRange struct {
	Start int
	End   int
}

type SourceFormat string

const (
	SourceFormatMBCH  SourceFormat = ".mbch"
	SourceFormatSAB   SourceFormat = ".sab"
	SourceFormatVEH   SourceFormat = ".veh"
	SourceFormatSiege SourceFormat = ".siege"
	SourceFormatMBTC  SourceFormat = ".mbtc"
)

type SourceSyntaxClass uint8

const (
	SourceSyntaxText SourceSyntaxClass = iota
	SourceSyntaxComment
	SourceSyntaxString
	SourceSyntaxNumber
	SourceSyntaxConstant
	SourceSyntaxHeader
	SourceSyntaxPunctuation
	sourceSyntaxClassCount
)

type SourceSyntaxRun struct {
	Range SourceRange
	Class SourceSyntaxClass
}

type SourceSeverity string

const (
	SourceError   SourceSeverity = "error"
	SourceWarning SourceSeverity = "warning"
)

type SourceDiagnostic struct {
	Range    SourceRange
	Line     int // One-based line.
	Column   int // One-based Unicode rune column.
	Severity SourceSeverity
	Message  string
}

type SourceBracketPair struct {
	Open  int
	Close int
}

type SourceAnalysis struct {
	Syntax      []SourceSyntaxRun
	Diagnostics []SourceDiagnostic
	Brackets    []SourceBracketPair
}

type SourceCompletion struct {
	Label      string
	InsertText string
	Detail     string
	Replace    SourceRange
}

type SourceContext struct {
	Block    string
	Field    string
	Prefix   string
	Replace  SourceRange
	InString bool
	AtKey    bool
}

// SourcePalette is independent of the surrounding application theme.
type SourcePalette struct {
	Name            string
	Colors          [sourceSyntaxClassCount]color.NRGBA
	Background      color.NRGBA
	Selection       color.NRGBA
	Caret           color.NRGBA
	LineNumber      color.NRGBA
	Error           color.NRGBA
	Warning         color.NRGBA
	MatchingBracket color.NRGBA
}
