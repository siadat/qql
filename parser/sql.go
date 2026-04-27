package parser

import (
	"fmt"
	"regexp"
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
	tokMatches
	tokLParen
	tokRParen
	tokSelect
	tokFrom
	tokWhere
	tokOrder
	tokBy
	tokAsc
	tokDesc
	tokLimit
	tokOffset
	tokWith
	tokComma
	tokStar
	tokSource
	tokExclude
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
			case "order":
				kind = tokOrder
			case "by":
				kind = tokBy
			case "asc":
				kind = tokAsc
			case "desc":
				kind = tokDesc
			case "limit":
				kind = tokLimit
			case "offset":
				kind = tokOffset
			case "with":
				kind = tokWith
			case "matches":
				kind = tokMatches
			case "exclude":
				kind = tokExclude
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

type parseState struct {
	toks []token
	pos  int
}

func (p *parseState) peek() token { return p.toks[p.pos] }

func (p *parseState) advance() token {
	t := p.toks[p.pos]
	p.pos++
	return t
}

type OrderTerm struct {
	Col  string
	Desc bool
}

// WithOptions holds the configuration knobs parsed from a trailing WITH clause.
// All fields are optional; an empty value means the user did not set that key.
type WithOptions struct {
	Prefix   string
	Provider string
}

// ParseSQL parses a qql query and returns the parsed pieces.
//
// whereRaw is the original substring of `src` covering the WHERE expression
// (between WHERE and the next clause), trimmed of trailing whitespace. It is
// empty when there is no WHERE clause. External providers receive this as a
// hint so they can choose to filter at the source; qql still re-applies the
// parsed `pred` to whatever rows the provider returns.
//
// limit is -1 when no LIMIT clause is present. LIMIT 0 is a valid query that
// yields zero rows. offset defaults to 0 (no rows skipped).
//
// isCount is true for `SELECT COUNT(*)`. When set, the caller should collapse
// the post-WHERE rows into a single row with a `count` column. selected is nil
// in that case.
//
// excluded is the column list from `SELECT * EXCLUDE(col1, col2, ...)`. It is
// only set when the projection was a star plus an EXCLUDE clause; the caller
// should drop those columns from whatever set `*` would otherwise produce.
func ParseSQL(src string) (selected []string, excluded []string, source string, pred WhereExpr, orderBy []OrderTerm, limit, offset int, with WithOptions, whereRaw string, isCount bool, err error) {
	limit = -1
	fail := func(e error) (selected []string, excluded []string, source string, pred WhereExpr, orderBy []OrderTerm, limit, offset int, with WithOptions, whereRaw string, isCount bool, err error) {
		return nil, nil, "", nil, nil, -1, 0, WithOptions{}, "", false, e
	}
	toks, err := tokenize(src)
	if err != nil {
		return fail(err)
	}
	p := &parseState{toks: toks}

	if p.peek().kind == tokSelect {
		p.advance()
		selected, excluded, isCount, err = parseSelectList(p)
		if err != nil {
			return fail(err)
		}
	}

	if p.peek().kind == tokFrom {
		p.advance()
		t := p.peek()
		if t.kind != tokSource {
			return fail(fmt.Errorf("expected source after FROM at offset %d, got %q", t.pos, t.text))
		}
		p.advance()
		source = t.text
	}

	if p.peek().kind == tokWhere {
		p.advance()
		whereStart := p.peek().pos
		pred, err = p.parseOr()
		if err != nil {
			return fail(err)
		}
		whereRaw = strings.TrimRight(src[whereStart:p.peek().pos], " \t\n\r")
	}

	if p.peek().kind == tokOrder {
		p.advance()
		t := p.advance()
		if t.kind != tokBy {
			return fail(fmt.Errorf("expected BY after ORDER at offset %d, got %q", t.pos, t.text))
		}
		orderBy, err = parseOrderBy(p)
		if err != nil {
			return fail(err)
		}
	}

	if p.peek().kind == tokLimit {
		p.advance()
		n, perr := parseNonNegativeInt(p, "LIMIT")
		if perr != nil {
			return fail(perr)
		}
		limit = n
	}

	if p.peek().kind == tokOffset {
		p.advance()
		n, perr := parseNonNegativeInt(p, "OFFSET")
		if perr != nil {
			return fail(perr)
		}
		offset = n
	}

	if p.peek().kind == tokWith {
		p.advance()
		with, err = parseWith(p)
		if err != nil {
			return fail(err)
		}
	}

	if t := p.peek(); t.kind != tokEOF {
		return fail(fmt.Errorf("unexpected token %q at offset %d", t.text, t.pos))
	}
	return selected, excluded, source, pred, orderBy, limit, offset, with, whereRaw, isCount, nil
}

// parseNonNegativeInt consumes the next token, expecting a non-negative
// integer literal. The clauseName is used in error messages.
func parseNonNegativeInt(p *parseState, clauseName string) (int, error) {
	t := p.advance()
	if t.kind != tokNumber {
		return 0, fmt.Errorf("expected number after %s at offset %d, got %q", clauseName, t.pos, t.text)
	}
	n, err := strconv.ParseInt(t.text, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer, got %q at offset %d", clauseName, t.text, t.pos)
	}
	return int(n), nil
}

// parseWith reads a comma-separated list of `key = string` pairs after the
// WITH keyword. Recognized keys: `prefix` (path glob for nested rows) and
// `provider` (e.g. `external:./script.py` for user-supplied row sources).
// Unknown or duplicate keys error. Empty fields in the returned WithOptions
// mean the user did not set that key.
func parseWith(p *parseState) (opts WithOptions, err error) {
	seen := map[string]bool{}
	for {
		k := p.advance()
		if k.kind != tokIdent {
			return WithOptions{}, fmt.Errorf("expected key in WITH at offset %d, got %q", k.pos, k.text)
		}
		eq := p.advance()
		if eq.kind != tokEq {
			return WithOptions{}, fmt.Errorf("expected '=' after WITH key at offset %d, got %q", eq.pos, eq.text)
		}
		v := p.advance()
		if v.kind != tokString {
			return WithOptions{}, fmt.Errorf("expected string value in WITH at offset %d, got %q", v.pos, v.text)
		}
		key := strings.ToLower(k.text)
		if seen[key] {
			return WithOptions{}, fmt.Errorf("duplicate WITH key %q at offset %d", k.text, k.pos)
		}
		switch key {
		case "prefix":
			opts.Prefix = v.text
		case "provider":
			opts.Provider = v.text
		default:
			return WithOptions{}, fmt.Errorf("unknown WITH key %q at offset %d", k.text, k.pos)
		}
		seen[key] = true
		if p.peek().kind != tokComma {
			break
		}
		p.advance()
	}
	return opts, nil
}

func parseOrderBy(p *parseState) ([]OrderTerm, error) {
	var terms []OrderTerm
	for {
		t := p.advance()
		if t.kind != tokIdent {
			return nil, fmt.Errorf("expected column name in ORDER BY at offset %d, got %q", t.pos, t.text)
		}
		term := OrderTerm{Col: t.text}
		switch p.peek().kind {
		case tokAsc:
			p.advance()
		case tokDesc:
			p.advance()
			term.Desc = true
		}
		terms = append(terms, term)
		if p.peek().kind != tokComma {
			break
		}
		p.advance()
	}
	return terms, nil
}

func parseSelectList(p *parseState) (cols []string, excluded []string, isCount bool, err error) {
	if p.peek().kind == tokStar {
		p.advance()
		if p.peek().kind == tokExclude {
			p.advance()
			excluded, err = parseExcludeList(p)
			if err != nil {
				return nil, nil, false, err
			}
		}
		return nil, excluded, false, nil
	}
	// COUNT is recognized lexically only when followed by `(` so that "count"
	// remains usable as a regular column name.
	if t := p.peek(); t.kind == tokIdent && strings.EqualFold(t.text, "count") && p.toks[p.pos+1].kind == tokLParen {
		p.advance() // count
		p.advance() // (
		star := p.advance()
		if star.kind != tokStar {
			return nil, nil, false, fmt.Errorf("expected * inside COUNT() at offset %d, got %q (only COUNT(*) is supported)", star.pos, star.text)
		}
		rp := p.advance()
		if rp.kind != tokRParen {
			return nil, nil, false, fmt.Errorf("expected ')' to close COUNT(*) at offset %d, got %q", rp.pos, rp.text)
		}
		if p.peek().kind == tokExclude {
			return nil, nil, false, fmt.Errorf("EXCLUDE is not allowed with COUNT(*) at offset %d", p.peek().pos)
		}
		return nil, nil, true, nil
	}
	for {
		t := p.advance()
		if t.kind != tokIdent {
			return nil, nil, false, fmt.Errorf("expected column name at offset %d, got %q", t.pos, t.text)
		}
		cols = append(cols, t.text)
		if p.peek().kind != tokComma {
			break
		}
		p.advance()
	}
	if p.peek().kind == tokExclude {
		return nil, nil, false, fmt.Errorf("EXCLUDE is only allowed after SELECT * at offset %d", p.peek().pos)
	}
	return cols, nil, false, nil
}

// parseExcludeList consumes `(col1, col2, ...)` after the EXCLUDE keyword. The
// list must be non-empty; identifiers may carry dot paths just like SELECT
// projections.
func parseExcludeList(p *parseState) ([]string, error) {
	if t := p.peek(); t.kind != tokLParen {
		return nil, fmt.Errorf("expected '(' after EXCLUDE at offset %d, got %q", t.pos, t.text)
	}
	p.advance()
	var cols []string
	for {
		t := p.advance()
		if t.kind != tokIdent {
			return nil, fmt.Errorf("expected column name in EXCLUDE list at offset %d, got %q", t.pos, t.text)
		}
		cols = append(cols, t.text)
		if p.peek().kind != tokComma {
			break
		}
		p.advance()
	}
	if t := p.peek(); t.kind != tokRParen {
		return nil, fmt.Errorf("expected ')' to close EXCLUDE list at offset %d, got %q", t.pos, t.text)
	}
	p.advance()
	return cols, nil
}

func ParseWhere(src string) (WhereExpr, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("empty where expression")
	}
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	p := &parseState{toks: toks}
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

func (p *parseState) parseOr() (WhereExpr, error) {
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

func (p *parseState) parseAnd() (WhereExpr, error) {
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

func (p *parseState) parseNot() (WhereExpr, error) {
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

func (p *parseState) parsePrimary() (WhereExpr, error) {
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

func (p *parseState) parseComparison() (WhereExpr, error) {
	left, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	if p.peek().kind == tokMatches || (p.peek().kind == tokNot && p.pos+1 < len(p.toks) && p.toks[p.pos+1].kind == tokMatches) {
		negated := p.peek().kind == tokNot
		if negated {
			p.advance()
		}
		p.advance()
		pat := p.advance()
		if pat.kind != tokString {
			return nil, fmt.Errorf("expected string regex after MATCHES at offset %d, got %q", pat.pos, pat.text)
		}
		re, err := regexp.Compile(pat.text)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q at offset %d: %v", pat.text, pat.pos, err)
		}
		var expr WhereExpr = &matchesExpr{left: left, re: re}
		if negated {
			expr = &notExpr{inner: expr}
		}
		return expr, nil
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

func (p *parseState) parseOperand() (operand, error) {
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
