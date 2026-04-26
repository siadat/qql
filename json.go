package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func loadJSON(path string) (any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var v any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return v, nil
}

func rowsToJSON(rows []row, selected []string) []map[string]any {
	cols := resolveCols(rows, selected)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		obj := make(map[string]any, len(cols))
		for _, c := range cols {
			if c == "id" {
				obj["id"] = r.id
			} else {
				obj[c] = r.cols[c]
			}
		}
		out = append(out, obj)
	}
	return out
}

func printJSON(w io.Writer, rows []row, selected []string) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rowsToJSON(rows, selected))
}

func printJSONL(w io.Writer, rows []row, selected []string) {
	enc := json.NewEncoder(w)
	for _, obj := range rowsToJSON(rows, selected) {
		_ = enc.Encode(obj)
	}
}
