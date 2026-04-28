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
		wantOffset   int
		wantWith     WithOptions
		wantWhereRaw string
	}{
		{"empty", "", nil, "", false, nil, -1, 0, WithOptions{}, ""},
		{"select cols", "SELECT a, b, c", []string{"a", "b", "c"}, "", false, nil, -1, 0, WithOptions{}, ""},
		{"select star", "SELECT *", nil, "", false, nil, -1, 0, WithOptions{}, ""},
		{"select dot path", "SELECT address.city, tags.0", []string{"address.city", "tags.0"}, "", false, nil, -1, 0, WithOptions{}, ""},
		{"where only", "WHERE age = 30", nil, "", true, nil, -1, 0, WithOptions{}, "age = 30"},
		{"select and where", "SELECT a WHERE b = 1", []string{"a"}, "", true, nil, -1, 0, WithOptions{}, "b = 1"},
		{"star and where", "SELECT * WHERE b = 1", nil, "", true, nil, -1, 0, WithOptions{}, "b = 1"},
		{"lowercase keywords", "select a where b = 1", []string{"a"}, "", true, nil, -1, 0, WithOptions{}, "b = 1"},
		{"mixed case keywords", "Select a Where b = 1", []string{"a"}, "", true, nil, -1, 0, WithOptions{}, "b = 1"},
		{"from only", "FROM git:.", nil, "git:.", false, nil, -1, 0, WithOptions{}, ""},
		{"select from", "SELECT * FROM git:.", nil, "git:.", false, nil, -1, 0, WithOptions{}, ""},
		{"select from where", "SELECT * FROM git:. WHERE author = 'Sina'", nil, "git:.", true, nil, -1, 0, WithOptions{}, "author = 'Sina'"},
		{"from path", "SELECT a FROM /tmp/foo.json WHERE x = 1", []string{"a"}, "/tmp/foo.json", true, nil, -1, 0, WithOptions{}, "x = 1"},
		{"from with colon", "FROM git:/home/me/repo", nil, "git:/home/me/repo", false, nil, -1, 0, WithOptions{}, ""},

		{"order by single asc default", "ORDER BY a", nil, "", false, []OrderTerm{{Col: "a"}}, -1, 0, WithOptions{}, ""},
		{"order by explicit asc", "ORDER BY a ASC", nil, "", false, []OrderTerm{{Col: "a"}}, -1, 0, WithOptions{}, ""},
		{"order by desc", "ORDER BY a DESC", nil, "", false, []OrderTerm{{Col: "a", Desc: true}}, -1, 0, WithOptions{}, ""},
		{"order by multi", "ORDER BY a, b DESC, c", nil, "", false, []OrderTerm{{Col: "a"}, {Col: "b", Desc: true}, {Col: "c"}}, -1, 0, WithOptions{}, ""},
		{"order by dot path", "ORDER BY user.age", nil, "", false, []OrderTerm{{Col: "user.age"}}, -1, 0, WithOptions{}, ""},
		{"order by lowercase", "order by a desc", nil, "", false, []OrderTerm{{Col: "a", Desc: true}}, -1, 0, WithOptions{}, ""},
		{"select where order by", "SELECT a WHERE b = 1 ORDER BY a DESC", []string{"a"}, "", true, []OrderTerm{{Col: "a", Desc: true}}, -1, 0, WithOptions{}, "b = 1"},
		{"select from where order by", "SELECT * FROM git:. WHERE author = 'Sina' ORDER BY time DESC", nil, "git:.", true, []OrderTerm{{Col: "time", Desc: true}}, -1, 0, WithOptions{}, "author = 'Sina'"},

		{"limit only", "LIMIT 10", nil, "", false, nil, 10, 0, WithOptions{}, ""},
		{"limit zero", "LIMIT 0", nil, "", false, nil, 0, 0, WithOptions{}, ""},
		{"limit lowercase", "limit 5", nil, "", false, nil, 5, 0, WithOptions{}, ""},
		{"limit after order by", "ORDER BY a LIMIT 3", nil, "", false, []OrderTerm{{Col: "a"}}, 3, 0, WithOptions{}, ""},
		{"limit before with", "SELECT * LIMIT 7 WITH provider = 'external:./x.py'", nil, "", false, nil, 7, 0, WithOptions{Provider: "external:./x.py"}, ""},
		{"full query with limit", "SELECT key, cpu WHERE cpu >= 8 ORDER BY cpu DESC LIMIT 2 WITH provider = 'external:./x.py'", []string{"key", "cpu"}, "", true, []OrderTerm{{Col: "cpu", Desc: true}}, 2, 0, WithOptions{Provider: "external:./x.py"}, "cpu >= 8"},

		{"offset only", "OFFSET 5", nil, "", false, nil, -1, 5, WithOptions{}, ""},
		{"offset zero", "OFFSET 0", nil, "", false, nil, -1, 0, WithOptions{}, ""},
		{"offset lowercase", "offset 3", nil, "", false, nil, -1, 3, WithOptions{}, ""},
		{"limit and offset", "LIMIT 10 OFFSET 5", nil, "", false, nil, 10, 5, WithOptions{}, ""},
		{"order limit offset", "ORDER BY a LIMIT 2 OFFSET 1", nil, "", false, []OrderTerm{{Col: "a"}}, 2, 1, WithOptions{}, ""},
		{"offset before with", "SELECT * OFFSET 4 WITH provider = 'external:./x.py'", nil, "", false, nil, -1, 4, WithOptions{Provider: "external:./x.py"}, ""},
		{"full query with offset", "SELECT key, cpu WHERE cpu >= 8 ORDER BY cpu DESC LIMIT 2 OFFSET 1 WITH provider = 'external:./x.py'", []string{"key", "cpu"}, "", true, []OrderTerm{{Col: "cpu", Desc: true}}, 2, 1, WithOptions{Provider: "external:./x.py"}, "cpu >= 8"},

		{"with provider", "WITH provider = 'external:./x.py'", nil, "", false, nil, -1, 0, WithOptions{Provider: "external:./x.py"}, ""},
		{"with provider lowercase", "with provider = 'external:./x.py'", nil, "", false, nil, -1, 0, WithOptions{Provider: "external:./x.py"}, ""},
		{"with provider double quoted", `WITH provider = "external:./x.py"`, nil, "", false, nil, -1, 0, WithOptions{Provider: "external:./x.py"}, ""},
		{"select then with", "SELECT key, cpu WITH provider = 'external:./x.py'", []string{"key", "cpu"}, "", false, nil, -1, 0, WithOptions{Provider: "external:./x.py"}, ""},
		{"star select then with", "SELECT * WITH provider = 'external:./x.py'", nil, "", false, nil, -1, 0, WithOptions{Provider: "external:./x.py"}, ""},
		{"full query with at end", "SELECT * FROM testdata/regions.yaml WHERE cpu >= 8 ORDER BY ram DESC WITH provider = 'external:./x.py'", nil, "testdata/regions.yaml", true, []OrderTerm{{Col: "ram", Desc: true}}, -1, 0, WithOptions{Provider: "external:./x.py"}, "cpu >= 8"},
		{"full query external provider", "SELECT key, size FROM /etc WHERE size > 1000 ORDER BY size DESC WITH provider = 'external:./fs.py'", []string{"key", "size"}, "/etc", true, []OrderTerm{{Col: "size", Desc: true}}, -1, 0, WithOptions{Provider: "external:./fs.py"}, "size > 1000"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, _, source, pred, orderBy, limit, offset, with, whereRaw, isCount, err := ParseSQL(c.src)
			if err != nil {
				t.Fatalf("parse %q: %v", c.src, err)
			}
			if isCount {
				t.Errorf("isCount: got true, want false")
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
			if offset != c.wantOffset {
				t.Errorf("offset: got %d, want %d", offset, c.wantOffset)
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
		{"with key only", "WITH provider"},
		{"with no value", "WITH provider ="},
		{"with non-string value", "WITH provider = 5"},
		{"with unknown key", "WITH foo = 'x'"},
		{"with rejects removed prefix key", "WITH prefix = 'x'"},
		{"with duplicate provider", "WITH provider = 'external:./x', provider = 'external:./y'"},
		{"with provider non-string", "WITH provider = 5"},
		{"with trailing comma", "WITH provider = 'external:./x',"},
		{"with before select", "WITH provider = 'external:./x' SELECT a"},
		{"with before order by", "WITH provider = 'external:./x' ORDER BY a"},
		{"matches non-string rhs", "WHERE x MATCHES 5"},
		{"matches null rhs", "WHERE x MATCHES null"},
		{"matches invalid regex", "WHERE x MATCHES '['"},
		{"matches without rhs", "WHERE x MATCHES"},
		{"not matches non-string rhs", "WHERE x NOT MATCHES 5"},
		{"not matches invalid regex", "WHERE x NOT MATCHES '['"},
		{"not matches without rhs", "WHERE x NOT MATCHES"},
		{"limit alone", "LIMIT"},
		{"limit non-number", "LIMIT abc"},
		{"limit float", "LIMIT 1.5"},
		{"limit string", "LIMIT '5'"},
		{"limit before order by", "LIMIT 5 ORDER BY a"},
		{"limit before where", "LIMIT 5 WHERE a = 1"},
		{"limit after with", "WITH provider = 'external:./x' LIMIT 5"},
		{"count missing rparen", "SELECT COUNT(*"},
		{"count missing star", "SELECT COUNT()"},
		{"count of column", "SELECT COUNT(age)"},
		{"count distinct", "SELECT COUNT(DISTINCT x)"},
		{"count mixed with cols", "SELECT a, COUNT(*)"},
		{"count then col after comma", "SELECT COUNT(*), a"},
		{"offset alone", "OFFSET"},
		{"offset non-number", "OFFSET abc"},
		{"offset float", "OFFSET 1.5"},
		{"offset string", "OFFSET '5'"},
		{"offset before limit", "OFFSET 5 LIMIT 3"},
		{"offset before order by", "OFFSET 5 ORDER BY a"},
		{"offset after with", "WITH provider = 'external:./x' OFFSET 5"},
		{"exclude without parens", "SELECT * EXCLUDE a"},
		{"exclude empty list", "SELECT * EXCLUDE()"},
		{"exclude unclosed paren", "SELECT * EXCLUDE(a"},
		{"exclude trailing comma", "SELECT * EXCLUDE(a,)"},
		{"exclude non-ident", "SELECT * EXCLUDE(1)"},
		{"exclude after explicit list", "SELECT a EXCLUDE(b)"},
		{"exclude after count", "SELECT COUNT(*) EXCLUDE(a)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, _, _, _, _, _, _, _, err := ParseSQL(c.src)
			if err == nil {
				t.Errorf("expected error for %q", c.src)
			}
		})
	}
}

func TestParseSQLCount(t *testing.T) {
	cases := []struct {
		name         string
		src          string
		wantHasPred  bool
		wantWhereRaw string
		wantSource   string
	}{
		{"count alone", "SELECT COUNT(*)", false, "", ""},
		{"count lowercase", "select count(*)", false, "", ""},
		{"count with whitespace", "SELECT COUNT( * )", false, "", ""},
		{"count with from", "SELECT COUNT(*) FROM /tmp/foo.json", false, "", "/tmp/foo.json"},
		{"count with where", "SELECT COUNT(*) WHERE x = 1", true, "x = 1", ""},
		{"count with where and order", "SELECT COUNT(*) WHERE x = 1 ORDER BY y", true, "x = 1", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, _, source, pred, _, _, _, _, whereRaw, isCount, err := ParseSQL(c.src)
			if err != nil {
				t.Fatalf("parse %q: %v", c.src, err)
			}
			if !isCount {
				t.Errorf("isCount: got false, want true")
			}
			if cols != nil {
				t.Errorf("cols: got %v, want nil", cols)
			}
			if (pred != nil) != c.wantHasPred {
				t.Errorf("pred: got %v, want hasPred=%v", pred, c.wantHasPred)
			}
			if whereRaw != c.wantWhereRaw {
				t.Errorf("whereRaw: got %q, want %q", whereRaw, c.wantWhereRaw)
			}
			if source != c.wantSource {
				t.Errorf("source: got %q, want %q", source, c.wantSource)
			}
		})
	}
}

func TestParseSQLExclude(t *testing.T) {
	cases := []struct {
		name         string
		src          string
		wantExcluded []string
	}{
		{"exclude single", "SELECT * EXCLUDE(a)", []string{"a"}},
		{"exclude multi", "SELECT * EXCLUDE(a, b, c)", []string{"a", "b", "c"}},
		{"exclude dot path", "SELECT * EXCLUDE(address.city)", []string{"address.city"}},
		{"exclude lowercase keyword", "SELECT * exclude(a)", []string{"a"}},
		{"exclude with full pipeline", "SELECT * EXCLUDE(status) FROM /tmp/x.json WHERE cpu > 8 ORDER BY ram WITH provider = 'external:./x.py'", []string{"status"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols, excluded, _, _, _, _, _, _, _, isCount, err := ParseSQL(c.src)
			if err != nil {
				t.Fatalf("parse %q: %v", c.src, err)
			}
			if cols != nil {
				t.Errorf("cols: got %v, want nil (SELECT *)", cols)
			}
			if isCount {
				t.Errorf("isCount: got true, want false")
			}
			if !reflect.DeepEqual(excluded, c.wantExcluded) {
				t.Errorf("excluded: got %v, want %v", excluded, c.wantExcluded)
			}
		})
	}
}

// "count" without a following `(` is still a usable column name — the special
// parsing kicks in only on the function-call form COUNT(*).
func TestParseSQLCountNotKeyword(t *testing.T) {
	cols, _, _, _, _, _, _, _, _, isCount, err := ParseSQL("SELECT count, total")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if isCount {
		t.Errorf("isCount: got true, want false")
	}
	want := []string{"count", "total"}
	if !reflect.DeepEqual(cols, want) {
		t.Errorf("cols: got %v, want %v", cols, want)
	}
}

func TestParseSQLEvalIntegration(t *testing.T) {
	rows := []row{
		{"key": "alice", "age": 30, "city": "SF"},
		{"key": "bob", "age": 25, "city": "NYC"},
		{"key": "carol", "age": 41, "city": "LA"},
	}

	_, _, _, pred, _, _, _, _, _, _, err := ParseSQL("SELECT age WHERE age >= 30")
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
