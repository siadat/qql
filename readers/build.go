package readers

import (
	"sort"
	"strconv"
)

func buildRows(value any) []map[string]any {
	m, ok := value.(map[string]any)
	if !ok {
		row := map[string]any{}
		flatten(value, "", row)
		return []map[string]any{row}
	}

	rows := make([]map[string]any, 0, len(m))
	for k, v := range m {
		row := map[string]any{"id": k}
		flatten(v, "", row)
		rows = append(rows, row)
	}
	return rows
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
		out[prefix] = x
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}
