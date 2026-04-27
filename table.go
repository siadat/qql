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
// followed by a blank line and a "Frequency | Column | Value" summary table
// for the columns whose value was identical across every row.
//
// The point is to keep the main table narrow enough to fit common terminal
// widths: columns that carry no per-row information (e.g. a status column
// that's "up" for every match) are pulled out so the rows that DO vary stay
// readable. The hoisted columns aren't dropped, just relocated to the
// compact summary block below.
func printTableWithSummary(w io.Writer, rows []row, selected []string, header bool) {
	cols := resolveCols(rows, selected)
	variable, constant, constValues := partitionConstantCols(rows, cols)

	if len(variable) > 0 || len(rows) == 0 {
		printTable(w, rows, variable, header)
	}

	if len(constant) == 0 {
		return
	}
	if len(variable) > 0 || len(rows) == 0 {
		fmt.Fprintln(w)
	}
	freq := fmt.Sprintf("%d/%d", len(rows), len(rows))
	summaryRows := make([]row, 0, len(constant))
	for _, c := range constant {
		summaryRows = append(summaryRows, row{
			"Frequency": freq,
			"Column":    c,
			"Value":     constValues[c],
		})
	}
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
	printTable(w, statsRows, []string{"Unique", "Column", "Values"}, header)
}

func formatStatValue(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprintf("%v", v)
}
