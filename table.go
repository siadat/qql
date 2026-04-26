package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type row = map[string]any

// resolveCols picks the columns for output: selected verbatim if given,
// otherwise the union of keys across rows. The wildcard-capture columns
// (`key`, `key_capture_1`, `key_capture_2`, …) are pulled to the front so the
// row identifier and its prefix-captures read left-to-right; the rest follow
// alphabetically.
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
	var keyCols, otherCols []string
	for c := range colSet {
		if isKeyCol(c) {
			keyCols = append(keyCols, c)
		} else {
			otherCols = append(otherCols, c)
		}
	}
	sort.Slice(keyCols, func(i, j int) bool {
		return keyColRank(keyCols[i]) < keyColRank(keyCols[j])
	})
	sort.Strings(otherCols)
	return append(keyCols, otherCols...)
}

func isKeyCol(c string) bool {
	return c == "key" || strings.HasPrefix(c, "key_capture_")
}

// keyColRank orders the wildcard-capture columns: `key`, `key_capture_1`,
// `key_capture_2`, …. `key` (the row's full-path identifier) leads, then
// `key_capture_<n>` by n. A column like `key_capture_foo` (non-numeric suffix)
// trails the numeric ones.
func keyColRank(c string) int {
	if c == "key" {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimPrefix(c, "key_capture_"))
	if err != nil {
		return 1<<31 - 1
	}
	return n
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
