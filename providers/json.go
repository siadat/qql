package providers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func loadJSON(path string) ([]map[string]any, error) {
	tree, err := ReadJSONTree(path)
	if err != nil {
		return nil, err
	}
	return rowsFromTree(tree)
}

// ReadJSONTree decodes the file as a single JSON document and returns the raw
// tree (map / slice / scalar) without flattening into rows. Numbers are
// preserved as json.Number so the original textual representation survives a
// later round-trip through WriteJSONTreeAtomic.
func ReadJSONTree(path string) (any, error) {
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

// WriteJSONTreeAtomic serializes tree as indented JSON and replaces path
// atomically (write to a sibling temp file, fsync, rename). The temp file
// lives in the same directory so the rename stays on one filesystem.
func WriteJSONTreeAtomic(path string, tree any) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".qql-update-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(tree); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// Flatten exposes the package-internal flatten helper so callers (UPDATE in
// main.go) can build the same row representation that rowsFromTree produces
// for an entry, without re-deriving the dot-notation rules.
func Flatten(v any, out map[string]any) { flatten(v, "", out) }
