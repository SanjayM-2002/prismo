package schema

import (
	"reflect"
	"strings"
	"testing"
)

func TestLex(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []token
	}{
		{
			name: "empty",
			src:  "",
			want: []token{{Kind: tokenEOF, Line: 1}},
		},
		{
			name: "field with attribute",
			src:  "email String? @unique",
			want: []token{
				{Kind: tokenIdent, Text: "email", Line: 1},
				{Kind: tokenIdent, Text: "String", Line: 1},
				{Kind: tokenQuestion, Text: "?", Line: 1},
				{Kind: tokenAt, Text: "@", Line: 1},
				{Kind: tokenIdent, Text: "unique", Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
		{
			name: "all symbols",
			src:  "{}()@?=,",
			want: []token{
				{Kind: tokenLBrace, Text: "{", Line: 1},
				{Kind: tokenRBrace, Text: "}", Line: 1},
				{Kind: tokenLParen, Text: "(", Line: 1},
				{Kind: tokenRParen, Text: ")", Line: 1},
				{Kind: tokenAt, Text: "@", Line: 1},
				{Kind: tokenQuestion, Text: "?", Line: 1},
				{Kind: tokenEquals, Text: "=", Line: 1},
				{Kind: tokenComma, Text: ",", Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
		{
			name: "numbers",
			src:  "42 -1 3.14",
			want: []token{
				{Kind: tokenNumber, Text: "42", Line: 1},
				{Kind: tokenNumber, Text: "-1", Line: 1},
				{Kind: tokenNumber, Text: "3.14", Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
		{
			name: "strings",
			src:  `"mysql" "" "say \"hi\"" "a\\b"`,
			want: []token{
				{Kind: tokenString, Text: "mysql", Line: 1},
				{Kind: tokenString, Text: "", Line: 1},
				{Kind: tokenString, Text: `say "hi"`, Line: 1},
				{Kind: tokenString, Text: `a\b`, Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
		{
			name: "identifiers with digits and underscores",
			src:  "user_id2 _x",
			want: []token{
				{Kind: tokenIdent, Text: "user_id2", Line: 1},
				{Kind: tokenIdent, Text: "_x", Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
		{
			name: "default with function call",
			src:  "@default(autoincrement())",
			want: []token{
				{Kind: tokenAt, Text: "@", Line: 1},
				{Kind: tokenIdent, Text: "default", Line: 1},
				{Kind: tokenLParen, Text: "(", Line: 1},
				{Kind: tokenIdent, Text: "autoincrement", Line: 1},
				{Kind: tokenLParen, Text: "(", Line: 1},
				{Kind: tokenRParen, Text: ")", Line: 1},
				{Kind: tokenRParen, Text: ")", Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
		{
			name: "comments are skipped",
			src:  "// a comment\nid Int // trailing",
			want: []token{
				{Kind: tokenIdent, Text: "id", Line: 2},
				{Kind: tokenIdent, Text: "Int", Line: 2},
				{Kind: tokenEOF, Line: 2},
			},
		},
		{
			name: "blank lines collapse into one newline",
			src:  "\n\nmodel User {\n\n\n  id Int\r\n}\n",
			want: []token{
				{Kind: tokenIdent, Text: "model", Line: 3},
				{Kind: tokenIdent, Text: "User", Line: 3},
				{Kind: tokenLBrace, Text: "{", Line: 3},
				{Kind: tokenNewline, Text: "\n", Line: 3},
				{Kind: tokenIdent, Text: "id", Line: 6},
				{Kind: tokenIdent, Text: "Int", Line: 6},
				{Kind: tokenNewline, Text: "\n", Line: 6},
				{Kind: tokenRBrace, Text: "}", Line: 7},
				{Kind: tokenNewline, Text: "\n", Line: 7},
				{Kind: tokenEOF, Line: 8},
			},
		},
		{
			name: "datasource line",
			src:  `url = env("DATABASE_URL")`,
			want: []token{
				{Kind: tokenIdent, Text: "url", Line: 1},
				{Kind: tokenEquals, Text: "=", Line: 1},
				{Kind: tokenIdent, Text: "env", Line: 1},
				{Kind: tokenLParen, Text: "(", Line: 1},
				{Kind: tokenString, Text: "DATABASE_URL", Line: 1},
				{Kind: tokenRParen, Text: ")", Line: 1},
				{Kind: tokenEOF, Line: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lex(tt.src)
			if err != nil {
				t.Fatalf("lex(%q) returned error: %v", tt.src, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("lex(%q)\n got: %+v\nwant: %+v", tt.src, got, tt.want)
			}
		})
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr string
	}{
		{name: "unexpected character", src: "model # User", wantErr: "line 1: unexpected character '#'"},
		{name: "unexpected unicode character", src: "id Int\né", wantErr: "line 2: unexpected character 'é'"},
		{name: "single slash", src: "a / b", wantErr: "line 1: unexpected character '/'"},
		{name: "minus without number", src: "@default(-)", wantErr: "line 1: expected a number after '-'"},
		{name: "unterminated string at end", src: `"mysql`, wantErr: "line 1: unterminated string"},
		{name: "unterminated string at newline", src: "\n\"mysql\n\"", wantErr: "line 2: unterminated string"},
		{name: "backslash at end", src: `"abc\`, wantErr: "line 1: unterminated string"},
		{name: "unknown escape", src: `"a\nb"`, wantErr: `line 1: unknown escape \n in string`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lex(tt.src)
			if err == nil {
				t.Fatalf("lex(%q) returned no error, want %q", tt.src, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("lex(%q) error = %q, want it to contain %q", tt.src, err, tt.wantErr)
			}
		})
	}
}
