package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"text/tabwriter"
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

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprint(tw, "id")
	for _, c := range cols {
		fmt.Fprintf(tw, "\t%s", c)
	}
	fmt.Fprintln(tw)

	for _, r := range rows {
		fmt.Fprint(tw, r.id)
		for _, c := range cols {
			v, ok := r.cols[c]
			if !ok || v == nil {
				fmt.Fprint(tw, "\t")
			} else {
				fmt.Fprintf(tw, "\t%v", v)
			}
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}
