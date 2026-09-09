package parsers


import "fmt"

type TokenType int

const (
	TokenWhitespace TokenType = iota
	TokenComment
	TokenString // quoted or unquoted
	TokenBraceOpen
	TokenBraceClose
)

type Token struct {
	Type TokenType
	Text string
}

func Lex(source string) ([]Token, error) {
	var tokens []Token
	i := 0
	for i < len(source) {
		// Try to match whitespace
		if isSpace(source[i]) {
			start := i
			for i < len(source) && isSpace(source[i]) {
				i++
			}
			tokens = append(tokens, Token{Type: TokenWhitespace, Text: source[start:i]})
			continue
		}

		// Try to match comment // — the only comment form the engine
		// knows. `/* ... */` is NOT a comment in siege config files
		// (SGPV/SGVG have no block-comment handling); it lexes as
		// ordinary text and must be preserved as such.
		if i+1 < len(source) && source[i] == '/' && source[i+1] == '/' {
			start := i
			for i < len(source) && source[i] != '\n' {
				i++
			}
			tokens = append(tokens, Token{Type: TokenComment, Text: source[start:i]})
			continue
		}

		// Brace
		if source[i] == '{' {
			tokens = append(tokens, Token{Type: TokenBraceOpen, Text: "{"})
			i++
			continue
		}
		if source[i] == '}' {
			tokens = append(tokens, Token{Type: TokenBraceClose, Text: "}"})
			i++
			continue
		}

		// Quoted string. Engine SGPV (bg_saga.c:271-295) parses quoted
		// values to the next bare double quote only — newlines do NOT
		// terminate the token, so descriptions and briefings may span
		// multiple lines inside one quoted value.
		if source[i] == '"' {
			start := i
			i++
			for i < len(source) && source[i] != '"' {
				i++
			}
			if i < len(source) && source[i] == '"' {
				i++ // include closing quote
			}
			tokens = append(tokens, Token{Type: TokenString, Text: source[start:i]})
			continue
		}

		// Unquoted string
		start := i
		for i < len(source) && !isSpace(source[i]) && source[i] != '{' && source[i] != '}' && source[i] != '"' {
			if i+1 < len(source) && source[i] == '/' && source[i+1] == '/' {
				break
			}
			i++
		}
		tokens = append(tokens, Token{Type: TokenString, Text: source[start:i]})
	}
	return tokens, nil
}

// LexDiagnostics reports lexical conditions the ENGINE would reject or
// misread, with messages matching the engine's own errors. The lexer
// itself still preserves the raw draft (Lex never fails); these
// diagnostics are surfaced through AssessMBCHSourceBuffers so validation
// can catch a draft the game would refuse to load.
func LexDiagnostics(source string) []string {
	var diags []string
	i := 0
	for i < len(source) {
		switch {
		case isSpace(source[i]):
			i++
		case source[i] == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case source[i] == '"':
			start := i
			i++
			for i < len(source) && source[i] != '"' {
				i++
			}
			if i >= len(source) {
				// Engine: Com_Error(ERR_DROP, "Unexpected EOF while
				// looking for endquote...") — bg_saga.c:291-293.
				diags = append(diags, fmt.Sprintf("unexpected EOF while looking for endquote (unterminated quote starting at byte %d)", start))
			} else {
				i++ // closing quote
			}
		case source[i] == '{' || source[i] == '}':
			i++
		default:
			for i < len(source) && !isSpace(source[i]) && source[i] != '{' && source[i] != '}' && source[i] != '"' {
				if i+1 < len(source) && source[i] == '/' && source[i+1] == '/' {
					break
				}
				i++
			}
		}
	}
	return diags
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}
