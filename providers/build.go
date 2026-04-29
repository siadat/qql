package providers

import (
	"sort"
	"strconv"
)

// rowsFromTree turns a nested tree (e.g. a decoded JSON or YAML value) into
// flat rows, one row per top-level entry. Two shapes are auto-detected:
//
//   - map root: each top-level key becomes a row, with the key in the `key`
//     column and the value flattened into per-column entries.
//   - list root: each top-level element becomes a row, with its 0-based index
//     (as a string) in the `key` column and the element flattened into
//     per-column entries.
//
// A scalar root degenerates to a single row whose only column is `value`.
// There is no path/glob configuration: anything more elaborate than the two
// shapes above belongs in an external provider.
func rowsFromTree(tree any) ([]map[string]any, error) {
	switch x := tree.(type) {
	case map[string]any:
		rows := make([]map[string]any, 0, len(x))
		for k, child := range x {
			row := map[string]any{"key": k}
			flatten(child, "", row)
			rows = append(rows, row)
		}
		return rows, nil
	case []any:
		rows := make([]map[string]any, 0, len(x))
		for i, child := range x {
			row := map[string]any{"key": strconv.Itoa(i)}
			flatten(child, "", row)
			rows = append(rows, row)
		}
		return rows, nil
	default:
		row := map[string]any{}
		flatten(tree, "", row)
		return []map[string]any{row}, nil
	}
}

func flatten(v any, prefix string, out map[string]any) {
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			flatten(x[k], joinPath(prefix, k), out)
		}
	case []any:
		if len(x) == 0 {
			return
		}
		for i, e := range x {
			flatten(e, joinPath(prefix, strconv.Itoa(i)), out)
		}
	default:
		name := prefix
		if name == "" {
			name = "value"
		}
		out[name] = x
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
