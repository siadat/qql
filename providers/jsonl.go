package providers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// LoadStdin reads stdin in either JSONL form (one JSON object per line) or
// as a single JSON document, auto-detecting which. Detection is "try JSONL
// first": if every non-blank, non-comment line decodes to a JSON object,
// it's JSONL. Otherwise the entire buffer is parsed as one JSON value and
// dispatched through rowsFromTree (the same path JSON files take).
//
// One ergonomic special case: a top-level JSON array of objects is treated
// like a JSONL stream — each element becomes a row directly, without the
// synthetic `key` column rowsFromTree would otherwise add for an array root.
// That covers common pipes like `kubectl get pods -o json | jq '.items'`.
//
// firstKeys is the first row's keys in JSON-document order when we can
// derive them (JSONL stream or top-level array of objects), or nil
// otherwise. Callers use it to drive the default `SELECT *` projection.
func LoadStdin(r io.Reader) ([]map[string]any, []string, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	if len(bytes.TrimSpace(buf)) == 0 {
		return nil, nil, nil
	}
	if rows, firstKeys, jerr := LoadJSONL(bytes.NewReader(buf)); jerr == nil {
		return rows, firstKeys, nil
	}
	return loadSingleJSONDoc(buf)
}

// loadSingleJSONDoc parses buf as exactly one JSON value and turns it into
// rows. A top-level array whose elements are all JSON objects is unfolded
// row-per-element (so the user can pipe in a JSON array of records and
// have it behave like JSONL); everything else goes through rowsFromTree.
func loadSingleJSONDoc(buf []byte) ([]map[string]any, []string, error) {
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, nil, fmt.Errorf("decode JSON: %w", err)
	}
	if arr, ok := v.([]any); ok && len(arr) > 0 && allObjects(arr) {
		rows := make([]map[string]any, 0, len(arr))
		for _, e := range arr {
			rows = append(rows, e.(map[string]any))
		}
		return rows, firstArrayElementKeys(buf), nil
	}
	rows, err := rowsFromTree(v)
	return rows, nil, err
}

func allObjects(arr []any) bool {
	for _, e := range arr {
		if _, ok := e.(map[string]any); !ok {
			return false
		}
	}
	return true
}

// firstArrayElementKeys re-decodes buf just far enough to capture the first
// array element's bytes, then walks its tokens to recover key order
// (encoding/json's map decoder loses it).
func firstArrayElementKeys(buf []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(buf))
	tok, err := dec.Token()
	if err != nil {
		return nil
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return nil
	}
	if !dec.More() {
		return nil
	}
	var first json.RawMessage
	if err := dec.Decode(&first); err != nil {
		return nil
	}
	keys, err := jsonObjectKeysInOrder(string(first))
	if err != nil {
		return nil
	}
	return keys
}

// StreamJSONL reads newline-delimited JSON objects from r and invokes onRow
// once per row in stream order. firstKeys carries the keys of the first
// object in JSON-document order and is passed to every call so the caller
// can lock in a column projection on the very first row. If onRow returns a
// non-nil error, scanning stops and that error is returned. Blank lines and
// `#`-prefixed comment lines are skipped to keep the format hand-friendly.
//
// The underlying decoder is configured with UseNumber so int/float
// distinctions survive — the buffered loader, the JSON file loader, and the
// streaming loader all agree on number representation.
func StreamJSONL(r io.Reader, onRow func(row map[string]any, firstKeys []string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var firstKeys []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if firstKeys == nil {
			keys, kerr := jsonObjectKeysInOrder(line)
			if kerr != nil {
				return fmt.Errorf("decode JSONL line %d: %w", lineNum, kerr)
			}
			firstKeys = keys
		}
		var row map[string]any
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&row); err != nil {
			return fmt.Errorf("decode JSONL line %d: %w", lineNum, err)
		}
		if err := onRow(row, firstKeys); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// LoadJSONL reads the entire stream into memory and returns the rows along
// with the first object's keys. Use it when you need to apply ORDER BY or
// run multiple passes over the data; use StreamJSONL when you want to emit
// rows as they arrive.
func LoadJSONL(r io.Reader) (rows []map[string]any, firstKeys []string, err error) {
	err = StreamJSONL(r, func(row map[string]any, fk []string) error {
		firstKeys = fk
		rows = append(rows, row)
		return nil
	})
	return rows, firstKeys, err
}

// jsonObjectKeysInOrder returns the top-level keys of a JSON object in the
// order they appear in the source bytes. encoding/json's map decoder loses
// order, so we walk tokens manually for the first row only.
func jsonObjectKeysInOrder(line string) ([]string, error) {
	dec := json.NewDecoder(strings.NewReader(line))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected JSON object, got %v", tok)
	}
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %v", keyTok)
		}
		keys = append(keys, key)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
	}
	return keys, nil
}
