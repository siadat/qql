package main

import "fmt"

func parseSQL(src string) (selected []string, pred whereExpr, err error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, nil, err
	}
	p := &parser{toks: toks}

	if p.peek().kind == tokSelect {
		p.advance()
		selected, err = parseSelectList(p)
		if err != nil {
			return nil, nil, err
		}
	}

	if p.peek().kind == tokWhere {
		p.advance()
		pred, err = p.parseOr()
		if err != nil {
			return nil, nil, err
		}
	}

	if t := p.peek(); t.kind != tokEOF {
		return nil, nil, fmt.Errorf("unexpected token %q at offset %d", t.text, t.pos)
	}
	return selected, pred, nil
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
