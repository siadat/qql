package providers

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// loadYAML decodes every document in the file (separated by `---`) and returns
// one row group per document. Truly empty documents (e.g. between consecutive
// `---` markers) are skipped. Callers render one table per group.
func loadYAML(path, prefix string) ([][]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	var groups [][]map[string]any
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		if v == nil {
			continue
		}
		rows, err := rowsFromTree(v, prefix)
		if err != nil {
			return nil, err
		}
		groups = append(groups, rows)
	}
	return groups, nil
}
