package providers

import (
	"fmt"
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
		name     string
		prefix string
		input    string
		want     []map[string]any
	}{
		{
			name: "flat map (default path)",
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
			name: "non-map root produces single row without key",
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
			name:     "explicit single-wildcard path",
			prefix: "*",
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
			name:     "two wildcards capture key_capture_1 and key",
			prefix: "*.servers.*",
			input: `
				region-a:
				  servers:
				    web1: {cpu: 8, ram: 32}
				    db1: {cpu: 32, ram: 128}
				region-b:
				  servers:
				    web1: {cpu: 4, ram: 16}
			`,
			want: []map[string]any{
				{"key_capture_1": "region-a", "key": "region-a.servers.db1", "cpu": 32, "ram": 128},
				{"key_capture_1": "region-a", "key": "region-a.servers.web1", "cpu": 8, "ram": 32},
				{"key_capture_1": "region-b", "key": "region-b.servers.web1", "cpu": 4, "ram": 16},
			},
		},
		{
			name:     "three wildcards capture key_capture_1 key_capture_2 key",
			prefix: "*.*.*",
			input: `
				a:
				  b:
				    c: {x: 1}
				d:
				  e:
				    f: {x: 2}
			`,
			want: []map[string]any{
				{"key_capture_1": "a", "key_capture_2": "b", "key": "a.b.c", "x": 1},
				{"key_capture_1": "d", "key_capture_2": "e", "key": "d.e.f", "x": 2},
			},
		},
		{
			name:     "literal path with no wildcards yields one row",
			prefix: "alice.address",
			input: `
				alice:
				  address:
				    city: SF
				    zip: "94103"
				bob:
				  address:
				    city: NYC
			`,
			want: []map[string]any{
				{"city": "SF", "zip": "94103"},
			},
		},
		{
			name:     "wildcard mixed with literal",
			prefix: "regions.*.cpu",
			input: `
				regions:
				  east:
				    cpu: 8
				  west:
				    cpu: 16
			`,
			want: []map[string]any{
				{"key": "regions.east.cpu", "": 8},
				{"key": "regions.west.cpu", "": 16},
			},
		},
		{
			name:     "non-matching branches silently skipped",
			prefix: "*.servers.*",
			input: `
				region-a:
				  servers:
				    web1: {cpu: 8}
				broken:
				  servers: "not a map"
				no-servers:
				  other: {x: 1}
			`,
			want: []map[string]any{
				{"key_capture_1": "region-a", "key": "region-a.servers.web1", "cpu": 8},
			},
		},
		{
			name:     "path matches nothing yields empty",
			prefix: "*.does-not-exist.*",
			input: `
				alice:
				  friends:
				    bob: {age: 30}
			`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := mustParseYAML(t, tt.input)
			got, err := buildRows(value, tt.prefix)
			if err != nil {
				t.Fatalf("buildRows: %v", err)
			}
			sort.Slice(got, func(i, j int) bool {
				return rowKey(got[i]) < rowKey(got[j])
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows(%#v, %q) =\n got:  %#v\n want: %#v", value, tt.prefix, got, tt.want)
			}
		})
	}
}

// rowKey produces a stable comparator for sorting rows in tests. It walks
// key_capture_1, key_capture_2, … then key and joins their string values,
// giving a deterministic total order for cases with one or more wildcard
// captures.
func rowKey(r map[string]any) string {
	var parts []string
	for i := 1; ; i++ {
		k := fmt.Sprintf("key_capture_%d", i)
		v, ok := r[k].(string)
		if !ok {
			break
		}
		parts = append(parts, v)
	}
	if v, ok := r["key"].(string); ok {
		parts = append(parts, v)
	}
	return strings.Join(parts, "/")
}
