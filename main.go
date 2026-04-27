package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/siadat/qql/parser"
	"github.com/siadat/qql/providers"
)

func main() {
	var outFlag string
	flag.StringVar(&outFlag, "out", "table", "output format: table, json, jsonl")
	flag.StringVar(&outFlag, "o", "table", "output format (shorthand for --out)")
	noHeader := flag.Bool("no-header", false, "hide the header row and separator in table output")
	summary := flag.Bool("summary", false, "shrink the table by hoisting columns whose value is identical across every row into a small summary table printed below, so the main table is narrower and fits more terminals")
	stats := flag.Bool("stats", false, "instead of the rows, print a per-column breakdown: unique-value count and a 'value (freq)' list sorted by frequency, useful for sizing up a result set without scrolling")
	noTruncate := flag.Bool("no-truncate", false, "do not clip lines to terminal width. by default, when stdout is an interactive terminal, table output is truncated so wrapping does not garble the layout")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-o FORMAT] QUERY [file ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	selected, excluded, sqlSource, pred, orderBy, limit, offset, with, whereRaw, isCount, err := parser.ParseSQL(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	var paths []string
	if sqlSource != "" {
		paths = append(paths, sqlSource)
	}
	paths = append(paths, args[1:]...)

	var rows []row
	if with.Provider != "" {
		// Provider-driven loads run once per query, not once per path. External
		// providers receive every path via ctx.Files and decide how to
		// interleave them; the git provider reads its repo path out of the
		// provider value itself and ignores Source/Files entirely.
		ctx := providers.Context{
			Source:   sqlSource,
			Files:    paths,
			Prefix:   with.Prefix,
			Provider: with.Provider,
			Select:   selected,
			Where:    whereRaw,
			OrderBy:  toProviderOrderBy(orderBy),
			Limit:    limit,
			Offset:   offset,
		}
		var err error
		rows, err = providers.Load(ctx)
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
				Prefix:   with.Prefix,
				Provider: with.Provider,
				Select:   selected,
				Where:    whereRaw,
				OrderBy:  toProviderOrderBy(orderBy),
				Limit:    limit,
				Offset:   offset,
			}
			pathRows, err := providers.Load(ctx)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			rows = append(rows, pathRows...)
		}
	}

	if pred != nil {
		filtered := rows[:0]
		for _, r := range rows {
			if pred.Eval(r) {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}

	if isCount {
		rows = []row{{"count": len(rows)}}
		selected = []string{"count"}
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
		switch {
		case *stats:
			printStats(out, rows, selected, excluded, !*noHeader)
		case *summary:
			printTableWithSummary(out, rows, selected, excluded, !*noHeader)
		default:
			printTable(out, rows, selected, excluded, !*noHeader)
		}
	case "json":
		printJSON(out, rows, selected, excluded)
	case "jsonl":
		printJSONL(out, rows, selected, excluded)
	default:
		fmt.Fprintf(os.Stderr, "unknown output format %q (want table, json, or jsonl)\n", outFlag)
		os.Exit(2)
	}

	if trunc != nil {
		_ = trunc.Flush()
		if trunc.Truncated() {
			fmt.Fprintf(os.Stderr, "Output was clipped to fit terminal width %d.\nPass --no-truncate or widen the terminal for the full output.\n", trunc.width)
		}
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
