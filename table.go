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

	rows := make([]row, 0, len(m))
	for k, v := range m {
		cols := map[string]any{}
		flatten(v, "", cols)
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

func resolveCols(rows []row, selected []string) []string {
	if selected != nil {
		return selected
	}
	colSet := map[string]struct{}{}
	for _, r := range rows {
		for k := range r.cols {
			if k != "id" {
				colSet[k] = struct{}{}
			}
		}
	}
	auto := make([]string, 0, len(colSet))
	for c := range colSet {
		auto = append(auto, c)
	}
	sort.Strings(auto)
	return append([]string{"id"}, auto...)
}

func printTable(w io.Writer, rows []row, selected []string) {
	cols := resolveCols(rows, selected)

	cellAt := func(r row, c string) string {
		if c == "id" {
			return r.id
		}
		v, ok := r.cols[c]
		if !ok || v == nil {
			return "null"
		}
		return fmt.Sprintf("%v", v)
	}

	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	for _, r := range rows {
		for i, c := range cols {
			if s := cellAt(r, c); len(s) > widths[i] {
				widths[i] = len(s)
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

	writeRow(cols)

	sep := make([]string, len(cols))
	for i, width := range widths {
		sep[i] = strings.Repeat("-", width)
	}
	writeRow(sep)

	for _, r := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = cellAt(r, c)
		}
		writeRow(vals)
	}
}
