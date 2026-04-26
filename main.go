package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	selectFlag := flag.String("select", "", "comma-separated list of columns to include")
	whereFlag := flag.String("where", "", "filter expression, e.g. \"age >= 30 AND active = true\"")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--select col1,col2,...] [--where EXPR] <file> [file ...]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var selected []string
	if *selectFlag != "" {
		for _, c := range strings.Split(*selectFlag, ",") {
			if c = strings.TrimSpace(c); c != "" {
				selected = append(selected, c)
			}
		}
	}

	var pred whereExpr
	if *whereFlag != "" {
		p, err := parseWhere(*whereFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		pred = p
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

	printTable(os.Stdout, rows, selected)
}
