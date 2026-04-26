package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	selectFlag := flag.String("select", "", "comma-separated list of columns to include")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--select col1,col2,...] <file> [file ...]\n", os.Args[0])
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

	var rows []row
	for _, path := range paths {
		v, err := loadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rows = append(rows, buildRows(path, v)...)
	}

	printTable(os.Stdout, rows, selected)
}
