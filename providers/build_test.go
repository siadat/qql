package providers

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	i := 0
	for i < len(lines[0]) && (lines[0][i] == ' ' || lines[0][i] == '\t') {
		i++
	}
	prefix := lines[0][:i]
	for j, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[j] = line[len(prefix):]
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func mustParseYAML(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := yaml.Unmarshal([]byte(dedent(s)), &v); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}
	return v
}

func TestBuildRows(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []map[string]any
	}{
		{
			name: "flat map",
			input: `
				alice:
				  age: 30
				bob:
				  age: 25
			`,
			want: []map[string]any{
				{"key": "alice", "age": 30},
				{"key": "bob", "age": 25},
			},
		},
		{
			name: "nested map flattened to dot paths",
			input: `
				alice:
				  address:
				    city: SF
				    zip: "94103"
			`,
			want: []map[string]any{
				{"key": "alice", "address.city": "SF", "address.zip": "94103"},
			},
		},
		{
			name: "array elements use index path components",
			input: `
				alice:
				  tags: [eng, lead]
			`,
			want: []map[string]any{
				{"key": "alice", "tags.0": "eng", "tags.1": "lead"},
			},
		},
		{
			name: "list root: each element becomes a row keyed by its index",
			input: `
				- 1
				- 2
				- 3
			`,
			want: []map[string]any{
				{"key": "0", "": 1},
				{"key": "1", "": 2},
				{"key": "2", "": 3},
			},
		},
		{
			name: "list-of-maps root: index is the key, fields become columns",
			input: `
				- {col1: row1col1, col2: row1col2}
				- {col1: row2col1, col2: row2col2}
			`,
			want: []map[string]any{
				{"key": "0", "col1": "row1col1", "col2": "row1col2"},
				{"key": "1", "col1": "row2col1", "col2": "row2col2"},
			},
		},
		{
			name: "list-of-lists root: index is the key, inner indices become columns",
			input: `
				- [a, b]
				- [c, d, [e, f]]
			`,
			want: []map[string]any{
				{"key": "0", "0": "a", "1": "b"},
				{"key": "1", "0": "c", "1": "d", "2.0": "e", "2.1": "f"},
			},
		},
		{
			name: "empty map and array produce no columns",
			input: `
				a: {}
				b: []
			`,
			want: []map[string]any{
				{"key": "a"},
				{"key": "b"},
			},
		},
		{
			name: "scalar value under root key uses empty path",
			input: `
				alice: "yes"
			`,
			want: []map[string]any{
				{"key": "alice", "": "yes"},
			},
		},
		{
			name: "null leaf preserved in cols",
			input: `
				alice:
				  middle_name: null
			`,
			want: []map[string]any{
				{"key": "alice", "middle_name": nil},
			},
		},
		{
			name:  "scalar root yields a single row with empty-key value",
			input: `42`,
			want: []map[string]any{
				{"": 42},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := mustParseYAML(t, tt.input)
			got, err := rowsFromTree(value)
			if err != nil {
				t.Fatalf("rowsFromTree: %v", err)
			}
			sort.Slice(got, func(i, j int) bool {
				return rowKey(got[i]) < rowKey(got[j])
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rowsFromTree(%#v) =\n got:  %#v\n want: %#v", value, got, tt.want)
			}
		})
	}
}

// rowKey returns the row's `key` value as a string for stable test ordering.
// Rows that don't carry a key (e.g. literal-path queries that yielded a single
// row) sort first.
func rowKey(r map[string]any) string {
	v, _ := r["key"].(string)
	return v
}
