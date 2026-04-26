package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type row = map[string]any

func resolveCols(rows []row, selected []string) []string {
	if selected != nil {
		return selected
	}
	colSet := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			colSet[k] = struct{}{}
		}
	}
	auto := make([]string, 0, len(colSet))
	for c := range colSet {
		auto = append(auto, c)
	}
	sort.Strings(auto)
	return auto
}

func printTable(w io.Writer, rows []row, selected []string, header bool) {
	cols := resolveCols(rows, selected)

	cellAt := func(r row, c string) string {
		v, ok := r[c]
		if !ok || v == nil {
			return "null"
		}
		return fmt.Sprintf("%v", v)
	}

	widths := make([]int, len(cols))
	if header {
		for i, c := range cols {
			widths[i] = len(c)
		}
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

	if header {
		writeRow(cols)
		sep := make([]string, len(cols))
		for i, width := range widths {
			sep[i] = strings.Repeat("-", width)
		}
		writeRow(sep)
	}

	for _, r := range rows {
		vals := make([]string, len(cols))
		for i, c := range cols {
			vals[i] = cellAt(r, c)
		}
		writeRow(vals)
	}
}
