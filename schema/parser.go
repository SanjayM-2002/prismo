package schema

import "fmt"

// Parse parses the contents of a schema file.
func Parse(src string) (*Schema, error) {
	tokens, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens}
	return p.parseSchema()
}

type parser struct {
	tokens []token
	pos    int
}

// peek returns the current token without consuming it.
func (p *parser) peek() token {
	return p.tokens[p.pos]
}

// next consumes and returns the current token.
func (p *parser) next() token {
	tok := p.tokens[p.pos]
	// The last token is EOF; staying on it means callers never index past the end.
	if tok.Kind != tokenEOF {
		p.pos++
	}
	return tok
}

// expect consumes the current token if it has the given kind.
func (p *parser) expect(kind tokenKind) (token, error) {
	tok := p.peek()
	if tok.Kind != kind {
		return token{}, fmt.Errorf("line %d: expected %s, got %s", tok.Line, kind, describe(tok))
	}
	return p.next(), nil
}

func (p *parser) skipNewlines() {
	for p.peek().Kind == tokenNewline {
		p.next()
	}
}

// endOfLine consumes the newline that ends a line. End of file also counts.
func (p *parser) endOfLine() error {
	tok := p.peek()
	switch tok.Kind {
	case tokenNewline:
		p.next()
		return nil
	case tokenEOF:
		return nil
	default:
		return fmt.Errorf("line %d: expected newline, got %s", tok.Line, describe(tok))
	}
}

func (p *parser) parseSchema() (*Schema, error) {
	s := &Schema{}
	for {
		p.skipNewlines()
		tok := p.peek()
		switch {
		case tok.Kind == tokenEOF:
			return s, nil
		case tok.Kind == tokenIdent && tok.Text == "model":
			m, err := p.parseModel()
			if err != nil {
				return nil, err
			}
			s.Models = append(s.Models, m)
		case tok.Kind == tokenIdent && tok.Text == "datasource":
			if s.Datasource != nil {
				return nil, fmt.Errorf("line %d: only one datasource block is allowed", tok.Line)
			}
			ds, err := p.parseDatasource()
			if err != nil {
				return nil, err
			}
			s.Datasource = ds
		default:
			return nil, fmt.Errorf("line %d: expected a model or datasource block, got %s", tok.Line, describe(tok))
		}
	}
}

func (p *parser) parseDatasource() (*Datasource, error) {
	start := p.next() // the "datasource" keyword

	name, err := p.expect(tokenIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenLBrace); err != nil {
		return nil, err
	}

	ds := &Datasource{Name: name.Text, Line: start.Line}
	seen := map[string]bool{}
	for {
		p.skipNewlines()
		switch p.peek().Kind {
		case tokenRBrace:
			p.next()
			return ds, p.endOfLine()
		case tokenEOF:
			return nil, fmt.Errorf("line %d: datasource %s is missing a closing '}'", ds.Line, ds.Name)
		}

		key, err := p.expect(tokenIdent)
		if err != nil {
			return nil, err
		}
		if seen[key.Text] {
			return nil, fmt.Errorf("line %d: duplicate datasource field %q", key.Line, key.Text)
		}
		seen[key.Text] = true

		if _, err := p.expect(tokenEquals); err != nil {
			return nil, err
		}

		switch key.Text {
		case "provider":
			value, err := p.expect(tokenString)
			if err != nil {
				return nil, err
			}
			ds.Provider = value.Text
		case "url":
			if err := p.parseURL(ds); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("line %d: unknown datasource field %q", key.Line, key.Text)
		}

		if err := p.endOfLine(); err != nil {
			return nil, err
		}
	}
}

// parseURL parses the value of a datasource url: "..." or env("NAME").
func (p *parser) parseURL(ds *Datasource) error {
	tok := p.next()
	switch {
	case tok.Kind == tokenString:
		ds.URL = tok.Text
		return nil
	case tok.Kind == tokenIdent && tok.Text == "env":
		if _, err := p.expect(tokenLParen); err != nil {
			return err
		}
		name, err := p.expect(tokenString)
		if err != nil {
			return err
		}
		if _, err := p.expect(tokenRParen); err != nil {
			return err
		}
		ds.URLEnv = name.Text
		return nil
	default:
		return fmt.Errorf("line %d: expected a string or env(\"NAME\") for url, got %s", tok.Line, describe(tok))
	}
}

func (p *parser) parseModel() (Model, error) {
	start := p.next() // the "model" keyword

	name, err := p.expect(tokenIdent)
	if err != nil {
		return Model{}, err
	}
	if _, err := p.expect(tokenLBrace); err != nil {
		return Model{}, err
	}

	m := Model{Name: name.Text, Line: start.Line}
	for {
		p.skipNewlines()
		switch p.peek().Kind {
		case tokenRBrace:
			p.next()
			return m, p.endOfLine()
		case tokenEOF:
			return Model{}, fmt.Errorf("line %d: model %s is missing a closing '}'", m.Line, m.Name)
		}

		f, err := p.parseField()
		if err != nil {
			return Model{}, err
		}
		m.Fields = append(m.Fields, f)
	}
}

func (p *parser) parseField() (Field, error) {
	name, err := p.expect(tokenIdent)
	if err != nil {
		return Field{}, err
	}
	typeTok, err := p.expect(tokenIdent)
	if err != nil {
		return Field{}, err
	}
	typ, ok := ParseFieldType(typeTok.Text)
	if !ok {
		return Field{}, fmt.Errorf("line %d: unknown type %q", typeTok.Line, typeTok.Text)
	}

	f := Field{Name: name.Text, Type: typ, Line: name.Line}
	if p.peek().Kind == tokenQuestion {
		p.next()
		f.Optional = true
	}
	for p.peek().Kind == tokenAt {
		if err := p.parseAttribute(&f); err != nil {
			return Field{}, err
		}
	}
	return f, p.endOfLine()
}

func (p *parser) parseAttribute(f *Field) error {
	p.next() // the "@"

	name, err := p.expect(tokenIdent)
	if err != nil {
		return err
	}

	switch name.Text {
	case "id":
		if f.ID {
			return duplicateAttribute(name)
		}
		f.ID = true
		return p.noArguments(name)
	case "unique":
		if f.Unique {
			return duplicateAttribute(name)
		}
		f.Unique = true
		return p.noArguments(name)
	case "default":
		if f.Default != nil {
			return duplicateAttribute(name)
		}
		d, err := p.parseDefault()
		if err != nil {
			return err
		}
		f.Default = d
		return nil
	default:
		return fmt.Errorf("line %d: unknown attribute @%s", name.Line, name.Text)
	}
}

func (p *parser) noArguments(name token) error {
	if p.peek().Kind == tokenLParen {
		return fmt.Errorf("line %d: @%s takes no arguments", name.Line, name.Text)
	}
	return nil
}

func duplicateAttribute(name token) error {
	return fmt.Errorf("line %d: duplicate attribute @%s", name.Line, name.Text)
}

// parseDefault parses the "(value)" part of @default(value).
func (p *parser) parseDefault() (*Default, error) {
	if _, err := p.expect(tokenLParen); err != nil {
		return nil, err
	}

	tok := p.next()
	var d Default
	switch {
	case tok.Kind == tokenString:
		d = Default{Kind: DefaultString, Value: tok.Text}
	case tok.Kind == tokenNumber:
		d = Default{Kind: DefaultNumber, Value: tok.Text}
	case tok.Kind == tokenIdent && p.peek().Kind == tokenLParen:
		p.next()
		if _, err := p.expect(tokenRParen); err != nil {
			return nil, err
		}
		d = Default{Kind: DefaultFunc, Value: tok.Text}
	case tok.Kind == tokenIdent && (tok.Text == "true" || tok.Text == "false"):
		d = Default{Kind: DefaultBool, Value: tok.Text}
	default:
		return nil, fmt.Errorf("line %d: expected a default value, got %s", tok.Line, describe(tok))
	}

	if _, err := p.expect(tokenRParen); err != nil {
		return nil, err
	}
	return &d, nil
}

// describe formats a token for error messages, e.g. `identifier "User"`.
func describe(tok token) string {
	switch tok.Kind {
	case tokenIdent, tokenString, tokenNumber:
		return fmt.Sprintf("%s %q", tok.Kind, tok.Text)
	default:
		return tok.Kind.String()
	}
}
