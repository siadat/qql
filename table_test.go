package main

import (
	"bytes"
	"testing"
)

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name string
		rows []row
		want string
	}{
		{
			name: "rows with mixed types",
			rows: []row{
				{"key": "alice", "age": 30, "city": "SF"},
				{"key": "bob", "age": 25, "active": false},
			},
			want: `[
  {
    "active": null,
    "age": 30,
    "city": "SF",
    "key": "alice"
  },
  {
    "active": false,
    "age": 25,
    "city": null,
    "key": "bob"
  }
]
`,
		},
		{
			name: "empty rows",
			rows: nil,
			want: "[]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printJSON(&buf, tt.rows, nil)
			if got := buf.String(); got != tt.want {
				t.Errorf("printJSON =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestPrintJSONL(t *testing.T) {
	tests := []struct {
		name string
		rows []row
		want string
	}{
		{
			name: "rows with mixed types",
			rows: []row{
				{"key": "alice", "age": 30, "city": "SF"},
				{"key": "bob", "age": 25, "active": false},
			},
			want: `{"active":null,"age":30,"city":"SF","key":"alice"}
{"active":false,"age":25,"city":null,"key":"bob"}
`,
		},
		{
			name: "empty rows",
			rows: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printJSONL(&buf, tt.rows, nil)
			if got := buf.String(); got != tt.want {
				t.Errorf("printJSONL =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}
