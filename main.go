package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/siadat/qql/parser"
	"github.com/siadat/qql/providers"
)

func main() {
	var outFlag string
	flag.StringVar(&outFlag, "out", "table", "output format: table, json, jsonl, shell")
	flag.StringVar(&outFlag, "o", "table", "output format (shorthand for --out)")
	noHeader := flag.Bool("no-header", false, "hide the header row, separator, and trailing row count in table output")
	summary := flag.Bool("summary", false, "shrink the table by hoisting columns whose value is identical across every row into a small summary table printed below, so the main table is narrower and fits more terminals")
	stats := flag.Bool("stats", false, "instead of the rows, print a per-column breakdown: unique-value count and a 'value (freq)' list sorted by frequency, useful for sizing up a result set without scrolling")
	noTruncate := flag.Bool("no-truncate", false, "do not clip lines to terminal width. by default, when stdout is an interactive terminal, table output is truncated so wrapping does not garble the layout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-o FORMAT] QUERY [file ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	// First positional is always the query (empty when omitted, which the
	// parser turns into `SELECT *`). Remaining positionals are data
	// sources. Lets `cmd | qql` work with no args at all.
	queryStr := ""
	var posArgs []string
	if len(args) > 0 {
		queryStr = args[0]
		posArgs = args[1:]
	}

	stmt, err := parser.Parse(queryStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if u, ok := stmt.(*parser.UpdateStmt); ok {
		runUpdate(u, posArgs)
		return
	}

	sel := stmt.(*parser.SelectStmt)
	selected, excluded, sqlSource, pred, orderBy, limit, offset, with, whereRaw, isCount :=
		sel.Selected, sel.Excluded, sel.Source, sel.Pred, sel.OrderBy, sel.Limit, sel.Offset, sel.With, sel.WhereRaw, sel.IsCount

	var paths []string
	if sqlSource != "" {
		paths = append(paths, sqlSource)
	}
	paths = append(paths, posArgs...)

	// Auto-inject `-` whenever stdin is piped so `cmd | qql 'SELECT *'`
	// works without an explicit `-`. Skipped when the user already typed
	// `-` (the duplicate check below would otherwise fire). When stdin is
	// a TTY we leave paths alone so an interactive `qql 'SELECT *'` still
	// hits the usage path instead of hanging on a blank read.
	if !isTerminal(os.Stdin) {
		hasStdin := false
		for _, p := range paths {
			if p == "-" {
				hasStdin = true
				break
			}
		}
		if !hasStdin {
			paths = append(paths, "-")
		}
	}

	stdinIdx := -1
	for i, p := range paths {
		if p == "-" {
			if stdinIdx >= 0 {
				fmt.Fprintln(os.Stderr, "stdin source `-` may only appear once")
				os.Exit(2)
			}
			stdinIdx = i
		}
	}

	var groups [][]row
	if stdinIdx >= 0 {
		if with.Provider != "" {
			fmt.Fprintln(os.Stderr, "stdin source `-` cannot be combined with WITH provider")
			os.Exit(2)
		}
		if len(paths) > 1 {
			fmt.Fprintln(os.Stderr, "stdin source `-` cannot be combined with other files")
			os.Exit(2)
		}
		// Wrap stdin so we can peek the first byte to choose between the
		// streaming path (JSONL only) and the buffered path (auto-detect
		// JSONL vs single JSON document).
		stdinReader := bufio.NewReader(os.Stdin)
		firstByte, err := peekFirstNonWS(stdinReader)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		// Streaming path: JSONL output, no aggregation that needs the full
		// result, and an input that *could* be JSONL. A leading `[` (or
		// scalar/EOF) means a single JSON document, which is fundamentally
		// not stream-friendly — fall through to the buffered path.
		if outFlag == "jsonl" && len(orderBy) == 0 && !isCount && firstByte == '{' {
			streamStdinJSONL(stdinReader, selected, excluded, pred, limit, offset)
			return
		}
		stdinRows, firstKeys, err := providers.LoadStdin(stdinReader)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		groups = [][]row{stdinRows}
		// "First object determines the columns": when the user wrote SELECT *,
		// fall back to the first row's keys in their JSONL order so the output
		// matches what was piped in instead of the alphabetical union.
		if selected == nil && len(firstKeys) > 0 {
			selected = firstKeys
		}
	} else if with.Provider != "" {
		// Provider-driven loads run once per query, not once per path. External
		// providers receive every path via ctx.Files and decide how to
		// interleave them. The git provider reads its repo path out of the
		// provider value itself and ignores Source/Files entirely.
		ctx := providers.Context{
			Source:   sqlSource,
			Files:    paths,
			Provider: with.Provider,
			Select:   selected,
			Where:    whereRaw,
			OrderBy:  toProviderOrderBy(orderBy),
			Limit:    limit,
			Offset:   offset,
		}
		var err error
		groups, err = providers.Load(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		if len(paths) == 0 {
			flag.Usage()
			os.Exit(2)
		}
		for _, path := range paths {
			ctx := providers.Context{
				Source:   path,
				Files:    paths,
				Provider: with.Provider,
				Select:   selected,
				Where:    whereRaw,
				OrderBy:  toProviderOrderBy(orderBy),
				Limit:    limit,
				Offset:   offset,
			}
			pathGroups, err := providers.Load(ctx)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			groups = append(groups, pathGroups...)
		}
		// Multiple positional files keep their original "concatenate into one
		// table" semantics: passing N files is treated as one combined source.
		// Multi-doc YAML still splits, but only when its file is the sole
		// source — that's the case where the `---` separators are the user's
		// only signal for how to group output.
		if len(paths) > 1 {
			groups = [][]row{flattenGroups(groups)}
		}
	}

	if err := validateColumns(groups, selected, excluded, pred, orderBy); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// WHERE/ORDER BY/LIMIT/OFFSET/COUNT apply per group so each table is a
	// self-contained query result; multi-doc YAML thus yields independent
	// "top N" tables rather than a global one. json/jsonl re-flatten below.
	for i, rows := range groups {
		if pred != nil {
			filtered := rows[:0]
			for _, r := range rows {
				ok, err := pred.Eval(r)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(2)
				}
				if ok {
					filtered = append(filtered, r)
				}
			}
			rows = filtered
		}

		if isCount {
			rows = []row{{"count": len(rows)}}
		}

		if len(orderBy) > 0 {
			sort.SliceStable(rows, func(i, j int) bool {
				for _, term := range orderBy {
					c := parser.CompareValues(rows[i][term.Col], rows[j][term.Col])
					if term.Desc {
						c = -c
					}
					if c != 0 {
						return c < 0
					}
				}
				return false
			})
		}

		if offset > 0 {
			if offset >= len(rows) {
				rows = nil
			} else {
				rows = rows[offset:]
			}
		}
		if limit >= 0 && len(rows) > limit {
			rows = rows[:limit]
		}

		groups[i] = rows
	}

	if isCount {
		selected = []string{"count"}
	}

	if *summary && *stats {
		fmt.Fprintln(os.Stderr, "--summary and --stats are mutually exclusive")
		os.Exit(2)
	}
	if (*summary || *stats) && outFlag != "table" {
		fmt.Fprintln(os.Stderr, "--summary and --stats are only supported for table output")
		os.Exit(2)
	}
	// Truncation is opt-in to TTY-only and only for table-shaped output
	// (json/jsonl must stay structurally valid for downstream consumers).
	var out io.Writer = os.Stdout
	var trunc *truncatingWriter
	if !*noTruncate && outFlag == "table" && isTerminal(os.Stdout) {
		if width, ok := terminalWidth(os.Stdout); ok {
			trunc = newTruncatingWriter(os.Stdout, width)
			out = trunc
		}
	}

	switch outFlag {
	case "table":
		printed := false
		for _, rows := range groups {
			if len(rows) == 0 {
				continue
			}
			if printed {
				fmt.Fprintln(out)
			}
			switch {
			case *stats:
				printStats(out, rows, selected, excluded, !*noHeader)
			case *summary:
				printTableWithSummary(out, rows, selected, excluded, !*noHeader)
			default:
				printTable(out, rows, selected, excluded, !*noHeader)
			}
			if !*noHeader {
				fmt.Fprintln(out, formatRowCount(len(rows)))
			}
			printed = true
		}
		// All groups were empty (e.g., the WHERE filtered every row out).
		// Print a single row-count line so the user can tell the query ran
		// and matched nothing, instead of seeing an empty stdout.
		if !printed && !*noHeader {
			fmt.Fprintln(out, formatRowCount(0))
		}
	case "json":
		printJSON(out, flattenGroups(groups), selected, excluded)
	case "jsonl":
		printJSONL(out, flattenGroups(groups), selected, excluded)
	case "shell":
		printShell(out, flattenGroups(groups), selected, excluded)
	default:
		fmt.Fprintf(os.Stderr, "unknown output format %q (want table, json, jsonl, or shell)\n", outFlag)
		os.Exit(2)
	}

	if trunc != nil {
		_ = trunc.Flush()
		if trunc.Truncated() {
			fmt.Fprintf(os.Stderr, "Output was clipped to fit terminal width %d.\nPass --no-truncate or widen the terminal for the full output.\n", trunc.width)
		}
	}
}

func formatRowCount(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}

// validateColumns reports the first SELECT/EXCLUDE/WHERE/ORDER BY identifier
// that doesn't appear in any loaded row, so a typo like `WHERE staus = 'up'`
// surfaces a clear error instead of silently filtering everything away.
//
// The check uses the union across all groups: if any group has the column,
// it's accepted (multi-doc YAML often mixes schemas, and we'd rather not
// reject a column that exists in some docs). If there are no rows at all,
// validation is skipped — there's nothing to display anyway, so erroring
// would just add noise on top of an empty result.
func validateColumns(groups [][]row, selected, excluded []string, pred parser.WhereExpr, orderBy []parser.OrderTerm) error {
	available := map[string]struct{}{}
	for _, g := range groups {
		for _, r := range g {
			for k := range r {
				available[k] = struct{}{}
			}
		}
	}
	if len(available) == 0 {
		return nil
	}

	check := func(col, source string) error {
		if _, ok := available[col]; ok {
			return nil
		}
		return fmt.Errorf("column %q referenced in %s does not exist\navailable columns: %s",
			col, source, strings.Join(sortedKeys(available), ", "))
	}

	for _, c := range selected {
		if err := check(c, "SELECT"); err != nil {
			return err
		}
	}
	for _, c := range excluded {
		if err := check(c, "EXCLUDE"); err != nil {
			return err
		}
	}
	if pred != nil {
		for _, c := range parser.ReferencedCols(pred) {
			if err := check(c, "WHERE"); err != nil {
				return err
			}
		}
	}
	for _, t := range orderBy {
		if err := check(t.Col, "ORDER BY"); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func flattenGroups(groups [][]row) []row {
	var total int
	for _, g := range groups {
		total += len(g)
	}
	out := make([]row, 0, total)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// errStreamLimitReached unwinds the StreamJSONL callback once LIMIT is met
// without conflating the early-stop signal with a real I/O / parse error.
var errStreamLimitReached = errors.New("stream limit reached")

// streamStdinJSONL is the real-time path: scan stdin one JSON object at a
// time, apply WHERE/OFFSET/LIMIT, and write each surviving row straight to
// os.Stdout. The first object's keys lock in the column projection (when
// the user wrote SELECT *) and become the validation set for column
// references — we never see a "later row" here, so the first row IS the
// schema.
func streamStdinJSONL(in io.Reader, selected, excluded []string, pred parser.WhereExpr, limit, offset int) {
	enc := json.NewEncoder(os.Stdout)
	var skipped, emitted int
	var validated bool
	cols := selected
	err := providers.StreamJSONL(in, func(r row, firstKeys []string) error {
		if !validated {
			if err := validateAgainstKeys(firstKeys, selected, excluded, pred); err != nil {
				return err
			}
			if cols == nil {
				cols = firstKeys
			}
			validated = true
		}
		if pred != nil {
			ok, err := pred.Eval(r)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
		}
		if skipped < offset {
			skipped++
			return nil
		}
		if limit >= 0 && emitted >= limit {
			return errStreamLimitReached
		}
		obj := projectRow(r, cols, excluded)
		if err := enc.Encode(obj); err != nil {
			return err
		}
		emitted++
		return nil
	})
	if err != nil && !errors.Is(err, errStreamLimitReached) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// validateAgainstKeys is the streaming counterpart of validateColumns: it
// checks SELECT/EXCLUDE/WHERE references against a single ordered key set
// (the first row's keys). Empty keys means an empty stream, which we treat
// as "nothing to validate" so an empty pipe just exits cleanly.
func validateAgainstKeys(keys, selected, excluded []string, pred parser.WhereExpr) error {
	if len(keys) == 0 {
		return nil
	}
	available := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		available[k] = struct{}{}
	}
	check := func(col, source string) error {
		if _, ok := available[col]; ok {
			return nil
		}
		return fmt.Errorf("column %q referenced in %s does not exist\navailable columns: %s",
			col, source, strings.Join(keys, ", "))
	}
	for _, c := range selected {
		if err := check(c, "SELECT"); err != nil {
			return err
		}
	}
	for _, c := range excluded {
		if err := check(c, "EXCLUDE"); err != nil {
			return err
		}
	}
	if pred != nil {
		for _, c := range parser.ReferencedCols(pred) {
			if err := check(c, "WHERE"); err != nil {
				return err
			}
		}
	}
	return nil
}

// projectRow builds the per-row object that gets JSON-encoded. Mirrors what
// rowsToJSON does in json.go so the streaming output and the buffered
// `-o jsonl` output produce the same bytes for the same input.
func projectRow(r row, cols, excluded []string) map[string]any {
	excludeSet := make(map[string]struct{}, len(excluded))
	for _, c := range excluded {
		excludeSet[c] = struct{}{}
	}
	out := make(map[string]any, len(cols))
	for _, c := range cols {
		if _, skip := excludeSet[c]; skip {
			continue
		}
		out[c] = r[c]
	}
	return out
}

// peekFirstNonWS consumes any leading JSON whitespace and returns the next
// byte without consuming it. Returns 0 on EOF (so callers can treat empty
// input as "fall through to the buffered path"). The whitespace it consumes
// is not part of any JSON value, so leaving it stripped is harmless for
// downstream parsers.
func peekFirstNonWS(r *bufio.Reader) (byte, error) {
	for {
		b, err := r.Peek(1)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, nil
			}
			return 0, err
		}
		c := b[0]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			if _, err := r.ReadByte(); err != nil {
				return 0, err
			}
			continue
		}
		return c, nil
	}
}

func toProviderOrderBy(in []parser.OrderTerm) []providers.OrderTerm {
	if in == nil {
		return nil
	}
	out := make([]providers.OrderTerm, len(in))
	for i, t := range in {
		out[i] = providers.OrderTerm{Col: t.Col, Desc: t.Desc}
	}
	return out
}
