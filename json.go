package main

import (
	"encoding/json"
	"io"
)

func rowsToJSON(rows []row, selected []string, excluded []string) []map[string]any {
	cols := resolveCols(rows, selected, excluded)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		obj := make(map[string]any, len(cols))
		for _, c := range cols {
			obj[c] = r[c]
		}
		out = append(out, obj)
	}
	return out
}

func printJSON(w io.Writer, rows []row, selected []string, excluded []string) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rowsToJSON(rows, selected, excluded))
}

func printJSONL(w io.Writer, rows []row, selected []string, excluded []string) {
	enc := json.NewEncoder(w)
	for _, obj := range rowsToJSON(rows, selected, excluded) {
		_ = enc.Encode(obj)
	}
}
