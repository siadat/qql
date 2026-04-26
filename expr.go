package main

import (
	"encoding/json"
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
