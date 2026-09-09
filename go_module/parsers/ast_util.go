package parsers

import (
	"fmt"
	"sort"
	"strings"
)

// truncateSGPV returns the value the ENGINE actually stores: SGPV
// terminates any value — quoted or not — at an inline `//`
// (bg_saga.c:283-285; same check on typed reads at bg_saga.c:1621-1625).
// Model values use the truncated form; the raw document text is
// preserved so a draft is never rewritten into the truncated shape.
func truncateSGPV(s string) string {
	if i := strings.Index(s, "//"); i != -1 {
		return s[:i]
	}
	return s
}

// getFieldValueSGPV is getFieldValue with SGPV truncation applied — the
// value the engine's paired-value reader would hand to the model.
func getFieldValueSGPV(block *ASTBlock, key string) (string, bool) {
	v, ok := getFieldValue(block, key)
	return truncateSGPV(v), ok
}

// getFieldValue finds the FIRST occurrence of `key` (case-insensitive)
// and returns its unquoted value.
func getFieldValue(block *ASTBlock, key string) (string, bool) {
	keyLower := strings.ToLower(key)

	
	for i := 0; i < len(block.Children); i++ {
		node := block.Children[i]
		if tok, ok := node.(*ASTToken); ok && tok.Type == TokenString {
			if strings.ToLower(unquote(tok.Text)) == keyLower {
				for j := i + 1; j < len(block.Children); j++ {
					vNode := block.Children[j]
					if vTok, ok := vNode.(*ASTToken); ok && vTok.Type == TokenString {
						return unquote(vTok.Text), true
					}
					if _, ok := vNode.(*ASTBlock); ok {
						break
					}
				}
			}
			for i++; i < len(block.Children); i++ {
				if vTok, ok := block.Children[i].(*ASTToken); ok && vTok.Type == TokenString {
					break
				}
				if _, ok := block.Children[i].(*ASTBlock); ok {
					i--
					break
				}
			}
		}
	}
	return "", false
}

// setFieldValue updates the FIRST occurrence of an existing field or adds a new one.
func setFieldValue(block *ASTBlock, key string, val string, useQuotes bool) {
	keyLower := strings.ToLower(key)
	
	valStr := val
	if useQuotes {
		valStr = fmt.Sprintf("\"%s\"", val)
	} else if val == "" {
		valStr = "\"\""
	}
	
	for i := 0; i < len(block.Children); i++ {
		node := block.Children[i]
		if tok, ok := node.(*ASTToken); ok && tok.Type == TokenString {
			if strings.ToLower(unquote(tok.Text)) == keyLower {
				for j := i + 1; j < len(block.Children); j++ {
					vNode := block.Children[j]
					if vTok, ok := vNode.(*ASTToken); ok && vTok.Type == TokenString {
						if vTok.Text == valStr {
							return
						}
						if unquote(vTok.Text) == val {
							return
						}
						// Raw-draft preservation: a token holding the
						// full (untruncated) text satisfies a model value
						// the engine would truncate at `//`.
						if truncateSGPV(unquote(vTok.Text)) == val {
							return
						}
						vTok.Text = valStr
						return
					}
					if _, ok := vNode.(*ASTBlock); ok {
						break
					}
				}
				
				block.Children = append(block.Children[:i+1], append([]ASTNode{
					&ASTToken{Type: TokenWhitespace, Text: " "},
					&ASTToken{Type: TokenString, Text: valStr},
				}, block.Children[i+1:]...)...)
				return
			}
			for i++; i < len(block.Children); i++ {
				if vTok, ok := block.Children[i].(*ASTToken); ok && vTok.Type == TokenString {
					break
				}
				if _, ok := block.Children[i].(*ASTBlock); ok {
					i--
					break
				}
			}
		}
	}
	
	insertIdx := len(block.Children)
	var indent string = "\t"
	if insertIdx > 0 {
		lastNode := block.Children[insertIdx-1]
		if wTok, ok := lastNode.(*ASTToken); ok && wTok.Type == TokenWhitespace {
			if strings.Contains(wTok.Text, "\n") {
				lines := strings.Split(wTok.Text, "\n")
				indent = lines[len(lines)-1]
				if indent == "" {
					// Whitespace ending in a bare newline (freshly
					// created blocks): fall back to a single tab.
					indent = "\t"
				}
			}
		}
	} else {
		block.Children = append(block.Children, &ASTToken{Type: TokenWhitespace, Text: "\n"})
		insertIdx = 1
	}
	
	var newNodes []ASTNode
	if insertIdx > 0 {
		lastNode := block.Children[insertIdx-1]
		if wTok, ok := lastNode.(*ASTToken); ok && wTok.Type == TokenWhitespace {
			if !strings.Contains(wTok.Text, "\n") {
				newNodes = append(newNodes, &ASTToken{Type: TokenWhitespace, Text: "\n"})
			}
		} else {
			newNodes = append(newNodes, &ASTToken{Type: TokenWhitespace, Text: "\n"})
		}
	}
	
	newNodes = append(newNodes, 
		&ASTToken{Type: TokenWhitespace, Text: indent},
		&ASTToken{Type: TokenString, Text: key},
		&ASTToken{Type: TokenWhitespace, Text: "\t\t"},
		&ASTToken{Type: TokenString, Text: valStr},
		&ASTToken{Type: TokenWhitespace, Text: "\n"},
	)
	
	block.Children = append(block.Children, newNodes...)
}

func removeField(block *ASTBlock, key string) {
	keyLower := strings.ToLower(key)
	
	for i := 0; i < len(block.Children); i++ {
		node := block.Children[i]
		if tok, ok := node.(*ASTToken); ok && tok.Type == TokenString {
			if strings.ToLower(unquote(tok.Text)) == keyLower {
				startIdx := i
				for startIdx > 0 {
					if wTok, ok := block.Children[startIdx-1].(*ASTToken); ok && wTok.Type == TokenWhitespace {
						if strings.Contains(wTok.Text, "\n") {
							break
						}
						startIdx--
					} else {
						break
					}
				}
				
				endIdx := i
				for endIdx = i + 1; endIdx < len(block.Children); endIdx++ {
					if vTok, ok := block.Children[endIdx].(*ASTToken); ok && vTok.Type == TokenString {
						break
					}
					if _, ok := block.Children[endIdx].(*ASTBlock); ok {
						endIdx--
						break
					}
				}
				
				if endIdx < len(block.Children) {
					for endIdx++; endIdx < len(block.Children); endIdx++ {
						if wTok, ok := block.Children[endIdx].(*ASTToken); ok {
							if wTok.Type == TokenWhitespace {
								if strings.Contains(wTok.Text, "\n") {
									break
								}
							} else if wTok.Type == TokenComment {
								continue
							} else {
								endIdx--
								break
							}
						} else {
							endIdx--
							break
						}
					}
				}
				
				block.Children = append(block.Children[:startIdx], block.Children[endIdx:]...)
				
				i = startIdx - 1
				if i < -1 {
					i = -1
				}
				continue
			}
			for i++; i < len(block.Children); i++ {
				if vTok, ok := block.Children[i].(*ASTToken); ok && vTok.Type == TokenString {
					break
				}
				if _, ok := block.Children[i].(*ASTBlock); ok {
					i--
					break
				}
			}
		}
	}
}

// cloneASTDocument deep-copies a document so Generate* can sync into a
// private snapshot. The caller's retained AST baseline (the parse-time
// document kept on the model's ctx) must stay pristine: editors render
// the live source panel by calling Generate on every keystroke, and a
// mutated baseline would compound deletions and reorderings across
// calls.
func cloneASTDocument(doc *ASTDocument) *ASTDocument {
	if doc == nil {
		return nil
	}
	var cloneNode func(ASTNode) ASTNode
	cloneNode = func(n ASTNode) ASTNode {
		switch t := n.(type) {
		case *ASTToken:
			return &ASTToken{Type: t.Type, Text: t.Text}
		case *ASTBlock:
			var name, open, close *ASTToken
			if t.NameToken != nil {
				name = &ASTToken{Type: t.NameToken.Type, Text: t.NameToken.Text}
			}
			if t.OpenBrace != nil {
				open = &ASTToken{Type: t.OpenBrace.Type, Text: t.OpenBrace.Text}
			}
			if t.CloseBrace != nil {
				close = &ASTToken{Type: t.CloseBrace.Type, Text: t.CloseBrace.Text}
			}
			b := &ASTBlock{NameToken: name, OpenBrace: open, CloseBrace: close}
			for _, c := range t.Preamble {
				b.Preamble = append(b.Preamble, cloneNode(c))
			}
			for _, c := range t.Children {
				b.Children = append(b.Children, cloneNode(c))
			}
			return b
		default:
			return n
		}
	}
	out := &ASTDocument{}
	for _, n := range doc.Nodes {
		out.Nodes = append(out.Nodes, cloneNode(n))
	}
	return out
}

// walkSGPVPairs visits paired key/value candidates exactly the way the
// engine's SGPV (bg_saga.c:216-300) discovers them inside a group:
//
//   - the first string token on each line is the key candidate;
//   - its value is the next string token, unless a nested block (group
//     start) comes first, in which case the key is bare and the whole
//     group is skipped;
//   - after a pair the rest of the line is dead text (SGPV skips to the
//     next newline for any non-matching key), so tokens after the value
//     are never misread as keys.
//
// visit receives the key node index, the unquoted key, the value node
// index (-1 when bare), the unquoted value and whether a value exists.
// Quoted values may span lines: the token text carries the newlines, so
// the post-pair line skip simply looks for the next whitespace token
// containing a newline AFTER the value token.
func walkSGPVPairs(nodes []ASTNode, visit func(keyIdx int, key string, valIdx int, val string, hasVal bool) bool) {
	i := 0
	for i < len(nodes) {
		tok, ok := nodes[i].(*ASTToken)
		if !ok || tok.Type != TokenString {
			i++
			continue
		}
		key := unquote(tok.Text)
		valIdx := -1
		val := ""
		for j := i + 1; j < len(nodes); j++ {
			if _, ok := nodes[j].(*ASTBlock); ok {
				break // group start: this key is bare
			}
			if t2, ok := nodes[j].(*ASTToken); ok && t2.Type == TokenString {
				valIdx = j
				val = truncateSGPV(unquote(t2.Text))
				break
			}
		}
		if !visit(i, key, valIdx, val, valIdx != -1) {
			return
		}
		// Skip to the end of the line (past any nested groups).
		k := i + 1
		if valIdx != -1 {
			k = valIdx + 1
		}
		for k < len(nodes) {
			if _, ok := nodes[k].(*ASTBlock); ok {
				k++ // whole group is dead text for this line
				continue
			}
			if t2, ok := nodes[k].(*ASTToken); ok && t2.Type == TokenWhitespace && strings.Contains(t2.Text, "\n") {
				k++
				break
			}
			k++
		}
		i = k
	}
}

// syncExtraFieldsAST reconciles a block's untyped (extra) field lines
// with the model's extras map, losslessly in both directions:
//
//   - lines whose key was modeled at parse time but has since been
//     deleted from the map are removed from the document;
//   - map entries are upserted in sorted key order (deterministic
//     output; new keys append at the block tail with matching indent).
//
// isTyped reports whether a key is modeled natively (never removed and
// never written from the extras map). Bare keys (no value token) are
// left untouched — they carry no modeled state.
func syncExtraFieldsAST(block *ASTBlock, extras map[string]string, isTyped func(string) bool) {
	// Pass 1: remove deleted fields (has a value, unmodeled, absent from map).
	for {
		removed := false
		walkSGPVPairs(block.Children, func(keyIdx int, key string, valIdx int, _ string, hasVal bool) bool {
			if hasVal && !isTyped(key) {
				if _, live := extras[key]; !live {
					removeFieldRange(&block.Children, keyIdx, valIdx)
					removed = true
					return false // restart: indices shifted
				}
			}
			return true
		})
		if !removed {
			break
		}
	}

	// Pass 2: upsert in sorted order for deterministic output.
	keys := make([]string, 0, len(extras))
	for k := range extras {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := extras[k]
		if strings.ContainsAny(v, " \t\n") && !strings.HasPrefix(v, "{") {
			setFieldValue(block, k, v, true)
		} else if strings.HasPrefix(v, "{") {
			// Raw nested block stored as text: only rewrite when the
			// model actually changed shape.
			setFieldValue(block, k, v, false)
		} else {
			setFieldValue(block, k, v, false)
		}
	}
}

// removeFieldRange deletes nodes [keyIdx..valIdx] from children along
// with the trailing whitespace up to and including the line's newline,
// and the leading intra-line whitespace — the same cleanup removeField
// applies, but for an already-located line.
func removeFieldRange(children *[]ASTNode, keyIdx, valIdx int) {
	nodes := *children
	startIdx := keyIdx
	for startIdx > 0 {
		if wTok, ok := nodes[startIdx-1].(*ASTToken); ok && wTok.Type == TokenWhitespace {
			if strings.Contains(wTok.Text, "\n") {
				break
			}
			startIdx--
		} else {
			break
		}
	}
	endIdx := valIdx
	if endIdx < len(nodes) {
		for endIdx++; endIdx < len(nodes); endIdx++ {
			if wTok, ok := nodes[endIdx].(*ASTToken); ok {
				if wTok.Type == TokenWhitespace {
					if strings.Contains(wTok.Text, "\n") {
						break
					}
				} else if wTok.Type == TokenComment {
					continue
				} else {
					endIdx--
					break
				}
			} else {
				endIdx--
				break
			}
		}
	}
	*children = append(nodes[:startIdx], nodes[endIdx:]...)
}

// walkTokenPairs visits key/value pairs sequentially: every string
// token is a key and the NEXT string token is its value (nested blocks
// make the key bare). This mirrors the engine's COM_Parse-style parsers
// (saber/vehicle files), where every pair is effective regardless of
// line layout — unlike SGPV, which is line-structured.
func walkTokenPairs(nodes []ASTNode, visit func(keyIdx int, key string, valIdx int, val string, hasVal bool) bool) {
	i := 0
	for i < len(nodes) {
		tok, ok := nodes[i].(*ASTToken)
		if !ok || tok.Type != TokenString {
			i++
			continue
		}
		key := unquote(tok.Text)
		valIdx := -1
		val := ""
		for j := i + 1; j < len(nodes); j++ {
			if _, ok := nodes[j].(*ASTBlock); ok {
				break
			}
			if t2, ok := nodes[j].(*ASTToken); ok && t2.Type == TokenString {
				valIdx = j
				val = unquote(t2.Text)
				break
			}
		}
		if !visit(i, key, valIdx, val, valIdx != -1) {
			return
		}
		if valIdx != -1 {
			i = valIdx + 1
		} else {
			i++
		}
	}
}

// removeStaleExtrasSeq is removeStaleExtras with COM_Parse-style
// sequential pairing (saber/vehicle files).
func removeStaleExtrasSeq(block *ASTBlock, live map[string]string, typed func(string) bool) {
	for {
		removed := false
		walkTokenPairs(block.Children, func(keyIdx int, key string, valIdx int, _ string, hasVal bool) bool {
			if hasVal && !typed(key) {
				if _, ok := live[key]; !ok {
					removeFieldRange(&block.Children, keyIdx, valIdx)
					removed = true
					return false
				}
			}
			return true
		})
		if !removed {
			break
		}
	}
}
