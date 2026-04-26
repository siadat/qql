package main

import (
	"bytes"
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
		path  string
		input string
		want  []row
	}{
		{
			name: "flat map",
			path: "f.json",
			input: `
				alice:
				  age: 30
				bob:
				  age: 25
			`,
			want: []row{
				{id: "alice", cols: map[string]any{"age": 30}},
				{id: "bob", cols: map[string]any{"age": 25}},
			},
		},
		{
			name: "nested map flattened to dot paths",
			path: "f.json",
			input: `
				alice:
				  address:
				    city: SF
				    zip: "94103"
			`,
			want: []row{
				{id: "alice", cols: map[string]any{
					"address.city": "SF",
					"address.zip":  "94103",
				}},
			},
		},
		{
			name: "array elements use index path components",
			path: "f.json",
			input: `
				alice:
				  tags: [eng, lead]
			`,
			want: []row{
				{id: "alice", cols: map[string]any{
					"tags.0": "eng",
					"tags.1": "lead",
				}},
			},
		},
		{
			name: "non-map root falls back to file path as id",
			path: "f.json",
			input: `
				- 1
				- 2
				- 3
			`,
			want: []row{
				{id: "f.json", cols: map[string]any{
					"0": 1,
					"1": 2,
					"2": 3,
				}},
			},
		},
		{
			name: "empty map and array produce no columns",
			path: "f.json",
			input: `
				a: {}
				b: []
			`,
			want: []row{
				{id: "a", cols: map[string]any{}},
				{id: "b", cols: map[string]any{}},
			},
		},
		{
			name: "scalar value under root key uses empty path",
			path: "f.json",
			input: `
				alice: "yes"
			`,
			want: []row{
				{id: "alice", cols: map[string]any{"": "yes"}},
			},
		},
		{
			name: "null leaf preserved in cols",
			path: "f.json",
			input: `
				alice:
				  middle_name: null
			`,
			want: []row{
				{id: "alice", cols: map[string]any{"middle_name": nil}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := mustParseYAML(t, tt.input)
			got := buildRows(tt.path, value)
			sort.Slice(got, func(i, j int) bool { return got[i].id < got[j].id })
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows(%q, %#v) =\n got:  %#v\n want: %#v", tt.path, value, got, tt.want)
			}
		})
	}
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "rows with mixed types",
			input: `
				alice:
				  age: 30
				  city: SF
				bob:
				  age: 25
				  active: false
			`,
			want: `[
  {
    "active": null,
    "age": 30,
    "city": "SF",
    "id": "alice"
  },
  {
    "active": false,
    "age": 25,
    "city": null,
    "id": "bob"
  }
]
`,
		},
		{
			name:  "empty rows",
			input: `{}`,
			want:  "[]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := mustParseYAML(t, tt.input)
			rows := buildRows("f.yaml", value)
			sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
			var buf bytes.Buffer
			printJSON(&buf, rows, nil)
			if got := buf.String(); got != tt.want {
				t.Errorf("printJSON =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestPrintJSONL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "rows with mixed types",
			input: `
				alice:
				  age: 30
				  city: SF
				bob:
				  age: 25
				  active: false
			`,
			want: `{"active":null,"age":30,"city":"SF","id":"alice"}
{"active":false,"age":25,"city":null,"id":"bob"}
`,
		},
		{
			name:  "empty rows",
			input: `{}`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := mustParseYAML(t, tt.input)
			rows := buildRows("f.yaml", value)
			sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
			var buf bytes.Buffer
			printJSONL(&buf, rows, nil)
			if got := buf.String(); got != tt.want {
				t.Errorf("printJSONL =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
