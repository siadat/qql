package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/siadat/qql/readers"
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
	var pred whereExpr
	var sqlSource string
	if *sqlFlag != "" {
		s, src, p, err := parseSQL(*sqlFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		selected, sqlSource, pred = s, src, p
	}

	var paths []string
	if sqlSource != "" {
		paths = append(paths, sqlSource)
	}
	paths = append(paths, flag.Args()...)
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var rows []row
	for _, path := range paths {
		pathRows, err := readers.Load(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rows = append(rows, pathRows...)
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
