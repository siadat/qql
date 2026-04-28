package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

type row = map[string]any

type WhereExpr interface {
	Eval(r row) (bool, error)
	cols(out map[string]struct{})
}

// ReferencedCols returns the column identifiers used anywhere in the
// predicate, sorted ascending. Callers use it to surface a clear error when
// the user types `WHERE staus = 'up'` against rows that only have a `status`
// column, instead of silently filtering everything away.
func ReferencedCols(e WhereExpr) []string {
	out := map[string]struct{}{}
	e.cols(out)
	cols := make([]string, 0, len(out))
	for k := range out {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

type orExpr struct{ left, right WhereExpr }

// Eval short-circuits on a true left operand but always propagates evaluation
// errors (so a type-mismatch in either branch surfaces even when the other
// branch could short-circuit it away).
func (e *orExpr) Eval(r row) (bool, error) {
	l, err := e.left.Eval(r)
	if err != nil {
		return false, err
	}
	if l {
		return true, nil
	}
	return e.right.Eval(r)
}
func (e *orExpr) cols(out map[string]struct{}) { e.left.cols(out); e.right.cols(out) }

type andExpr struct{ left, right WhereExpr }

// Eval short-circuits on a false left operand but always propagates evaluation
// errors.
func (e *andExpr) Eval(r row) (bool, error) {
	l, err := e.left.Eval(r)
	if err != nil {
		return false, err
	}
	if !l {
		return false, nil
	}
	return e.right.Eval(r)
}
func (e *andExpr) cols(out map[string]struct{}) { e.left.cols(out); e.right.cols(out) }

type notExpr struct{ inner WhereExpr }

func (e *notExpr) Eval(r row) (bool, error) {
	v, err := e.inner.Eval(r)
	if err != nil {
		return false, err
	}
	return !v, nil
}
func (e *notExpr) cols(out map[string]struct{}) { e.inner.cols(out) }

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
	cols(out map[string]struct{})
}

type identOperand struct{ name string }

func (o *identOperand) resolve(r row) any            { return r[o.name] }
func (o *identOperand) cols(out map[string]struct{}) { out[o.name] = struct{}{} }

type numLit struct{ v float64 }

func (o *numLit) resolve(r row) any          { return o.v }
func (o *numLit) cols(_ map[string]struct{}) {}

type strLit struct{ v string }

func (o *strLit) resolve(r row) any          { return o.v }
func (o *strLit) cols(_ map[string]struct{}) {}

type boolLit struct{ v bool }

func (o *boolLit) resolve(r row) any          { return o.v }
func (o *boolLit) cols(_ map[string]struct{}) {}

type nullLit struct{}

func (o *nullLit) resolve(r row) any          { return nil }
func (o *nullLit) cols(_ map[string]struct{}) {}

type matchesExpr struct {
	left operand
	re   *regexp.Regexp
}

// Eval coerces non-string values via fmt.Sprint so MATCHES is useful on
// numbers/bools too (e.g. `size MATCHES '^4\d+$'`). nil short-circuits to false
// rather than matching against the literal "<nil>". MATCHES never raises a
// type-mismatch error since the regex side has no type to compare against.
func (e *matchesExpr) Eval(r row) (bool, error) {
	v := e.left.resolve(r)
	if v == nil {
		return false, nil
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprint(v)
	}
	return e.re.MatchString(s), nil
}

func (e *matchesExpr) cols(out map[string]struct{}) { e.left.cols(out) }

type cmpExpr struct {
	op          cmpOp
	left, right operand
}

// Eval compares the two operands. `null` literals follow SQL-like nil
// semantics. Genuine missing values (nil from r[col]) propagate as false to
// keep the common "row lacks this column" case quiet.
//
// When both operands are non-nil but of different scalar types (e.g. number
// vs string), Eval returns a type-mismatch error rather than silently treating
// the comparison as false. Otherwise a typo like `WHERE col2 != 1` against
// string-valued data would wipe the whole result set with no explanation.
func (e *cmpExpr) Eval(r row) (bool, error) {
	_, lNull := e.left.(*nullLit)
	_, rNull := e.right.(*nullLit)
	lv := e.left.resolve(r)
	rv := e.right.resolve(r)

	if lNull || rNull {
		switch e.op {
		case opEq:
			return lv == nil && rv == nil, nil
		case opNeq:
			return !(lv == nil && rv == nil), nil
		default:
			return false, nil
		}
	}

	if lv == nil || rv == nil {
		return false, nil
	}

	if lf, ok := toFloat(lv); ok {
		if rf, ok := toFloat(rv); ok {
			return cmpFloat(e.op, lf, rf), nil
		}
		return false, typeMismatchError(lv, rv)
	}
	if ls, ok := lv.(string); ok {
		if rs, ok := rv.(string); ok {
			return cmpString(e.op, ls, rs), nil
		}
		return false, typeMismatchError(lv, rv)
	}
	if lb, ok := lv.(bool); ok {
		if rb, ok := rv.(bool); ok {
			return cmpBool(e.op, lb, rb), nil
		}
		return false, typeMismatchError(lv, rv)
	}
	return false, typeMismatchError(lv, rv)
}

func (e *cmpExpr) cols(out map[string]struct{}) { e.left.cols(out); e.right.cols(out) }

func typeMismatchError(lv, rv any) error {
	return fmt.Errorf("type mismatch in WHERE: cannot compare %s with %s",
		formatTypedValue(lv), formatTypedValue(rv))
}

// formatTypedValue renders a value with its runtime type tag for the
// type-mismatch error, e.g. `"row1col2" (string)` or `1 (number)`.
func formatTypedValue(v any) string {
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("%q (string)", x)
	case bool:
		return fmt.Sprintf("%v (bool)", x)
	case json.Number:
		return fmt.Sprintf("%s (number)", x.String())
	case float64, float32, int, int64, uint64:
		return fmt.Sprintf("%v (number)", x)
	default:
		return fmt.Sprintf("%v", x)
	}
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
