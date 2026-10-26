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

	assessPairs := func(nodes []ASTNode) {
		walkSGPVPairs(nodes, func(keyIdx int, key string, _ int, val string, hasVal bool) bool {
			if !hasVal {
				// The retained AST can attach a scalar value to a following
				// anonymous group as its NameToken: key value\n{...}.
				// That name is still the next lexical value before the brace.
				// Recover it for read-only assessment; mutable AST visitors
				// cannot use a block index as a scalar value-token index.
				for _, node := range nodes[keyIdx+1:] {
					if block, ok := node.(*ASTBlock); ok {
						if block.NameToken != nil {
							val = truncateSGPV(unquote(block.NameToken.Text))
							hasVal = true
						}
						break
					}
				}
			}
			if len(key) >= KeyMaxPayload {
				res.Diagnostics = append(res.Diagnostics, fmt.Sprintf("key exceeds %d bytes: %s", KeyMaxPayload, key))
			}
			if !hasVal {
				res.Diagnostics = append(res.Diagnostics, fmt.Sprintf("bare recognized value or empty key: %s", key))
				return true
			}
			// SGPV copies the value interior, without its quotes.
			if len(val) > res.MaxPairedValueBytes {
				res.MaxPairedValueBytes = len(val)
			}
			if val == "" {
				res.Diagnostics = append(res.Diagnostics, fmt.Sprintf("bare recognized value or empty key: %s", key))
			}
			return true
		})
	}
	// The engine also reads paired fields, notably description, directly
	// from the file buffer. Top-level groups are skipped by this scan.
	assessPairs(doc.Nodes)

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

		assessPairs(b.Children)
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
	for _, node := range nodes {
		switch n := node.(type) {
		case *ASTToken:
			total += len(n.Text)
		case *ASTBlock:
			if n.NameToken != nil {
				total += len(n.NameToken.Text)
			}
			total += rawInteriorLength(n.Preamble)
			if n.OpenBrace != nil {
				total += len(n.OpenBrace.Text)
			}
			total += rawInteriorLength(n.Children)
			if n.CloseBrace != nil {
				total += len(n.CloseBrace.Text)
			}
		}
	}
	return total
}
