package parsers

import (
	"fmt"
	"strings"
)

type BlockAssessmentResult struct {
	TotalFileBytes      int
	ClassInfoBytes      int
	WeaponInfoBytes     []int
	ForceInfoBytes      []int
	MaxPairedValueBytes int
	Diagnostics         []string
}

// AssessMBCHSourceBuffers measures a .mbch source the way the engine
// consumes it.
//
// Budget accounting (BG_SiegeGetValueGroup, bg_saga.c:1273-1316): the
// group interior is copied RAW — every character between the outer
// braces, comments and whitespace INCLUDED — before the destsize check
// (`if (j == destsize)` rejects the file). So payload sizes here count
// raw bytes, comments and all, excluding only the outermost braces.
//
// Paired-value accounting (SGPV, bg_saga.c:216-300): for each line the
// first token is the key; its value is the next token (quoted values may
// span lines); the value INTERIOR is what SGPV writes into the
// SIEGE_PARSE_BUF_LEN buffer (quotes excluded, `//` terminates a value
// even inside quotes); everything after the pair on a line is dead text
// and never misread as a key.
func AssessMBCHSourceBuffers(source string) (BlockAssessmentResult, error) {
	tokens, err := Lex(source)
	doc := parseAST(tokens)

	res := BlockAssessmentResult{
		TotalFileBytes: len(source),
	}
	// Engine-faithful lexical diagnostics (e.g. unterminated quote →
	// "unexpected EOF while looking for endquote"). The raw draft is
	// preserved; these findings block validation, not lexing.
	res.Diagnostics = append(res.Diagnostics, LexDiagnostics(source)...)
	var openBraces int
	for _, tok := range tokens {
		if tok.Type == TokenBraceOpen {
			openBraces++
		}
		if tok.Type == TokenBraceClose {
			openBraces--
		}
		if tok.Type == TokenString {
			text := tok.Text
			if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
				// SGPV has no escape sequences: a backslash-quote ends
				// the value early and the trailing quote leaks.
				if strings.Contains(text, "\\\"") {
					res.Diagnostics = append(res.Diagnostics, "escaped quotes under SGPV")
				}
				// SGPV breaks a value (even a quoted one) at `//`.
				if strings.Contains(text, "//") {
					res.Diagnostics = append(res.Diagnostics, "unsupported quoted //")
				}
			}
		}
	}
	if openBraces != 0 {
		res.Diagnostics = append(res.Diagnostics, "unbalanced braces")
	}

	// Find effective blocks and calculate payloads. The engine reads the
	// FIRST group with a given name (BG_SiegeGetValueGroup scans from
	// the top and returns on first match).
	seenBlocks := make(map[string]bool)

	for _, node := range doc.Nodes {
		b, ok := node.(*ASTBlock)
		if !ok || b.NameToken == nil {
			continue
		}
		name := strings.ToLower(b.NameToken.Text)

		if seenBlocks[name] {
			continue // engine uses first-effective group
		}
		seenBlocks[name] = true

		payloadSize := rawInteriorLength(b.Children)

		switch {
		case name == "classinfo":
			res.ClassInfoBytes = payloadSize
		case strings.HasPrefix(name, "weaponinfo"):
			res.WeaponInfoBytes = append(res.WeaponInfoBytes, payloadSize)
		case strings.HasPrefix(name, "forceinfo"):
			res.ForceInfoBytes = append(res.ForceInfoBytes, payloadSize)
		}

		// Paired-value scan, SGPV-accurate.
		walkSGPVPairs(b.Children, func(_ int, key string, _ int, val string, hasVal bool) bool {
			if len(key) >= KeyMaxPayload {
				// SGPV copies keys into checkKey[SIEGE_CLASS_KEY_LEN]
				// unbounded (bg_saga.c:236-244) — a too-long key is an
				// engine overflow hazard.
				res.Diagnostics = append(res.Diagnostics, fmt.Sprintf("key exceeds %d bytes: %s", KeyMaxPayload, key))
			}
			if !hasVal {
				res.Diagnostics = append(res.Diagnostics, fmt.Sprintf("bare recognized value or empty key: %s", key))
				return true
			}
			// Exact SGPV buffer accounting: the value interior (quotes
			// excluded) is what lands in pB.
			if len(val) > res.MaxPairedValueBytes {
				res.MaxPairedValueBytes = len(val)
			}
			if val == "" {
				res.Diagnostics = append(res.Diagnostics, fmt.Sprintf("bare recognized value or empty key: %s", key))
			}
			return true
		})
	}

	if err != nil {
		res.Diagnostics = append(res.Diagnostics, err.Error())
	}

	return res, nil
}

// rawInteriorLength counts the bytes BG_SiegeGetValueGroup copies from a
// group: all interior text except the group's own outer braces. Nested
// groups contribute their name, their inner braces and their interior
// (the copy keeps inner `{`/`}` when parseGroups > 0, bg_saga.c:1289-
// 1301). Comments and whitespace are raw text to the engine and count.
func rawInteriorLength(nodes []ASTNode) int {
	total := 0
	var walk func(nodes []ASTNode, outer bool)
	walk = func(nodes []ASTNode, outer bool) {
		for _, n := range nodes {
			switch t := n.(type) {
			case *ASTToken:
				total += len(t.Text)
			case *ASTBlock:
				if t.NameToken != nil {
					total += len(t.NameToken.Text)
				}
				if !outer {
					total += 2 // inner braces survive the copy
				}
				walk(t.Children, false)
			}
		}
	}
	walk(nodes, true)
	return total
}
