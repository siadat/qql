package main

import (
	"fmt"
	"strconv"
	"strings"
)

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokNumber
	tokString
	tokTrue
	tokFalse
	tokNull
	tokAnd
	tokOr
	tokNot
	tokEq
	tokNeq
	tokLt
	tokLte
	tokGt
	tokGte
	tokLParen
	tokRParen
	tokSelect
	tokFrom
	tokWhere
	tokComma
	tokStar
	tokSource
)

type token struct {
	kind tokKind
	text string
	pos  int
}

func tokenize(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if len(toks) > 0 && toks[len(toks)-1].kind == tokFrom {
			start := i
			for i < len(src) && src[i] != ' ' && src[i] != '\t' && src[i] != '\n' && src[i] != '\r' {
				i++
			}
			toks = append(toks, token{tokSource, src[start:i], start})
			continue
		}
		start := i
		switch {
		case isIdentStart(c):
			i++
			for i < len(src) && isIdentBody(src[i]) {
				i++
			}
			for i < len(src) && src[i] == '.' && i+1 < len(src) && isIdentBody(src[i+1]) {
				i++
				for i < len(src) && isIdentBody(src[i]) {
					i++
				}
			}
			text := src[start:i]
			kind := tokIdent
			switch strings.ToLower(text) {
			case "and":
				kind = tokAnd
			case "or":
				kind = tokOr
			case "not":
				kind = tokNot
			case "true":
				kind = tokTrue
			case "false":
				kind = tokFalse
			case "null":
				kind = tokNull
			case "select":
				kind = tokSelect
			case "from":
				kind = tokFrom
			case "where":
				kind = tokWhere
			}
			toks = append(toks, token{kind, text, start})
		case isDigit(c):
			i++
			for i < len(src) && isDigit(src[i]) {
				i++
			}
			if i < len(src) && src[i] == '.' && i+1 < len(src) && isDigit(src[i+1]) {
				i++
				for i < len(src) && isDigit(src[i]) {
					i++
				}
			}
			toks = append(toks, token{tokNumber, src[start:i], start})
		case c == '\'' || c == '"':
			quote := c
			i++
			for i < len(src) && src[i] != quote {
				i++
			}
			if i >= len(src) {
				return nil, fmt.Errorf("unterminated string starting at offset %d", start)
			}
			text := src[start+1 : i]
			i++
			toks = append(toks, token{tokString, text, start})
		case c == '(':
			i++
			toks = append(toks, token{tokLParen, "(", start})
		case c == ')':
			i++
			toks = append(toks, token{tokRParen, ")", start})
		case c == ',':
			i++
			toks = append(toks, token{tokComma, ",", start})
		case c == '*':
			i++
			toks = append(toks, token{tokStar, "*", start})
		case c == '=':
			i++
			toks = append(toks, token{tokEq, "=", start})
		case c == '!':
			if i+1 < len(src) && src[i+1] == '=' {
				i += 2
				toks = append(toks, token{tokNeq, "!=", start})
			} else {
				return nil, fmt.Errorf("unexpected '!' at offset %d (did you mean '!='?)", start)
			}
		case c == '<':
			i++
			if i < len(src) && src[i] == '=' {
				i++
				toks = append(toks, token{tokLte, "<=", start})
			} else {
				toks = append(toks, token{tokLt, "<", start})
			}
		case c == '>':
			i++
			if i < len(src) && src[i] == '=' {
				i++
				toks = append(toks, token{tokGte, ">=", start})
			} else {
				toks = append(toks, token{tokGt, ">", start})
			}
		default:
			return nil, fmt.Errorf("unexpected character %q at offset %d", c, start)
		}
	}
	toks = append(toks, token{tokEOF, "", len(src)})
	return toks, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentBody(c byte) bool {
	return isIdentStart(c) || isDigit(c)
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) advance() token {
	t := p.toks[p.pos]
	p.pos++
	return t
}

func parseSQL(src string) (selected []string, source string, pred whereExpr, err error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, "", nil, err
	}
	p := &parser{toks: toks}

	if p.peek().kind == tokSelect {
		p.advance()
		selected, err = parseSelectList(p)
		if err != nil {
			return nil, "", nil, err
		}
	}

	if p.peek().kind == tokFrom {
		p.advance()
		t := p.peek()
		if t.kind != tokSource {
			return nil, "", nil, fmt.Errorf("expected source after FROM at offset %d, got %q", t.pos, t.text)
		}
		p.advance()
		source = t.text
	}

	if p.peek().kind == tokWhere {
		p.advance()
		pred, err = p.parseOr()
		if err != nil {
			return nil, "", nil, err
		}
	}

	if t := p.peek(); t.kind != tokEOF {
		return nil, "", nil, fmt.Errorf("unexpected token %q at offset %d", t.text, t.pos)
	}
	return selected, source, pred, nil
}

func parseSelectList(p *parser) ([]string, error) {
	if p.peek().kind == tokStar {
		p.advance()
		return nil, nil
	}
	var cols []string
	for {
		t := p.advance()
		if t.kind != tokIdent {
			return nil, fmt.Errorf("expected column name at offset %d, got %q", t.pos, t.text)
		}
		cols = append(cols, t.text)
		if p.peek().kind != tokComma {
			break
		}
		p.advance()
	}
	return cols, nil
}

func parseWhere(src string) (whereExpr, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("empty where expression")
	}
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	e, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		t := p.peek()
		return nil, fmt.Errorf("unexpected token %q at offset %d", t.text, t.pos)
	}
	return e, nil
}

func (p *parser) parseOr() (whereExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOr {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &orExpr{left, right}
	}
	return left, nil
}

func (p *parser) parseAnd() (whereExpr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokAnd {
		p.advance()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &andExpr{left, right}
	}
	return left, nil
}

func (p *parser) parseNot() (whereExpr, error) {
	if p.peek().kind == tokNot {
		p.advance()
		inner, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &notExpr{inner}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (whereExpr, error) {
	if p.peek().kind == tokLParen {
		p.advance()
		e, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			t := p.peek()
			return nil, fmt.Errorf("expected ')' at offset %d, got %q", t.pos, t.text)
		}
		p.advance()
		return e, nil
	}
	return p.parseComparison()
}

func (p *parser) parseComparison() (whereExpr, error) {
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	var op cmpOp
	switch t.kind {
	case tokEq:
		op = opEq
	case tokNeq:
		op = opNeq
	case tokLt:
		op = opLt
	case tokLte:
		op = opLte
	case tokGt:
		op = opGt
	case tokGte:
		op = opGte
	default:
		return nil, fmt.Errorf("expected comparison operator at offset %d, got %q", t.pos, t.text)
	}
	p.advance()
	right, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	return &cmpExpr{op: op, left: left, right: right}, nil
}

func (p *parser) parseOperand() (operand, error) {
	t := p.advance()
	switch t.kind {
	case tokIdent:
		return &identOperand{name: t.text}, nil
	case tokNumber:
		f, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q at offset %d", t.text, t.pos)
		}
		return &numLit{v: f}, nil
	case tokString:
		return &strLit{v: t.text}, nil
	case tokTrue:
		return &boolLit{v: true}, nil
	case tokFalse:
		return &boolLit{v: false}, nil
	case tokNull:
		return &nullLit{}, nil
	default:
		return nil, fmt.Errorf("expected operand at offset %d, got %q", t.pos, t.text)
	}
}
