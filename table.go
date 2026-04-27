package main

import (
	"fmt"
	"io"
	"reflect"
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
// followed by a blank line and a small "Column | Value | Frequency" table
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
func printTableWithSummary(w io.Writer, rows []row, selected []string, header bool) {
	cols := resolveCols(rows, selected)
	variable, constant, _ := partitionConstantCols(rows, cols)

	if len(variable) > 0 || len(rows) == 0 {
		printTable(w, rows, variable, header)
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
	printTable(w, summaryRows, []string{"Column", "Value", "Frequency"}, header)
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
func printStats(w io.Writer, rows []row, selected []string, header bool) {
	cols := resolveCols(rows, selected)
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
			"Unique": len(order),
			"Column": c,
			"Values": strings.Join(parts, ", "),
		})
	}
	sort.SliceStable(statsRows, func(i, j int) bool {
		return statsRows[i]["Unique"].(int) < statsRows[j]["Unique"].(int)
	})

	// The value-frequency table (buildValueFrequencyRows) is intentionally
	// suppressed for now — the helper is still used by --summary, which
	// filters it to the max-frequency entries. Re-enable here when we want it
	// back in the --stats view.
	printTable(w, statsRows, []string{"Unique", "Column", "Values"}, header)
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
			"Column":    e.col,
			"Value":     e.value,
			"Frequency": fmt.Sprintf("%d/%d", e.count, total),
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
