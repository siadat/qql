package parser

import (
	"reflect"
	"testing"
)

func TestParseSQL(t *testing.T) {
	cases := []struct {
		name         string
		src          string
		wantCols     []string
		wantSource   string
		wantHasPred  bool
		wantOrderBy  []OrderTerm
		wantPrefix string
	}{
		{"empty", "", nil, "", false, nil, ""},
		{"select cols", "SELECT a, b, c", []string{"a", "b", "c"}, "", false, nil, ""},
		{"select star", "SELECT *", nil, "", false, nil, ""},
		{"select dot path", "SELECT address.city, tags.0", []string{"address.city", "tags.0"}, "", false, nil, ""},
		{"where only", "WHERE age = 30", nil, "", true, nil, ""},
		{"select and where", "SELECT a WHERE b = 1", []string{"a"}, "", true, nil, ""},
		{"star and where", "SELECT * WHERE b = 1", nil, "", true, nil, ""},
		{"lowercase keywords", "select a where b = 1", []string{"a"}, "", true, nil, ""},
		{"mixed case keywords", "Select a Where b = 1", []string{"a"}, "", true, nil, ""},
		{"from only", "FROM git:.", nil, "git:.", false, nil, ""},
		{"select from", "SELECT * FROM git:.", nil, "git:.", false, nil, ""},
		{"select from where", "SELECT * FROM git:. WHERE author = 'Sina'", nil, "git:.", true, nil, ""},
		{"from path", "SELECT a FROM /tmp/foo.json WHERE x = 1", []string{"a"}, "/tmp/foo.json", true, nil, ""},
		{"from with colon", "FROM git:/home/me/repo", nil, "git:/home/me/repo", false, nil, ""},

		{"order by single asc default", "ORDER BY a", nil, "", false, []OrderTerm{{Col: "a"}}, ""},
		{"order by explicit asc", "ORDER BY a ASC", nil, "", false, []OrderTerm{{Col: "a"}}, ""},
		{"order by desc", "ORDER BY a DESC", nil, "", false, []OrderTerm{{Col: "a", Desc: true}}, ""},
		{"order by multi", "ORDER BY a, b DESC, c", nil, "", false, []OrderTerm{{Col: "a"}, {Col: "b", Desc: true}, {Col: "c"}}, ""},
		{"order by dot path", "ORDER BY user.age", nil, "", false, []OrderTerm{{Col: "user.age"}}, ""},
		{"order by lowercase", "order by a desc", nil, "", false, []OrderTerm{{Col: "a", Desc: true}}, ""},
		{"select where order by", "SELECT a WHERE b = 1 ORDER BY a DESC", []string{"a"}, "", true, []OrderTerm{{Col: "a", Desc: true}}, ""},
		{"select from where order by", "SELECT * FROM git:. WHERE author = 'Sina' ORDER BY time DESC", nil, "git:.", true, []OrderTerm{{Col: "time", Desc: true}}, ""},

		{"with prefix star", "WITH prefix = '*'", nil, "", false, nil, "*"},
		{"with prefix nested", "WITH prefix = '*.servers.*'", nil, "", false, nil, "*.servers.*"},
		{"with prefix literal", "WITH prefix = 'region-a.servers'", nil, "", false, nil, "region-a.servers"},
		{"with prefix lowercase", "with prefix = '*.servers.*'", nil, "", false, nil, "*.servers.*"},
		{"with prefix double quoted", `WITH prefix = "*.servers.*"`, nil, "", false, nil, "*.servers.*"},
		{"select then with", "SELECT key, cpu WITH prefix = '*.servers.*'", []string{"key", "cpu"}, "", false, nil, "*.servers.*"},
		{"star select then with", "SELECT * WITH prefix = '*.servers.*'", nil, "", false, nil, "*.servers.*"},
		{"full query with at end", "SELECT * FROM testdata/regions.yaml WHERE cpu >= 8 ORDER BY ram DESC WITH prefix = '*.servers.*'", nil, "testdata/regions.yaml", true, []OrderTerm{{Col: "ram", Desc: true}}, "*.servers.*"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, source, pred, orderBy, prefix, err := ParseSQL(c.src)
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
			if prefix != c.wantPrefix {
				t.Errorf("prefix: got %q, want %q", prefix, c.wantPrefix)
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
			_, err := ParseWhere(c.expr)
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
		{"with alone", "WITH"},
		{"with key only", "WITH prefix"},
		{"with no value", "WITH prefix ="},
		{"with non-string value", "WITH prefix = 5"},
		{"with unknown key", "WITH foo = 'x'"},
		{"with duplicate prefix", "WITH prefix = 'x', prefix = 'y'"},
		{"with trailing comma", "WITH prefix = 'x',"},
		{"with before select", "WITH prefix = 'x' SELECT a"},
		{"with before order by", "WITH prefix = 'x' ORDER BY a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, _, _, err := ParseSQL(c.src)
			if err == nil {
				t.Errorf("expected error for %q", c.src)
			}
		})
	}
}

func TestParseSQLEvalIntegration(t *testing.T) {
	rows := []row{
		{"key": "alice", "age": 30, "city": "SF"},
		{"key": "bob", "age": 25, "city": "NYC"},
		{"key": "carol", "age": 41, "city": "LA"},
	}

	_, _, pred, _, _, err := ParseSQL("SELECT age WHERE age >= 30")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var got []string
	for _, r := range rows {
		if pred.Eval(r) {
			got = append(got, r["key"].(string))
		}
	}
	if want := []string{"alice", "carol"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
