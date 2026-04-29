package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/siadat/qql/parser"
	"github.com/siadat/qql/providers"
)

// runUpdate executes an UPDATE statement against a single JSON file. It mirrors
// rowsFromTree's row identification (each top-level entry is a row, with a
// synthetic `key` column) so users can write `WHERE key = 'foo'` against map
// roots and `WHERE key = '0'` against array roots, exactly the way SELECT sees
// them. Mutations land on the underlying tree entry, not the flattened row.
func runUpdate(stmt *parser.UpdateStmt, posArgs []string) {
	path, err := updateValidatePath(posArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	tree, err := providers.ReadJSONTree(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	entries, err := updateEntries(tree)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Collect each entry's flattened row first (with synthetic `key`) so we
	// can validate WHERE column names against the union before mutating
	// anything. Same spirit as validateColumns on the SELECT path.
	rows := make([]map[string]any, len(entries))
	for i, e := range entries {
		row := map[string]any{}
		providers.Flatten(e.value, row)
		row["key"] = e.key
		rows[i] = row
	}
	if err := updateValidateRefs(rows, stmt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	matched := 0
	for i, e := range entries {
		ok, err := stmt.Pred.Eval(rows[i])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if !ok {
			continue
		}
		for _, s := range stmt.Sets {
			if err := applySet(e.value, strings.Split(s.Col, "."), s.Value); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		matched++
	}

	if err := providers.WriteJSONTreeAtomic(path, tree); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(formatUpdateCount(matched))
}

func updateValidatePath(posArgs []string) (string, error) {
	if len(posArgs) == 0 {
		return "", fmt.Errorf("UPDATE requires a JSON file path as a positional argument")
	}
	if len(posArgs) > 1 {
		return "", fmt.Errorf("UPDATE accepts a single file path, got %d", len(posArgs))
	}
	path := posArgs[0]
	if path == "-" {
		return "", fmt.Errorf("UPDATE does not support reading from stdin")
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext != ".json" {
		return "", fmt.Errorf("UPDATE only supports .json files, got %q", ext)
	}
	return path, nil
}

// updateEntry pairs the synthetic `key` column with a reference to the tree
// node a SET assignment will mutate. For map roots the value is the
// per-key sub-object, for array roots it is the per-index element. The
// caller relies on map/slice reference semantics to mutate in place.
type updateEntry struct {
	key   string
	value any
}

func updateEntries(tree any) ([]updateEntry, error) {
	switch x := tree.(type) {
	case map[string]any:
		entries := make([]updateEntry, 0, len(x))
		for k, v := range x {
			entries = append(entries, updateEntry{key: k, value: v})
		}
		return entries, nil
	case []any:
		entries := make([]updateEntry, 0, len(x))
		for i, v := range x {
			entries = append(entries, updateEntry{key: strconv.Itoa(i), value: v})
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("UPDATE: scalar root has no fields to update")
	}
}

func updateValidateRefs(rows []map[string]any, stmt *parser.UpdateStmt) error {
	available := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			available[k] = struct{}{}
		}
	}
	if len(available) == 0 {
		return nil
	}
	check := func(col, source string) error {
		if _, ok := available[col]; ok {
			return nil
		}
		return fmt.Errorf("column %q referenced in %s does not exist\navailable columns: %s",
			col, source, strings.Join(sortedKeys(available), ", "))
	}
	if stmt.Pred != nil {
		for _, c := range parser.ReferencedCols(stmt.Pred) {
			if err := check(c, "WHERE"); err != nil {
				return err
			}
		}
	}
	return nil
}

// applySet walks `entry` along `path` and stores `value` at the leaf. The
// caller has split a dot-notation column on `.`, so each segment is either
// a map key or, when the parent is an array, a numeric index.
//
// Non-leaf segments must already exist as a map or array. Creating
// intermediate containers is intentionally not supported: a SET that lands
// in an unexpected place is more likely a typo than a feature request, and
// the user can always add the parent first via a separate UPDATE once that
// shape is supported.
func applySet(entry any, path []string, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("internal: empty SET path")
	}
	last := len(path) - 1
	cur := entry
	for i, seg := range path {
		if i == last {
			switch parent := cur.(type) {
			case map[string]any:
				parent[seg] = value
				return nil
			case []any:
				idx, err := strconv.Atoi(seg)
				if err != nil || idx < 0 || idx >= len(parent) {
					return fmt.Errorf("cannot SET %q: %q is not a valid array index", strings.Join(path, "."), seg)
				}
				parent[idx] = value
				return nil
			default:
				return fmt.Errorf("cannot SET %q: parent at %q is not an object or array", strings.Join(path, "."), strings.Join(path[:i], "."))
			}
		}
		switch parent := cur.(type) {
		case map[string]any:
			child, ok := parent[seg]
			if !ok {
				return fmt.Errorf("cannot SET %q: key %q does not exist", strings.Join(path, "."), seg)
			}
			cur = child
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(parent) {
				return fmt.Errorf("cannot SET %q: %q is not a valid array index", strings.Join(path, "."), seg)
			}
			cur = parent[idx]
		default:
			return fmt.Errorf("cannot SET %q: parent at %q is not an object or array", strings.Join(path, "."), strings.Join(path[:i], "."))
		}
	}
	return fmt.Errorf("internal: SET path walk fell through")
}

func formatUpdateCount(n int) string {
	if n == 1 {
		return "1 row updated"
	}
	return fmt.Sprintf("%d rows updated", n)
}
