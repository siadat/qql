package parser

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestEval(t *testing.T) {
	rows := []row{
		{"key": "alice", "age": 30, "address.city": "SF", "tags.0": "eng", "active": true, "score": 95.5},
		{"key": "bob", "age": 25, "address.city": "NYC", "active": false},
		{"key": "carol", "age": 41, "address.city": "LA", "active": true, "middle_name": nil},
	}

	cases := []struct {
		name    string
		expr    string
		want    []string
		wantErr bool // true if Eval should report a type-mismatch on at least one row
	}{
		{"eq num", "age = 30", []string{"alice"}, false},
		{"neq num", "age != 30", []string{"bob", "carol"}, false},
		{"lt num", "age < 30", []string{"bob"}, false},
		{"lte num", "age <= 30", []string{"alice", "bob"}, false},
		{"gt num", "age > 30", []string{"carol"}, false},
		{"gte num", "age >= 30", []string{"alice", "carol"}, false},
		{"float compare", "score > 95", []string{"alice"}, false},

		{"eq str", `address.city = "SF"`, []string{"alice"}, false},
		{"single-quoted str", `address.city = 'SF'`, []string{"alice"}, false},
		{"neq str", `address.city != "SF"`, []string{"bob", "carol"}, false},
		{"lt str", `address.city < "M"`, []string{"carol"}, false},

		{"key eq", `key = "alice"`, []string{"alice"}, false},

		{"bool true", "active = true", []string{"alice", "carol"}, false},
		{"bool false", "active = false", []string{"bob"}, false},
		{"bool neq", "active != true", []string{"bob"}, false},

		{"dot path index", `tags.0 = "eng"`, []string{"alice"}, false},

		// alice has tags.0 = "eng" (string), so comparing to 1 (number)
		// surfaces a type-mismatch error. Rows where tags.0 is missing
		// would silently fall through, but alice is iterated first.
		{"missing col vs val", "tags.0 = 1", nil, true},
		// `col != val` returns true for nil rows (missing column is
		// not equal to a concrete value) and false for the matching
		// non-nil row.
		{"missing col != val", `tags.0 != "eng"`, []string{"bob", "carol"}, false},
		{"missing col is null", "tags.0 = null", []string{"bob", "carol"}, false},
		{"missing col not null", "tags.0 != null", []string{"alice"}, false},
		{"explicit null is null", "middle_name = null", []string{"alice", "bob", "carol"}, false},

		{"type mismatch num/str", `age = "thirty"`, nil, true},
		{"type mismatch str/num", `address.city = 5`, nil, true},
		{"type mismatch str/bool", `address.city = true`, nil, true},

		{"AND", `age >= 30 AND active = true`, []string{"alice", "carol"}, false},
		{"OR", `age = 25 OR age = 41`, []string{"bob", "carol"}, false},
		{"AND-OR precedence", `age = 25 OR age >= 30 AND active = true`, []string{"alice", "bob", "carol"}, false},
		{"parens override precedence", `(age = 25 OR age >= 30) AND active = true`, []string{"alice", "carol"}, false},
		{"NOT NOT", "NOT NOT age = 30", []string{"alice"}, false},
		{"NOT around AND", "NOT (active = true AND age >= 30)", []string{"bob"}, false},
		{"case-insensitive keywords", "age = 30 AnD active = TRUE", []string{"alice"}, false},

		{"literal on left", "30 = age", []string{"alice"}, false},
		{"literal both sides true", "1 = 1", []string{"alice", "bob", "carol"}, false},
		{"literal both sides false", "1 = 2", nil, false},

		{"null = null", "null = null", []string{"alice", "bob", "carol"}, false},
		{"null = 5", "null = 5", nil, false},
		{"null != 5", "null != 5", []string{"alice", "bob", "carol"}, false},
		{"null < 5", "null < 5", nil, false},

		{"matches simple", `key MATCHES 'ali'`, []string{"alice"}, false},
		{"matches anchored", `key MATCHES '^a'`, []string{"alice"}, false},
		{"matches escaped dot", `address.city MATCHES '^N.*'`, []string{"bob"}, false},
		{"matches lowercase keyword", `key matches 'b.b'`, []string{"bob"}, false},
		{"matches no result", `key MATCHES 'zzz'`, nil, false},
		{"matches on number coerced", `age MATCHES '^4'`, []string{"carol"}, false},
		{"matches on missing col", `tags.0 MATCHES '.*'`, []string{"alice"}, false},
		{"NOT matches", `NOT key MATCHES '^a'`, []string{"bob", "carol"}, false},
		{"NOT MATCHES infix", `key NOT MATCHES '^a'`, []string{"bob", "carol"}, false},
		{"not matches lowercase", `key not matches '^a'`, []string{"bob", "carol"}, false},
		{"NOT MATCHES no result", `key NOT MATCHES '.'`, nil, false},
		{"NOT MATCHES on missing col", `tags.0 NOT MATCHES '.*'`, []string{"bob", "carol"}, false},
		{"NOT MATCHES AND eq", `key NOT MATCHES '^a' AND active = false`, []string{"bob"}, false},
		{"matches AND eq", `key MATCHES '^[ab]' AND active = true`, []string{"alice"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := ParseWhere(c.expr)
			if err != nil {
				t.Fatalf("parse %q: %v", c.expr, err)
			}
			var got []string
			var evalErr error
			for _, r := range rows {
				ok, err := expr.Eval(r)
				if err != nil {
					evalErr = err
					break
				}
				if ok {
					got = append(got, r["key"].(string))
				}
			}
			if c.wantErr {
				if evalErr == nil {
					t.Errorf("expr %q: expected type-mismatch error, got rows %v", c.expr, got)
				}
				return
			}
			if evalErr != nil {
				t.Errorf("expr %q: unexpected error: %v", c.expr, evalErr)
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("expr %q: got %v, want %v", c.expr, got, c.want)
			}
		})
	}
}

func TestEvalJSONNumber(t *testing.T) {
	r := row{"v": json.Number("42")}

	cases := []struct {
		expr    string
		want    bool
		wantErr bool
	}{
		{"v = 42", true, false},
		{"v != 42", false, false},
		{"v < 50", true, false},
		{"v > 41", true, false},
		{"v >= 42", true, false},
		{"v <= 42", true, false},
		{"v = 43", false, false},
		// number vs string literal is a type mismatch.
		{`v = "42"`, false, true},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			expr, err := ParseWhere(c.expr)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, err := expr.Eval(r)
			if c.wantErr {
				if err == nil {
					t.Errorf("%q: expected type-mismatch error", c.expr)
				}
				return
			}
			if err != nil {
				t.Errorf("%q: unexpected error: %v", c.expr, err)
				return
			}
			if got != c.want {
				t.Errorf("%q: got %v want %v", c.expr, got, c.want)
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	cases := []struct {
		name string
		a, b any
		want int
	}{
		{"nil vs nil", nil, nil, 0},
		{"nil < bool", nil, false, -1},
		{"bool > nil", true, nil, 1},
		{"nil < number", nil, 0, -1},
		{"nil < string", nil, "", -1},

		{"false < true", false, true, -1},
		{"true > false", true, false, 1},
		{"true == true", true, true, 0},

		{"int < int", 1, 2, -1},
		{"int > int", 5, 3, 1},
		{"int == int", 7, 7, 0},
		{"float < float", 1.5, 2.0, -1},
		{"int vs float equal", 3, 3.0, 0},
		{"json.Number vs int", json.Number("42"), 42, 0},
		{"json.Number vs float", json.Number("4.5"), 4.5, 0},
		{"json.Number lt", json.Number("1"), json.Number("2"), -1},
		{"int64 vs int", int64(10), 5, 1},
		{"uint64 vs float", uint64(2), 3.0, -1},
		{"float32 vs float64", float32(1.5), 1.5, 0},

		{"string lex lt", "apple", "banana", -1},
		{"string lex eq", "x", "x", 0},
		{"string lex gt", "z", "a", 1},

		{"bool < number", true, 0, -1},
		{"number < string", 99, "1", -1},
		{"string > number", "a", 1, 1},

		{"slice ties with slice", []any{1}, []any{2}, 0},
		{"map ties with map", map[string]any{"a": 1}, map[string]any{"b": 2}, 0},
		{"slice > string", []any{1}, "z", 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CompareValues(c.a, c.b); got != c.want {
				t.Errorf("CompareValues(%v, %v) = %d, want %d", c.a, c.b, got, c.want)
			}
		})
	}
}
