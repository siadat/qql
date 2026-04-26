package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func loadFile(path string) (any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var v any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return v, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <file.json> [file.json ...]\n", os.Args[0])
		os.Exit(2)
	}

	docs := make(map[string]any, len(os.Args)-1)
	for _, path := range os.Args[1:] {
		v, err := loadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		docs[path] = v
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(docs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
