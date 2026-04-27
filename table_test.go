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
			printJSON(&buf, tt.rows, nil, nil)
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
			printJSONL(&buf, tt.rows, nil, nil)
			if got := buf.String(); got != tt.want {
				t.Errorf("printJSONL =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestResolveColsExclude(t *testing.T) {
	rows := []row{
		{"key": "a", "cpu": 8, "ram": 16, "status": "up"},
		{"key": "b", "cpu": 4, "ram": 8, "status": "down"},
	}
	got := resolveCols(rows, nil, []string{"status", "ram"})
	want := []string{"key", "cpu"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveCols(*, exclude=status,ram): got %v, want %v", got, want)
	}

	// Excluding a non-existent column is a silent no-op.
	got = resolveCols(rows, nil, []string{"nonexistent"})
	want = []string{"key", "cpu", "ram", "status"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("resolveCols(*, exclude=nonexistent): got %v, want %v", got, want)
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
				"Column  Value  Frequency\n" +
				"------  -----  ---------\n" +
				"region  asia   2/2\n",
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
			want: "Column  Value  Frequency\n" +
				"------  -----  ---------\n" +
				"a       1      1/1\n" +
				"b       x      1/1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printTableWithSummary(&buf, tt.rows, tt.selected, nil, true)
			if got := buf.String(); got != tt.want {
				t.Errorf("printTableWithSummary =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestPrintStats(t *testing.T) {
	tests := []struct {
		name     string
		rows     []row
		selected []string
		want     string
	}{
		{
			name: "frequency descending then value ascending",
			rows: []row{
				{"col1": "x", "col2": "a"},
				{"col1": "x", "col2": "a"},
				{"col1": "x", "col2": "b"},
			},
			selected: []string{"col1", "col2"},
			want: "Cardinality  Column  Values\n" +
				"-----------  ------  ------------\n" +
				"1            col1    x (3)\n" +
				"2            col2    a (2), b (1)\n",
		},
		{
			name: "ties broken by value ascending",
			rows: []row{
				{"c": "b"},
				{"c": "a"},
				{"c": "c"},
			},
			selected: []string{"c"},
			want: "Cardinality  Column  Values\n" +
				"-----------  ------  -------------------\n" +
				"3            c       a (1), b (1), c (1)\n",
		},
		{
			name: "missing keys count as null",
			rows: []row{
				{"a": 1},
				{"a": 2},
				{"a": 1},
			},
			selected: []string{"a", "b"},
			want: "Cardinality  Column  Values\n" +
				"-----------  ------  ------------\n" +
				"1            b       null (3)\n" +
				"2            a       1 (2), 2 (1)\n",
		},
		{
			name:     "zero rows produces only the header",
			rows:     nil,
			selected: []string{"a", "b"},
			want: "Cardinality  Column  Values\n" +
				"-----------  ------  ------\n" +
				"0            a       \n" +
				"0            b       \n",
		},
		{
			name: "rows sorted by unique-value count ascending",
			rows: []row{
				{"low": "x", "high": "a"},
				{"low": "x", "high": "b"},
				{"low": "x", "high": "c"},
			},
			selected: []string{"high", "low"},
			want: "Cardinality  Column  Values\n" +
				"-----------  ------  -------------------\n" +
				"1            low     x (3)\n" +
				"3            high    a (1), b (1), c (1)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printStats(&buf, tt.rows, tt.selected, nil, true)
			if got := buf.String(); got != tt.want {
				t.Errorf("printStats =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
