package main

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

type row = map[string]any

// resolveCols picks the columns for output: selected verbatim if given,
// otherwise the union of keys across rows. The `key` column (set by the
// loader to the map key or list index of each top-level entry) is pulled to
// the front so the row identifier reads first, immediately followed by the
// `value` column when present (it carries the scalar payload for shapes like
// list-of-scalars or top-level scalar leaves, so it pairs visually with `key`).
// The rest follow alphabetically. Names in `excluded` are dropped from the
// result, supporting `SELECT * EXCLUDE(col1, col2)`.
func resolveCols(rows []row, selected []string, excluded []string) []string {
	var cols []string
	if selected != nil {
		cols = selected
	} else {
		colSet := map[string]struct{}{}
		for _, r := range rows {
			for k := range r {
				colSet[k] = struct{}{}
			}
		}
		hasKey := false
		hasValue := false
		others := make([]string, 0, len(colSet))
		for c := range colSet {
			switch c {
			case "key":
				hasKey = true
			case "value":
				hasValue = true
			default:
				others = append(others, c)
			}
		}
		sort.Strings(others)
		if hasKey {
			cols = append(cols, "key")
		}
		if hasValue {
			cols = append(cols, "value")
		}
		cols = append(cols, others...)
	}
	if len(excluded) == 0 {
		return cols
	}
	excludeSet := make(map[string]struct{}, len(excluded))
	for _, c := range excluded {
		excludeSet[c] = struct{}{}
	}
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		if _, skip := excludeSet[c]; !skip {
			out = append(out, c)
		}
	}
	return out
}

func printTable(w io.Writer, rows []row, selected []string, excluded []string, header bool) {
	cols := resolveCols(rows, selected, excluded)

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

// partitionConstantCols splits cols into ones whose value differs across rows
// (variable) and ones whose value is identical for every row (constant). The
// constValues map carries the shared value for each constant column. Missing
// keys count as nil, so a column absent from every row is treated as constant
// nil. With zero rows nothing is constant.
func partitionConstantCols(rows []row, cols []string) (variable, constant []string, constValues map[string]any) {
	if len(rows) == 0 {
		return cols, nil, nil
	}
	constValues = map[string]any{}
	for _, c := range cols {
		first := rows[0][c]
		allSame := true
		for _, r := range rows[1:] {
			if !reflect.DeepEqual(r[c], first) {
				allSame = false
				break
			}
		}
		if allSame {
			constant = append(constant, c)
			constValues[c] = first
		} else {
			variable = append(variable, c)
		}
	}
	return variable, constant, constValues
}

// printTableWithSummary prints the main table with constant columns stripped,
// followed by a blank line and a small "Coverage | Column | Value" table
// for the columns whose value was identical across every row.
//
// The point is to keep the main table narrow enough to fit common terminal
// widths: columns that carry no per-row information (e.g. a status column
// that's "up" for every match) are pulled out so the rows that DO vary stay
// readable. The hoisted columns aren't dropped, just relocated to the
// compact summary block below.
//
// The summary block is the same value-frequency table that --stats builds,
// restricted to the entries at max frequency (one row per constant column,
// frequency "N/N"). --summary is just --stats's value-frequency view filtered
// to count == len(rows).
func printTableWithSummary(w io.Writer, rows []row, selected []string, excluded []string, header bool) {
	cols := resolveCols(rows, selected, excluded)
	variable, constant, _ := partitionConstantCols(rows, cols)

	if len(variable) > 0 || len(rows) == 0 {
		printTable(w, rows, variable, nil, header)
	}

	if len(constant) == 0 {
		return
	}
	if len(variable) > 0 || len(rows) == 0 {
		fmt.Fprintln(w)
	}
	// Constant cols are exactly the max-frequency entries of the value-
	// frequency table, so passing only those cols is equivalent to filtering
	// the full table to count == len(rows).
	summaryRows := buildValueFrequencyRows(rows, constant)
	printTable(w, summaryRows, []string{"Coverage", "Column", "Value"}, nil, header)
}

// printStats replaces the main table with a per-column breakdown of the
// distinct values seen and how often each one occurs. It is meant for getting
// a quick feel for a result set's shape (cardinality, dominant values) without
// scrolling through every row, e.g. "is this column basically constant?", "how
// many distinct authors are in this commit query?".
//
// Each output row carries: the count of unique values, the column name, and a
// "val (freq)" list sorted by frequency descending (ties broken by value
// ascending) and joined with ", ". Missing keys count as nil and render as
// "null", matching the main table.
func printStats(w io.Writer, rows []row, selected []string, excluded []string, header bool) {
	cols := resolveCols(rows, selected, excluded)
	statsRows := make([]row, 0, len(cols))
	for _, c := range cols {
		counts := map[string]int{}
		var order []string
		for _, r := range rows {
			s := formatStatValue(r[c])
			if _, seen := counts[s]; !seen {
				order = append(order, s)
			}
			counts[s]++
		}
		sort.SliceStable(order, func(i, j int) bool {
			if counts[order[i]] != counts[order[j]] {
				return counts[order[i]] > counts[order[j]]
			}
			return order[i] < order[j]
		})
		parts := make([]string, len(order))
		for i, v := range order {
			parts[i] = fmt.Sprintf("%s (%d)", v, counts[v])
		}
		statsRows = append(statsRows, row{
			"Cardinality": len(order),
			"Column":      c,
			"Values":      strings.Join(parts, ", "),
		})
	}
	sort.SliceStable(statsRows, func(i, j int) bool {
		return statsRows[i]["Cardinality"].(int) < statsRows[j]["Cardinality"].(int)
	})

	// The value-frequency table (buildValueFrequencyRows) is intentionally
	// suppressed for now — the helper is still used by --summary, which
	// filters it to the max-frequency entries. Re-enable here when we want it
	// back in the --stats view.
	printTable(w, statsRows, []string{"Cardinality", "Column", "Values"}, nil, header)
}

// buildValueFrequencyRows returns one row per (column, value) pair across the
// dataset, sorted by frequency descending so dominant values surface first.
// Unlike the --summary view (which only hoists 100% constants), this lists
// every value, e.g. a 7/8 dominant value still shows up alongside the 1/8
// outlier. Ties on frequency break by column then value, both ascending, so
// the order is deterministic.
func buildValueFrequencyRows(rows []row, cols []string) []row {
	if len(rows) == 0 {
		return nil
	}
	type entry struct {
		col, value string
		count      int
	}
	var entries []entry
	for _, c := range cols {
		counts := map[string]int{}
		for _, r := range rows {
			counts[formatStatValue(r[c])]++
		}
		for v, n := range counts {
			entries = append(entries, entry{c, v, n})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		if entries[i].col != entries[j].col {
			return entries[i].col < entries[j].col
		}
		return entries[i].value < entries[j].value
	})
	total := len(rows)
	out := make([]row, 0, len(entries))
	for _, e := range entries {
		out = append(out, row{
			"Column":   e.col,
			"Value":    e.value,
			"Coverage": fmt.Sprintf("%d/%d", e.count, total),
		})
	}
	return out
}

func formatStatValue(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%v", v)
}
