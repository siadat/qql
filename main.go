package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	sqlFlag := flag.String("sql", "", `SQL-like query, e.g. "SELECT col1, col2 WHERE col3 > 5"`)
	var outFlag string
	flag.StringVar(&outFlag, "out", "table", "output format: table, json, jsonl")
	flag.StringVar(&outFlag, "o", "table", "output format (shorthand for --out)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--sql QUERY] [-o FORMAT] <file> [file ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var selected []string
	var pred whereExpr
	if *sqlFlag != "" {
		s, p, err := parseSQL(*sqlFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		selected, pred = s, p
	}

	var rows []row
	for _, path := range paths {
		v, err := loadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rows = append(rows, buildRows(path, v)...)
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
		printTable(os.Stdout, rows, selected)
	case "json":
		printJSON(os.Stdout, rows, selected)
	case "jsonl":
		printJSONL(os.Stdout, rows, selected)
	default:
		fmt.Fprintf(os.Stderr, "unknown output format %q (want table, json, or jsonl)\n", outFlag)
		os.Exit(2)
	}
}
