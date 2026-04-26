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
	}{
		{"empty", "", nil, "", false},
		{"select cols", "SELECT a, b, c", []string{"a", "b", "c"}, "", false},
		{"select star", "SELECT *", nil, "", false},
		{"select dot path", "SELECT address.city, tags.0", []string{"address.city", "tags.0"}, "", false},
		{"where only", "WHERE age = 30", nil, "", true},
		{"select and where", "SELECT a WHERE b = 1", []string{"a"}, "", true},
		{"star and where", "SELECT * WHERE b = 1", nil, "", true},
		{"lowercase keywords", "select a where b = 1", []string{"a"}, "", true},
		{"mixed case keywords", "Select a Where b = 1", []string{"a"}, "", true},
		{"from only", "FROM git:.", nil, "git:.", false},
		{"select from", "SELECT * FROM git:.", nil, "git:.", false},
		{"select from where", "SELECT * FROM git:. WHERE author = 'Sina'", nil, "git:.", true},
		{"from path", "SELECT a FROM /tmp/foo.json WHERE x = 1", []string{"a"}, "/tmp/foo.json", true},
		{"from with colon", "FROM git:/home/me/repo", nil, "git:/home/me/repo", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, source, pred, err := parseSQL(c.src)
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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, err := parseSQL(c.src)
			if err == nil {
				t.Errorf("expected error for %q", c.src)
			}
		})
	}
}

func TestParseSQLEvalIntegration(t *testing.T) {
	rows := []row{
		{id: "alice", cols: map[string]any{"age": 30, "city": "SF"}},
		{id: "bob", cols: map[string]any{"age": 25, "city": "NYC"}},
		{id: "carol", cols: map[string]any{"age": 41, "city": "LA"}},
	}

	_, _, pred, err := parseSQL("SELECT age WHERE age >= 30")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var got []string
	for _, r := range rows {
		if pred.Eval(r) {
			got = append(got, r.id)
		}
	}
	if want := []string{"alice", "carol"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
