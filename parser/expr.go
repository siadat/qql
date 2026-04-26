package parser

import (
	"encoding/json"
)

type row = map[string]any

type WhereExpr interface {
	Eval(r row) bool
}

type orExpr struct{ left, right WhereExpr }

func (e *orExpr) Eval(r row) bool { return e.left.Eval(r) || e.right.Eval(r) }

type andExpr struct{ left, right WhereExpr }

func (e *andExpr) Eval(r row) bool { return e.left.Eval(r) && e.right.Eval(r) }

type notExpr struct{ inner WhereExpr }

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
	return r[o.name]
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

// CompareValues returns -1, 0, or 1 in a total order across qql's runtime types.
// Values of different types are ordered by type rank (nil < bool < number < string < other);
// within a rank, natural ordering applies. Unknown types tie so stable sort preserves input order.
func CompareValues(a, b any) int {
	ra, rb := typeRank(a), typeRank(b)
	if ra != rb {
		if ra < rb {
			return -1
		}
		return 1
	}
	switch ra {
	case rankNil:
		return 0
	case rankBool:
		ab, bb := a.(bool), b.(bool)
		switch {
		case ab == bb:
			return 0
		case !ab:
			return -1
		default:
			return 1
		}
	case rankNumber:
		af, _ := toFloat(a)
		bf, _ := toFloat(b)
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	case rankString:
		as, bs := a.(string), b.(string)
		switch {
		case as < bs:
			return -1
		case as > bs:
			return 1
		default:
			return 0
		}
	default:
		return 0
	}
}

const (
	rankNil = iota
	rankBool
	rankNumber
	rankString
	rankOther
)

func typeRank(v any) int {
	if v == nil {
		return rankNil
	}
	switch v.(type) {
	case bool:
		return rankBool
	case json.Number, float64, float32, int, int64, uint64:
		return rankNumber
	case string:
		return rankString
	default:
		return rankOther
	}
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
