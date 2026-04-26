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
		name string
		expr string
		want []string
	}{
		{"eq num", "age = 30", []string{"alice"}},
		{"neq num", "age != 30", []string{"bob", "carol"}},
		{"lt num", "age < 30", []string{"bob"}},
		{"lte num", "age <= 30", []string{"alice", "bob"}},
		{"gt num", "age > 30", []string{"carol"}},
		{"gte num", "age >= 30", []string{"alice", "carol"}},
		{"float compare", "score > 95", []string{"alice"}},

		{"eq str", `address.city = "SF"`, []string{"alice"}},
		{"single-quoted str", `address.city = 'SF'`, []string{"alice"}},
		{"neq str", `address.city != "SF"`, []string{"bob", "carol"}},
		{"lt str", `address.city < "M"`, []string{"carol"}},

		{"key eq", `key = "alice"`, []string{"alice"}},

		{"bool true", "active = true", []string{"alice", "carol"}},
		{"bool false", "active = false", []string{"bob"}},
		{"bool neq", "active != true", []string{"bob"}},

		{"dot path index", `tags.0 = "eng"`, []string{"alice"}},

		{"missing col vs val", "tags.0 = 1", nil},
		{"missing col != val", `tags.0 != "eng"`, nil},
		{"missing col is null", "tags.0 = null", []string{"bob", "carol"}},
		{"missing col not null", "tags.0 != null", []string{"alice"}},
		{"explicit null is null", "middle_name = null", []string{"alice", "bob", "carol"}},
		{"NOT eq null on missing", "NOT (tags.0 = 5)", []string{"alice", "bob", "carol"}},

		{"type mismatch num/str", `age = "thirty"`, nil},
		{"type mismatch str/num", `address.city = 5`, nil},
		{"type mismatch str/bool", `address.city = true`, nil},

		{"AND", `age >= 30 AND active = true`, []string{"alice", "carol"}},
		{"OR", `age = 25 OR age = 41`, []string{"bob", "carol"}},
		{"AND-OR precedence", `age = 25 OR age >= 30 AND active = true`, []string{"alice", "bob", "carol"}},
		{"parens override precedence", `(age = 25 OR age >= 30) AND active = true`, []string{"alice", "carol"}},
		{"NOT NOT", "NOT NOT age = 30", []string{"alice"}},
		{"NOT around AND", "NOT (active = true AND age >= 30)", []string{"bob"}},
		{"case-insensitive keywords", "age = 30 AnD active = TRUE", []string{"alice"}},

		{"literal on left", "30 = age", []string{"alice"}},
		{"literal both sides true", "1 = 1", []string{"alice", "bob", "carol"}},
		{"literal both sides false", "1 = 2", nil},

		{"null = null", "null = null", []string{"alice", "bob", "carol"}},
		{"null = 5", "null = 5", nil},
		{"null != 5", "null != 5", []string{"alice", "bob", "carol"}},
		{"null < 5", "null < 5", nil},

		{"matches simple", `key MATCHES 'ali'`, []string{"alice"}},
		{"matches anchored", `key MATCHES '^a'`, []string{"alice"}},
		{"matches escaped dot", `address.city MATCHES '^N.*'`, []string{"bob"}},
		{"matches lowercase keyword", `key matches 'b.b'`, []string{"bob"}},
		{"matches no result", `key MATCHES 'zzz'`, nil},
		{"matches on number coerced", `age MATCHES '^4'`, []string{"carol"}},
		{"matches on missing col", `tags.0 MATCHES '.*'`, []string{"alice"}},
		{"NOT matches", `NOT key MATCHES '^a'`, []string{"bob", "carol"}},
		{"matches AND eq", `key MATCHES '^[ab]' AND active = true`, []string{"alice"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := ParseWhere(c.expr)
			if err != nil {
				t.Fatalf("parse %q: %v", c.expr, err)
			}
			var got []string
			for _, r := range rows {
				if expr.Eval(r) {
					got = append(got, r["key"].(string))
				}
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
		expr string
		want bool
	}{
		{"v = 42", true},
		{"v != 42", false},
		{"v < 50", true},
		{"v > 41", true},
		{"v >= 42", true},
		{"v <= 42", true},
		{"v = 43", false},
		{`v = "42"`, false},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			expr, err := ParseWhere(c.expr)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := expr.Eval(r); got != c.want {
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
