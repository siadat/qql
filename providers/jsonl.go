package providers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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
