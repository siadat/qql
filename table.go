package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type row struct {
	id   string
	cols map[string]any
}

func buildRows(path string, value any) []row {
	m, ok := value.(map[string]any)
	if !ok {
		cols := map[string]any{}
		flatten(value, "", cols)
		return []row{{id: path, cols: cols}}
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rows := make([]row, 0, len(keys))
	for _, k := range keys {
		cols := map[string]any{}
		flatten(m[k], "", cols)
		rows = append(rows, row{id: k, cols: cols})
	}
	return rows
}

func flatten(v any, prefix string, out map[string]any) {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			flatten(x[k], joinPath(prefix, k), out)
		}
	case []any:
		if len(x) == 0 {
			return
		}
		for i, e := range x {
			flatten(e, joinPath(prefix, strconv.Itoa(i)), out)
		}
	default:
		out[prefix] = x
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func printTable(w io.Writer, rows []row) {
	colSet := map[string]struct{}{}
	for _, r := range rows {
		for k := range r.cols {
			colSet[k] = struct{}{}
		}
	}
	cols := make([]string, 0, len(colSet))
	for c := range colSet {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	cellAt := func(r row, c string) string {
		v, ok := r.cols[c]
		if !ok || v == nil {
			return "null"
		}
		return fmt.Sprintf("%v", v)
	}

	widths := make([]int, len(cols)+1)
	widths[0] = len("id")
	for i, c := range cols {
		widths[i+1] = len(c)
	}
	for _, r := range rows {
		if len(r.id) > widths[0] {
			widths[0] = len(r.id)
		}
		for i, c := range cols {
			if s := cellAt(r, c); len(s) > widths[i+1] {
				widths[i+1] = len(s)
			}
		}
	}

	const gap = "  "
	writeRow := func(values []string) {
		for i, v := range values {
			if i > 0 {
				fmt.Fprint(w, gap)
			}
			fmt.Fprint(w, v)
			if i < len(values)-1 {
				if pad := widths[i] - len(v); pad > 0 {
					fmt.Fprint(w, strings.Repeat(" ", pad))
				}
			}
		}
		fmt.Fprintln(w)
	}

	header := append([]string{"id"}, cols...)
	writeRow(header)

	sep := make([]string, len(widths))
	for i, width := range widths {
		sep[i] = strings.Repeat("-", width)
	}
	writeRow(sep)

	for _, r := range rows {
		vals := make([]string, len(cols)+1)
		vals[0] = r.id
		for i, c := range cols {
			vals[i+1] = cellAt(r, c)
		}
		writeRow(vals)
	}
}
