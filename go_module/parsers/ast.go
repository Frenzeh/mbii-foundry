package parsers

import (
	"strings"
)

type ASTNode interface {
	String() string
}

type ASTToken struct {
	Type TokenType
	Text string
}

func (t *ASTToken) String() string {
	return t.Text
}

type ASTBlock struct {
	NameToken  *ASTToken
	Preamble   []ASTNode // Whitespace/comments between name and {
	OpenBrace  *ASTToken
	Children   []ASTNode // Inside the block
	CloseBrace *ASTToken
}

func (b *ASTBlock) String() string {
	var sb strings.Builder
	if b.NameToken != nil {
		sb.WriteString(b.NameToken.String())
	}
	for _, n := range b.Preamble {
		sb.WriteString(n.String())
	}
	if b.OpenBrace != nil {
		sb.WriteString(b.OpenBrace.String())
	}
	for _, n := range b.Children {
		sb.WriteString(n.String())
	}
	if b.CloseBrace != nil {
		sb.WriteString(b.CloseBrace.String())
	}
	return sb.String()
}

type ASTDocument struct {
	Nodes []ASTNode
}

func (d *ASTDocument) String() string {
	var sb strings.Builder
	for _, n := range d.Nodes {
		sb.WriteString(n.String())
	}
	return sb.String()
}

// unquote strips the surrounding quotes of a quoted string token. With
// the SGPV lexer a token is either fully quoted or quote-free; the only
// orphan case is an unterminated quote at EOF, where we still peel the
// dangling leading (or trailing) quote instead of leaking it into the
// parsed value — leaked quotes would come back doubled on the next save
// (syncStr wraps values in quotes unconditionally).
func unquote(s string) string {
	if len(s) >= 1 && s[0] == '"' {
		s = s[1:]
	}
	if len(s) >= 1 && s[len(s)-1] == '"' {
		s = s[:len(s)-1]
	}
	return s
}

// parseAST converts a stream of tokens into a hierarchical AST Document
func parseAST(tokens []Token) *ASTDocument {
	doc := &ASTDocument{}
	i := 0

	var parseNodes func(isBlock bool) []ASTNode
	parseNodes = func(isBlock bool) []ASTNode {
		var nodes []ASTNode
		for i < len(tokens) {
			tok := tokens[i]

			if tok.Type == TokenWhitespace || tok.Type == TokenComment {
				nodes = append(nodes, &ASTToken{Type: tok.Type, Text: tok.Text})
				i++
				continue
			}

			if tok.Type == TokenBraceClose {
				if isBlock {
					// Don't consume it here, let the block parser consume it
					break
				}
				// Stray close brace, just treat as token
				nodes = append(nodes, &ASTToken{Type: tok.Type, Text: tok.Text})
				i++
				continue
			}

			if tok.Type == TokenString {
				// Is it a block name?
				// Look ahead to see if there is an OpenBrace
				isBlockName := false
				j := i + 1
				var preamble []ASTNode
				for j < len(tokens) {
					t2 := tokens[j]
					if t2.Type == TokenWhitespace || t2.Type == TokenComment {
						preamble = append(preamble, &ASTToken{Type: t2.Type, Text: t2.Text})
						j++
					} else if t2.Type == TokenBraceOpen {
						isBlockName = true
						break
					} else {
						break
					}
				}

				if isBlockName {
					// We found a block!
					nameTok := &ASTToken{Type: tok.Type, Text: tok.Text}
					openBrace := &ASTToken{Type: tokens[j].Type, Text: tokens[j].Text}
					i = j + 1

					children := parseNodes(true)

					var closeBrace *ASTToken
					if i < len(tokens) && tokens[i].Type == TokenBraceClose {
						closeBrace = &ASTToken{Type: tokens[i].Type, Text: tokens[i].Text}
						i++
					} else {
						// Malformed block, missing close brace. We just preserve what we can.
					}

					block := &ASTBlock{
						NameToken:  nameTok,
						Preamble:   preamble,
						OpenBrace:  openBrace,
						Children:   children,
						CloseBrace: closeBrace,
					}
					nodes = append(nodes, block)
					continue
				}

				// Normal string token (key or value)
				nodes = append(nodes, &ASTToken{Type: tok.Type, Text: tok.Text})
				i++
				continue
			}

			if tok.Type == TokenBraceOpen {
				// Block without a name? Just treat as a block with nil NameToken
				openBrace := &ASTToken{Type: tok.Type, Text: tok.Text}
				i++
				children := parseNodes(true)
				var closeBrace *ASTToken
				if i < len(tokens) && tokens[i].Type == TokenBraceClose {
					closeBrace = &ASTToken{Type: tokens[i].Type, Text: tokens[i].Text}
					i++
				}
				nodes = append(nodes, &ASTBlock{
					NameToken:  nil,
					OpenBrace:  openBrace,
					Children:   children,
					CloseBrace: closeBrace,
				})
				continue
			}

			// Fallback
			nodes = append(nodes, &ASTToken{Type: tok.Type, Text: tok.Text})
			i++
		}
		return nodes
	}

	doc.Nodes = parseNodes(false)
	return doc
}
