package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/siadat/qql/parser"
	"github.com/siadat/qql/providers"
)

func main() {
	sqlFlag := flag.String("sql", "", `SQL-like query, e.g. "SELECT col1, col2 WHERE col3 > 5"`)
	var outFlag string
	flag.StringVar(&outFlag, "out", "table", "output format: table, json, jsonl")
	flag.StringVar(&outFlag, "o", "table", "output format (shorthand for --out)")
	noHeader := flag.Bool("no-header", false, "hide the header row and separator in table output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--sql QUERY] [-o FORMAT] <file> [file ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	var selected []string
	var pred parser.WhereExpr
	var sqlSource string
	var orderBy []parser.OrderTerm
	var with parser.WithOptions
	var whereRaw string
	if *sqlFlag != "" {
		s, src, p, ob, w, wr, err := parser.ParseSQL(*sqlFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		selected, sqlSource, pred, orderBy, with, whereRaw = s, src, p, ob, w, wr
	}

	var paths []string
	if sqlSource != "" {
		paths = append(paths, sqlSource)
	}
	paths = append(paths, flag.Args()...)

	var rows []row
	if strings.HasPrefix(with.Provider, "external:") {
		// External providers run once per query and receive every path via
		// ctx.Files; the script decides how to interleave them.
		ctx := providers.Context{
			Source:   sqlSource,
			Files:    paths,
			Prefix:   with.Prefix,
			Provider: with.Provider,
			Select:   selected,
			Where:    whereRaw,
			OrderBy:  toProviderOrderBy(orderBy),
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

	switch outFlag {
	case "table":
		printTable(os.Stdout, rows, selected, !*noHeader)
	case "json":
		printJSON(os.Stdout, rows, selected)
	case "jsonl":
		printJSONL(os.Stdout, rows, selected)
	default:
		fmt.Fprintf(os.Stderr, "unknown output format %q (want table, json, or jsonl)\n", outFlag)
		os.Exit(2)
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
