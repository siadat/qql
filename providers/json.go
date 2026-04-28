package providers

import (
	"encoding/json"
	"fmt"
	"os"
)

func loadJSON(path string) ([]map[string]any, error) {
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
	return rowsFromTree(v)
}
