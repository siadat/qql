package main

import (
	"reflect"
	"testing"
)

func TestParseSQL(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantCols    []string
		wantSource  string
		wantHasPred bool
		wantOrderBy []orderTerm
	}{
		{"empty", "", nil, "", false, nil},
		{"select cols", "SELECT a, b, c", []string{"a", "b", "c"}, "", false, nil},
		{"select star", "SELECT *", nil, "", false, nil},
		{"select dot path", "SELECT address.city, tags.0", []string{"address.city", "tags.0"}, "", false, nil},
		{"where only", "WHERE age = 30", nil, "", true, nil},
		{"select and where", "SELECT a WHERE b = 1", []string{"a"}, "", true, nil},
		{"star and where", "SELECT * WHERE b = 1", nil, "", true, nil},
		{"lowercase keywords", "select a where b = 1", []string{"a"}, "", true, nil},
		{"mixed case keywords", "Select a Where b = 1", []string{"a"}, "", true, nil},
		{"from only", "FROM git:.", nil, "git:.", false, nil},
		{"select from", "SELECT * FROM git:.", nil, "git:.", false, nil},
		{"select from where", "SELECT * FROM git:. WHERE author = 'Sina'", nil, "git:.", true, nil},
		{"from path", "SELECT a FROM /tmp/foo.json WHERE x = 1", []string{"a"}, "/tmp/foo.json", true, nil},
		{"from with colon", "FROM git:/home/me/repo", nil, "git:/home/me/repo", false, nil},

		{"order by single asc default", "ORDER BY a", nil, "", false, []orderTerm{{col: "a"}}},
		{"order by explicit asc", "ORDER BY a ASC", nil, "", false, []orderTerm{{col: "a"}}},
		{"order by desc", "ORDER BY a DESC", nil, "", false, []orderTerm{{col: "a", desc: true}}},
		{"order by multi", "ORDER BY a, b DESC, c", nil, "", false, []orderTerm{{col: "a"}, {col: "b", desc: true}, {col: "c"}}},
		{"order by dot path", "ORDER BY user.age", nil, "", false, []orderTerm{{col: "user.age"}}},
		{"order by lowercase", "order by a desc", nil, "", false, []orderTerm{{col: "a", desc: true}}},
		{"select where order by", "SELECT a WHERE b = 1 ORDER BY a DESC", []string{"a"}, "", true, []orderTerm{{col: "a", desc: true}}},
		{"select from where order by", "SELECT * FROM git:. WHERE author = 'Sina' ORDER BY time DESC", nil, "git:.", true, []orderTerm{{col: "time", desc: true}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, source, pred, orderBy, err := parseSQL(c.src)
			if err != nil {
				t.Fatalf("parse %q: %v", c.src, err)
			}
			if !reflect.DeepEqual(cols, c.wantCols) {
				t.Errorf("cols: got %v, want %v", cols, c.wantCols)
			}
			if source != c.wantSource {
				t.Errorf("source: got %q, want %q", source, c.wantSource)
			}
			if (pred != nil) != c.wantHasPred {
				t.Errorf("pred: got %v, want hasPred=%v", pred, c.wantHasPred)
			}
			if !reflect.DeepEqual(orderBy, c.wantOrderBy) {
				t.Errorf("orderBy: got %v, want %v", orderBy, c.wantOrderBy)
			}
		})
	}
}

func TestParseWhereErrors(t *testing.T) {
	cases := []struct {
		name string
		expr string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"trailing junk", "a = 1 foo"},
		{"unbalanced paren", "(a = 1"},
		{"bare bang", "a !== 1"},
		{"unterminated string", `a = "abc`},
		{"missing operator", "a 1"},
		{"missing rhs", "a ="},
		{"unexpected char", "a = @"},
		{"empty parens", "()"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseWhere(c.expr)
			if err == nil {
				t.Errorf("expected error for %q", c.expr)
			}
		})
	}
}

func TestParseSQLErrors(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"trailing junk", "SELECT a, b foo"},
		{"select missing column", "SELECT WHERE b = 1"},
		{"select trailing comma", "SELECT a,"},
		{"where without expr", "WHERE"},
		{"unbalanced paren in where", "WHERE (a = 1"},
		{"junk after where expr", "SELECT a WHERE b = 1 garbage"},
		{"select after where", "WHERE a = 1 SELECT b"},
		{"from without source", "SELECT * FROM"},
		{"from after where", "WHERE a = 1 FROM x"},
		{"order alone", "ORDER"},
		{"order without by", "ORDER a"},
		{"order by without col", "ORDER BY"},
		{"order by trailing comma", "ORDER BY a,"},
		{"order by literal", "ORDER BY 1"},
		{"order by double direction", "ORDER BY a ASC DESC"},
		{"select after order by", "ORDER BY a SELECT b"},
		{"where after order by", "ORDER BY a WHERE b = 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, _, err := parseSQL(c.src)
			if err == nil {
				t.Errorf("expected error for %q", c.src)
			}
		})
	}
}

func TestParseSQLEvalIntegration(t *testing.T) {
	rows := []row{
		{"id": "alice", "age": 30, "city": "SF"},
		{"id": "bob", "age": 25, "city": "NYC"},
		{"id": "carol", "age": 41, "city": "LA"},
	}

	_, _, pred, _, err := parseSQL("SELECT age WHERE age >= 30")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var got []string
	for _, r := range rows {
		if pred.Eval(r) {
			got = append(got, r["id"].(string))
		}
	}
	if want := []string{"alice", "carol"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
