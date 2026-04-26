package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type whereExpr interface {
	Eval(r row) bool
}

type orExpr struct{ left, right whereExpr }

func (e *orExpr) Eval(r row) bool { return e.left.Eval(r) || e.right.Eval(r) }

type andExpr struct{ left, right whereExpr }

func (e *andExpr) Eval(r row) bool { return e.left.Eval(r) && e.right.Eval(r) }

type notExpr struct{ inner whereExpr }

func (e *notExpr) Eval(r row) bool { return !e.inner.Eval(r) }

type cmpOp int

const (
	opEq cmpOp = iota
	opNeq
	opLt
	opLte
	opGt
	opGte
)

type operand interface {
	resolve(r row) any
}

type identOperand struct{ name string }

func (o *identOperand) resolve(r row) any {
	if o.name == "id" {
		return r.id
	}
	return r.cols[o.name]
}

type numLit struct{ v float64 }

func (o *numLit) resolve(r row) any { return o.v }

type strLit struct{ v string }

func (o *strLit) resolve(r row) any { return o.v }

type boolLit struct{ v bool }

func (o *boolLit) resolve(r row) any { return o.v }

type nullLit struct{}

func (o *nullLit) resolve(r row) any { return nil }

type cmpExpr struct {
	op          cmpOp
	left, right operand
}

func (e *cmpExpr) Eval(r row) bool {
	_, lNull := e.left.(*nullLit)
	_, rNull := e.right.(*nullLit)
	lv := e.left.resolve(r)
	rv := e.right.resolve(r)

	if lNull || rNull {
		switch e.op {
		case opEq:
			return lv == nil && rv == nil
		case opNeq:
			return !(lv == nil && rv == nil)
		default:
			return false
		}
	}

	if lv == nil || rv == nil {
		return false
	}

	if lf, ok := toFloat(lv); ok {
		if rf, ok := toFloat(rv); ok {
			return cmpFloat(e.op, lf, rf)
		}
		return false
	}
	if ls, ok := lv.(string); ok {
		if rs, ok := rv.(string); ok {
			return cmpString(e.op, ls, rs)
		}
		return false
	}
	if lb, ok := lv.(bool); ok {
		if rb, ok := rv.(bool); ok {
			return cmpBool(e.op, lb, rb)
		}
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	}
	return 0, false
}

func cmpFloat(op cmpOp, l, r float64) bool {
	switch op {
	case opEq:
		return l == r
	case opNeq:
		return l != r
	case opLt:
		return l < r
	case opLte:
		return l <= r
	case opGt:
		return l > r
	case opGte:
		return l >= r
	}
	return false
}

func cmpString(op cmpOp, l, r string) bool {
	switch op {
	case opEq:
		return l == r
	case opNeq:
		return l != r
	case opLt:
		return l < r
	case opLte:
		return l <= r
	case opGt:
		return l > r
	case opGte:
		return l >= r
	}
	return false
}

func cmpBool(op cmpOp, l, r bool) bool {
	switch op {
	case opEq:
		return l == r
	case opNeq:
		return l != r
	}
	return false
}

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
