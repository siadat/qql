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
		wantLimit    int
		wantWith     WithOptions
		wantWhereRaw string
	}{
		{"empty", "", nil, "", false, nil, -1, WithOptions{}, ""},
		{"select cols", "SELECT a, b, c", []string{"a", "b", "c"}, "", false, nil, -1, WithOptions{}, ""},
		{"select star", "SELECT *", nil, "", false, nil, -1, WithOptions{}, ""},
		{"select dot path", "SELECT address.city, tags.0", []string{"address.city", "tags.0"}, "", false, nil, -1, WithOptions{}, ""},
		{"where only", "WHERE age = 30", nil, "", true, nil, -1, WithOptions{}, "age = 30"},
		{"select and where", "SELECT a WHERE b = 1", []string{"a"}, "", true, nil, -1, WithOptions{}, "b = 1"},
		{"star and where", "SELECT * WHERE b = 1", nil, "", true, nil, -1, WithOptions{}, "b = 1"},
		{"lowercase keywords", "select a where b = 1", []string{"a"}, "", true, nil, -1, WithOptions{}, "b = 1"},
		{"mixed case keywords", "Select a Where b = 1", []string{"a"}, "", true, nil, -1, WithOptions{}, "b = 1"},
		{"from only", "FROM git:.", nil, "git:.", false, nil, -1, WithOptions{}, ""},
		{"select from", "SELECT * FROM git:.", nil, "git:.", false, nil, -1, WithOptions{}, ""},
		{"select from where", "SELECT * FROM git:. WHERE author = 'Sina'", nil, "git:.", true, nil, -1, WithOptions{}, "author = 'Sina'"},
		{"from path", "SELECT a FROM /tmp/foo.json WHERE x = 1", []string{"a"}, "/tmp/foo.json", true, nil, -1, WithOptions{}, "x = 1"},
		{"from with colon", "FROM git:/home/me/repo", nil, "git:/home/me/repo", false, nil, -1, WithOptions{}, ""},

		{"order by single asc default", "ORDER BY a", nil, "", false, []OrderTerm{{Col: "a"}}, -1, WithOptions{}, ""},
		{"order by explicit asc", "ORDER BY a ASC", nil, "", false, []OrderTerm{{Col: "a"}}, -1, WithOptions{}, ""},
		{"order by desc", "ORDER BY a DESC", nil, "", false, []OrderTerm{{Col: "a", Desc: true}}, -1, WithOptions{}, ""},
		{"order by multi", "ORDER BY a, b DESC, c", nil, "", false, []OrderTerm{{Col: "a"}, {Col: "b", Desc: true}, {Col: "c"}}, -1, WithOptions{}, ""},
		{"order by dot path", "ORDER BY user.age", nil, "", false, []OrderTerm{{Col: "user.age"}}, -1, WithOptions{}, ""},
		{"order by lowercase", "order by a desc", nil, "", false, []OrderTerm{{Col: "a", Desc: true}}, -1, WithOptions{}, ""},
		{"select where order by", "SELECT a WHERE b = 1 ORDER BY a DESC", []string{"a"}, "", true, []OrderTerm{{Col: "a", Desc: true}}, -1, WithOptions{}, "b = 1"},
		{"select from where order by", "SELECT * FROM git:. WHERE author = 'Sina' ORDER BY time DESC", nil, "git:.", true, []OrderTerm{{Col: "time", Desc: true}}, -1, WithOptions{}, "author = 'Sina'"},

		{"limit only", "LIMIT 10", nil, "", false, nil, 10, WithOptions{}, ""},
		{"limit zero", "LIMIT 0", nil, "", false, nil, 0, WithOptions{}, ""},
		{"limit lowercase", "limit 5", nil, "", false, nil, 5, WithOptions{}, ""},
		{"limit after order by", "ORDER BY a LIMIT 3", nil, "", false, []OrderTerm{{Col: "a"}}, 3, WithOptions{}, ""},
		{"limit before with", "SELECT * LIMIT 7 WITH prefix = '*.servers.*'", nil, "", false, nil, 7, WithOptions{Prefix: "*.servers.*"}, ""},
		{"full query with limit", "SELECT key, cpu WHERE cpu >= 8 ORDER BY cpu DESC LIMIT 2 WITH prefix = '*.servers.*'", []string{"key", "cpu"}, "", true, []OrderTerm{{Col: "cpu", Desc: true}}, 2, WithOptions{Prefix: "*.servers.*"}, "cpu >= 8"},

		{"with prefix star", "WITH prefix = '*'", nil, "", false, nil, -1, WithOptions{Prefix: "*"}, ""},
		{"with prefix nested", "WITH prefix = '*.servers.*'", nil, "", false, nil, -1, WithOptions{Prefix: "*.servers.*"}, ""},
		{"with prefix literal", "WITH prefix = 'region-a.servers'", nil, "", false, nil, -1, WithOptions{Prefix: "region-a.servers"}, ""},
		{"with prefix lowercase", "with prefix = '*.servers.*'", nil, "", false, nil, -1, WithOptions{Prefix: "*.servers.*"}, ""},
		{"with prefix double quoted", `WITH prefix = "*.servers.*"`, nil, "", false, nil, -1, WithOptions{Prefix: "*.servers.*"}, ""},
		{"select then with", "SELECT key, cpu WITH prefix = '*.servers.*'", []string{"key", "cpu"}, "", false, nil, -1, WithOptions{Prefix: "*.servers.*"}, ""},
		{"star select then with", "SELECT * WITH prefix = '*.servers.*'", nil, "", false, nil, -1, WithOptions{Prefix: "*.servers.*"}, ""},
		{"full query with at end", "SELECT * FROM testdata/regions.yaml WHERE cpu >= 8 ORDER BY ram DESC WITH prefix = '*.servers.*'", nil, "testdata/regions.yaml", true, []OrderTerm{{Col: "ram", Desc: true}}, -1, WithOptions{Prefix: "*.servers.*"}, "cpu >= 8"},

		{"with provider", "WITH provider = 'external:./x.py'", nil, "", false, nil, -1, WithOptions{Provider: "external:./x.py"}, ""},
		{"with provider lowercase", "with provider = 'external:./x.py'", nil, "", false, nil, -1, WithOptions{Provider: "external:./x.py"}, ""},
		{"with provider then prefix", "WITH provider = 'external:./x.py', prefix = '*.servers.*'", nil, "", false, nil, -1, WithOptions{Prefix: "*.servers.*", Provider: "external:./x.py"}, ""},
		{"with prefix then provider", "WITH prefix = '*.servers.*', provider = 'external:./x.py'", nil, "", false, nil, -1, WithOptions{Prefix: "*.servers.*", Provider: "external:./x.py"}, ""},
		{"full query external provider", "SELECT key, size FROM /etc WHERE size > 1000 ORDER BY size DESC WITH provider = 'external:./fs.py'", []string{"key", "size"}, "/etc", true, []OrderTerm{{Col: "size", Desc: true}}, -1, WithOptions{Provider: "external:./fs.py"}, "size > 1000"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, source, pred, orderBy, limit, with, whereRaw, err := ParseSQL(c.src)
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
			if limit != c.wantLimit {
				t.Errorf("limit: got %d, want %d", limit, c.wantLimit)
			}
			if with != c.wantWith {
				t.Errorf("with: got %+v, want %+v", with, c.wantWith)
			}
			if whereRaw != c.wantWhereRaw {
				t.Errorf("whereRaw: got %q, want %q", whereRaw, c.wantWhereRaw)
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
		{"with duplicate provider", "WITH provider = 'external:./x', provider = 'external:./y'"},
		{"with provider non-string", "WITH provider = 5"},
		{"with trailing comma", "WITH prefix = 'x',"},
		{"with before select", "WITH prefix = 'x' SELECT a"},
		{"with before order by", "WITH prefix = 'x' ORDER BY a"},
		{"matches non-string rhs", "WHERE x MATCHES 5"},
		{"matches null rhs", "WHERE x MATCHES null"},
		{"matches invalid regex", "WHERE x MATCHES '['"},
		{"matches without rhs", "WHERE x MATCHES"},
		{"limit alone", "LIMIT"},
		{"limit non-number", "LIMIT abc"},
		{"limit float", "LIMIT 1.5"},
		{"limit string", "LIMIT '5'"},
		{"limit before order by", "LIMIT 5 ORDER BY a"},
		{"limit before where", "LIMIT 5 WHERE a = 1"},
		{"limit after with", "WITH prefix = 'x' LIMIT 5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, _, _, _, _, err := ParseSQL(c.src)
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

	_, _, pred, _, _, _, _, err := ParseSQL("SELECT age WHERE age >= 30")
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
