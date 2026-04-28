package providers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// rowsFromTree turns a nested tree (e.g. a decoded JSON or YAML value) into
// flat rows by walking it under a dot-glob prefix.
//
// prefix is a dot-separated glob: "*" matches any map key or list index (and
// captures it as a column), literal segments descend into that map key or
// numeric list index without capturing. The rightmost "*" captures as column
// "key" with the FULL path from root to the row (including any literals after
// the wildcard, e.g. `*.servers` → "region-a.servers"). Earlier "*"s capture
// as "key_capture_1", "key_capture_2", … (1-indexed, left-to-right) with just
// the matched key. Empty prefix is treated as "*" — top-level map keys (or
// list indices) become rows, matching the original (pre-WITH) behavior for
// maps and giving lists a parallel "one row per element" shape.
//
// Branches that don't match (scalar where the path expects a map or list,
// missing key, out-of-range index) are silently skipped — there's no schema,
// so partial matches are normal.
func rowsFromTree(tree any, prefix string) ([]map[string]any, error) {
	if prefix == "" {
		prefix = "*"
	}
	segs := strings.Split(prefix, ".")
	for _, s := range segs {
		if s == "" {
			return nil, fmt.Errorf("invalid rows path %q: empty segment", prefix)
		}
	}
	// Default-path back-compat: a scalar at the root becomes a single row of
	// flattened columns instead of producing zero rows. List/map roots both go
	// through the wildcard walk so a top-level list expands to one row per
	// element, with the index serving as the row key.
	if len(segs) == 1 && segs[0] == "*" {
		if !isContainer(tree) {
			row := map[string]any{}
			flatten(tree, "", row)
			return []map[string]any{row}, nil
		}
	}
	names := wildcardColumnNames(segs)
	rightmost := rightmostWildcard(segs)
	matches := make([]string, len(segs))

	var rows []map[string]any
	walkRows(tree, segs, names, rightmost, 0, matches, &rows)
	return rows, nil
}

func isContainer(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

func rightmostWildcard(segs []string) int {
	idx := -1
	for i, s := range segs {
		if s == "*" {
			idx = i
		}
	}
	return idx
}

func wildcardColumnNames(segs []string) []string {
	names := make([]string, len(segs))
	var stars []int
	for i, s := range segs {
		if s == "*" {
			stars = append(stars, i)
		}
	}
	for n, pos := range stars {
		if n == len(stars)-1 {
			names[pos] = "key"
		} else {
			names[pos] = fmt.Sprintf("key_capture_%d", n+1)
		}
	}
	return names
}

func walkRows(v any, segs, names []string, rightmost, depth int, matches []string, rows *[]map[string]any) {
	if depth == len(segs) {
		row := make(map[string]any, len(segs)+4)
		for i, name := range names {
			if name == "" {
				continue
			}
			row[name] = captureValue(segs, matches, i, i == rightmost)
		}
		flatten(v, "", row)
		*rows = append(*rows, row)
		return
	}
	seg := segs[depth]
	switch x := v.(type) {
	case map[string]any:
		if seg == "*" {
			for k, child := range x {
				matches[depth] = k
				walkRows(child, segs, names, rightmost, depth+1, matches, rows)
			}
			return
		}
		child, ok := x[seg]
		if !ok {
			return
		}
		walkRows(child, segs, names, rightmost, depth+1, matches, rows)
	case []any:
		if seg == "*" {
			for i, child := range x {
				matches[depth] = strconv.Itoa(i)
				walkRows(child, segs, names, rightmost, depth+1, matches, rows)
			}
			return
		}
		// A literal segment against a list addresses an element by its
		// stringified index (matching how flatten names list children),
		// e.g. prefix `arr.0` descends into the first element of `arr`.
		idx, err := strconv.Atoi(seg)
		if err != nil || idx < 0 || idx >= len(x) {
			return
		}
		walkRows(x[idx], segs, names, rightmost, depth+1, matches, rows)
	}
}

// captureValue builds the column value for the wildcard at segs[i]:
//   - For non-rightmost wildcards: just the matched key.
//   - For the rightmost wildcard: the full path from root through the row,
//     i.e. all preceding literals + earlier wildcard matches + this match +
//     any trailing literals after the rightmost wildcard.
func captureValue(segs, matches []string, i int, isRightmost bool) string {
	if !isRightmost {
		return matches[i]
	}
	parts := make([]string, len(segs))
	for j, s := range segs {
		if s == "*" {
			parts[j] = matches[j]
		} else {
			parts[j] = s
		}
	}
	return strings.Join(parts, ".")
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
