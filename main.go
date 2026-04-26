package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <file> [file ...]\n", os.Args[0])
		os.Exit(2)
	}

	var rows []row
	for _, path := range os.Args[1:] {
		v, err := loadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		rows = append(rows, buildRows(path, v)...)
	}

	printTable(os.Stdout, rows)
}
