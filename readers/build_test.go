package readers

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
				{"id": "alice", "age": 30},
				{"id": "bob", "age": 25},
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
				{"id": "alice", "address.city": "SF", "address.zip": "94103"},
			},
		},
		{
			name: "array elements use index path components",
			input: `
				alice:
				  tags: [eng, lead]
			`,
			want: []map[string]any{
				{"id": "alice", "tags.0": "eng", "tags.1": "lead"},
			},
		},
		{
			name: "non-map root produces single row without id",
			input: `
				- 1
				- 2
				- 3
			`,
			want: []map[string]any{
				{"0": 1, "1": 2, "2": 3},
			},
		},
		{
			name: "empty map and array produce no columns",
			input: `
				a: {}
				b: []
			`,
			want: []map[string]any{
				{"id": "a"},
				{"id": "b"},
			},
		},
		{
			name: "scalar value under root key uses empty path",
			input: `
				alice: "yes"
			`,
			want: []map[string]any{
				{"id": "alice", "": "yes"},
			},
		},
		{
			name: "null leaf preserved in cols",
			input: `
				alice:
				  middle_name: null
			`,
			want: []map[string]any{
				{"id": "alice", "middle_name": nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := mustParseYAML(t, tt.input)
			got := buildRows(value)
			sort.Slice(got, func(i, j int) bool {
				return rowKey(got[i]) < rowKey(got[j])
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows(%#v) =\n got:  %#v\n want: %#v", value, got, tt.want)
			}
		})
	}
}

func rowKey(r map[string]any) string {
	if v, ok := r["id"].(string); ok {
		return v
	}
	return ""
}
