package main

import (
	"bytes"
	"reflect"
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

func TestPartitionConstantCols(t *testing.T) {
	tests := []struct {
		name         string
		rows         []row
		cols         []string
		wantVariable []string
		wantConstant []string
		wantValues   map[string]any
	}{
		{
			name:         "zero rows treats nothing as constant",
			rows:         nil,
			cols:         []string{"a", "b"},
			wantVariable: []string{"a", "b"},
			wantConstant: nil,
			wantValues:   nil,
		},
		{
			name: "single row makes every column constant",
			rows: []row{
				{"a": 1, "b": "x"},
			},
			cols:         []string{"a", "b"},
			wantVariable: nil,
			wantConstant: []string{"a", "b"},
			wantValues:   map[string]any{"a": 1, "b": "x"},
		},
		{
			name: "mixed variable and constant",
			rows: []row{
				{"region": "a", "cpu": 8, "status": "up"},
				{"region": "a", "cpu": 32, "status": "up"},
				{"region": "a", "cpu": 16, "status": "up"},
			},
			cols:         []string{"region", "cpu", "status"},
			wantVariable: []string{"cpu"},
			wantConstant: []string{"region", "status"},
			wantValues:   map[string]any{"region": "a", "status": "up"},
		},
		{
			name: "missing keys count as nil and stay constant",
			rows: []row{
				{"a": 1},
				{"a": 2},
			},
			cols:         []string{"a", "b"},
			wantVariable: []string{"a"},
			wantConstant: []string{"b"},
			wantValues:   map[string]any{"b": nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variable, constant, values := partitionConstantCols(tt.rows, tt.cols)
			if !reflect.DeepEqual(variable, tt.wantVariable) {
				t.Errorf("variable: got %v, want %v", variable, tt.wantVariable)
			}
			if !reflect.DeepEqual(constant, tt.wantConstant) {
				t.Errorf("constant: got %v, want %v", constant, tt.wantConstant)
			}
			if !reflect.DeepEqual(values, tt.wantValues) {
				t.Errorf("values: got %v, want %v", values, tt.wantValues)
			}
		})
	}
}

func TestPrintTableWithSummary(t *testing.T) {
	tests := []struct {
		name     string
		rows     []row
		selected []string
		want     string
	}{
		{
			name: "constant column hoisted into summary",
			rows: []row{
				{"key": "tokyo", "region": "asia", "cpu": 8},
				{"key": "osaka", "region": "asia", "cpu": 32},
			},
			selected: []string{"key", "region", "cpu"},
			want: "key    cpu\n" +
				"-----  ---\n" +
				"tokyo  8\n" +
				"osaka  32\n" +
				"\n" +
				"Frequency  Column  Value\n" +
				"---------  ------  -----\n" +
				"2/2        region  asia\n",
		},
		{
			name: "no constant columns prints only main table",
			rows: []row{
				{"a": 1, "b": 2},
				{"a": 3, "b": 4},
			},
			selected: []string{"a", "b"},
			want: "a  b\n" +
				"-  -\n" +
				"1  2\n" +
				"3  4\n",
		},
		{
			name: "all columns constant prints only summary",
			rows: []row{
				{"a": 1, "b": "x"},
			},
			selected: []string{"a", "b"},
			want: "Frequency  Column  Value\n" +
				"---------  ------  -----\n" +
				"1/1        a       1\n" +
				"1/1        b       x\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printTableWithSummary(&buf, tt.rows, tt.selected, true)
			if got := buf.String(); got != tt.want {
				t.Errorf("printTableWithSummary =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
