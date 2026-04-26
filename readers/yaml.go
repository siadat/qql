package readers

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func loadYAML(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var v any
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return buildRows(v), nil
}
