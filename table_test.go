package main

import (
	"reflect"
	"testing"
)

func TestBuildRows(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		value any
		want  []row
	}{
		{
			name: "flat map",
			path: "f.json",
			value: map[string]any{
				"alice": map[string]any{"age": 30},
				"bob":   map[string]any{"age": 25},
			},
			want: []row{
				{id: "alice", cols: map[string]any{"age": 30}},
				{id: "bob", cols: map[string]any{"age": 25}},
			},
		},
		{
			name: "nested map flattened to dot paths",
			path: "f.json",
			value: map[string]any{
				"alice": map[string]any{
					"address": map[string]any{"city": "SF", "zip": "94103"},
				},
			},
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
			value: map[string]any{
				"alice": map[string]any{"tags": []any{"eng", "lead"}},
			},
			want: []row{
				{id: "alice", cols: map[string]any{
					"tags.0": "eng",
					"tags.1": "lead",
				}},
			},
		},
		{
			name:  "non-map root falls back to file path as id",
			path:  "f.json",
			value: []any{1, 2, 3},
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
			value: map[string]any{
				"a": map[string]any{},
				"b": []any{},
			},
			want: []row{
				{id: "a", cols: map[string]any{}},
				{id: "b", cols: map[string]any{}},
			},
		},
		{
			name: "scalar value under root key uses empty path",
			path: "f.json",
			value: map[string]any{
				"alice": "yes",
			},
			want: []row{
				{id: "alice", cols: map[string]any{"": "yes"}},
			},
		},
		{
			name: "rows sorted by root key",
			path: "f.json",
			value: map[string]any{
				"c": map[string]any{"x": 1},
				"a": map[string]any{"x": 1},
				"b": map[string]any{"x": 1},
			},
			want: []row{
				{id: "a", cols: map[string]any{"x": 1}},
				{id: "b", cols: map[string]any{"x": 1}},
				{id: "c", cols: map[string]any{"x": 1}},
			},
		},
		{
			name: "null leaf preserved in cols",
			path: "f.json",
			value: map[string]any{
				"alice": map[string]any{"middle_name": nil},
			},
			want: []row{
				{id: "alice", cols: map[string]any{"middle_name": nil}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRows(tt.path, tt.value)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRows(%q, %#v) =\n got:  %#v\n want: %#v", tt.path, tt.value, got, tt.want)
			}
		})
	}
}
