package schema

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenString
	tokenNumber
	tokenLBrace
	tokenRBrace
	tokenLParen
	tokenRParen
	tokenAt
	tokenQuestion
	tokenEquals
	tokenComma
	tokenNewline
)

func (k tokenKind) String() string {
	switch k {
	case tokenEOF:
		return "end of file"
	case tokenIdent:
		return "identifier"
	case tokenString:
		return "string"
	case tokenNumber:
		return "number"
	case tokenNewline:
		return "newline"
	}
	for c, kind := range symbols {
		if kind == k {
			return fmt.Sprintf("'%c'", c)
		}
	}
	return fmt.Sprintf("token(%d)", int(k))
}

type token struct {
	Kind tokenKind
	// Text is the token as written, except for strings, where it is the
	// value without quotes.
	Text string
	Line int
}

var symbols = map[byte]tokenKind{
	'{': tokenLBrace,
	'}': tokenRBrace,
	'(': tokenLParen,
	')': tokenRParen,
	'@': tokenAt,
	'?': tokenQuestion,
	'=': tokenEquals,
	',': tokenComma,
}

// lex splits src into tokens. The last token is always tokenEOF.
func lex(src string) ([]token, error) {
	var tokens []token
	line := 1
	i := 0

	for i < len(src) {
		c := src[i]

		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++

		case c == '\n':
			// Fields end at the end of a line, so the parser needs newlines,
			// but only one between two lines of content.
			if len(tokens) > 0 && tokens[len(tokens)-1].Kind != tokenNewline {
				tokens = append(tokens, token{Kind: tokenNewline, Text: "\n", Line: line})
			}
			line++
			i++

		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}

		case isLetter(c):
			start := i
			for i < len(src) && (isLetter(src[i]) || isDigit(src[i])) {
				i++
			}
			tokens = append(tokens, token{Kind: tokenIdent, Text: src[start:i], Line: line})

		case isDigit(c) || c == '-':
			start := i
			i++
			for i < len(src) && (isDigit(src[i]) || src[i] == '.') {
				i++
			}
			text := src[start:i]
			if text == "-" {
				return nil, fmt.Errorf("line %d: expected a number after '-'", line)
			}
			tokens = append(tokens, token{Kind: tokenNumber, Text: text, Line: line})

		case c == '"':
			value, n, err := lexString(src[i:])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			tokens = append(tokens, token{Kind: tokenString, Text: value, Line: line})
			i += n

		default:
			kind, ok := symbols[c]
			if !ok {
				r, _ := utf8.DecodeRuneInString(src[i:])
				return nil, fmt.Errorf("line %d: unexpected character %q", line, r)
			}
			tokens = append(tokens, token{Kind: kind, Text: string(c), Line: line})
			i++
		}
	}

	tokens = append(tokens, token{Kind: tokenEOF, Line: line})
	return tokens, nil
}

// lexString reads a double-quoted string at the start of src. It returns
// the value without quotes and the number of bytes consumed.
func lexString(src string) (string, int, error) {
	var b strings.Builder
	i := 1 // skip the opening quote

	for i < len(src) {
		c := src[i]
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\n':
			return "", 0, fmt.Errorf("unterminated string")
		case '\\':
			if i+1 >= len(src) {
				return "", 0, fmt.Errorf("unterminated string")
			}
			next := src[i+1]
			if next != '"' && next != '\\' {
				return "", 0, fmt.Errorf("unknown escape \\%c in string", next)
			}
			b.WriteByte(next)
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}

	return "", 0, fmt.Errorf("unterminated string")
}

func isLetter(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
